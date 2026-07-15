// Package hr reads HR-contact spreadsheets (WhatsApp + email outreach lists)
// from an .xlsx file and serves them ranked by company importance.
//
// xlsx.go is a minimal, dependency-free .xlsx reader built on the standard
// library (archive/zip + encoding/xml). An .xlsx is a zip of XML parts; we only
// need shared strings and each worksheet's cell values, so a full spreadsheet
// library would be overkill.
package hr

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

// sheetData is one worksheet as rows of column→value (e.g. {"A":"delhivery"}).
type sheetData struct {
	Name string
	Rows []map[string]string
}

// workbook is the parsed set of worksheets from an .xlsx file.
type workbook struct {
	sheets []sheetData
}

// colRe extracts the column letters from a cell reference like "B12" → "B".
var colRe = regexp.MustCompile(`^[A-Z]+`)

// readXLSX opens an .xlsx file and returns its worksheets in workbook order,
// with shared-string and inline-string cells resolved to their text.
func readXLSX(path string) (*workbook, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("could not open %q: %w", path, err)
	}
	defer zr.Close()

	files := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		files[f.Name] = f
	}

	shared, err := readSharedStrings(files["xl/sharedStrings.xml"])
	if err != nil {
		return nil, err
	}

	// Sheet names + their target files, in workbook (display) order.
	names, err := readSheetOrder(files)
	if err != nil {
		return nil, err
	}

	wb := &workbook{}
	for _, s := range names {
		f := files[s.target]
		if f == nil {
			continue
		}
		rows, err := readWorksheet(f, shared)
		if err != nil {
			return nil, fmt.Errorf("sheet %q: %w", s.name, err)
		}
		wb.sheets = append(wb.sheets, sheetData{Name: s.name, Rows: rows})
	}
	return wb, nil
}

// sheetByIndex returns the worksheet at position i (0-based) or nil.
func (w *workbook) sheetByIndex(i int) *sheetData {
	if i < 0 || i >= len(w.sheets) {
		return nil
	}
	return &w.sheets[i]
}

func readSharedStrings(f *zip.File) ([]string, error) {
	if f == nil {
		return nil, nil // no shared strings is valid (all inline)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var doc struct {
		SI []struct {
			// <si> may hold a single <t> or several <r><t> runs; capture all text.
			Text string   `xml:"t"`
			Runs []string `xml:"r>t"`
		} `xml:"si"`
	}
	if err := xml.NewDecoder(rc).Decode(&doc); err != nil {
		return nil, fmt.Errorf("sharedStrings: %w", err)
	}
	out := make([]string, len(doc.SI))
	for i, si := range doc.SI {
		if len(si.Runs) > 0 {
			out[i] = strings.Join(si.Runs, "")
		} else {
			out[i] = si.Text
		}
	}
	return out, nil
}

type sheetRef struct {
	name   string
	target string // e.g. "xl/worksheets/sheet1.xml"
}

// readSheetOrder resolves workbook.xml sheet order + names to their part paths
// via workbook.xml.rels.
func readSheetOrder(files map[string]*zip.File) ([]sheetRef, error) {
	wbf := files["xl/workbook.xml"]
	if wbf == nil {
		return nil, fmt.Errorf("xl/workbook.xml missing")
	}
	rc, err := wbf.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var doc struct {
		Sheets []struct {
			Name string `xml:"name,attr"`
			RID  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
		} `xml:"sheets>sheet"`
	}
	if err := xml.NewDecoder(rc).Decode(&doc); err != nil {
		return nil, fmt.Errorf("workbook.xml: %w", err)
	}

	rels, err := readRels(files["xl/_rels/workbook.xml.rels"])
	if err != nil {
		return nil, err
	}

	var refs []sheetRef
	for _, s := range doc.Sheets {
		target := rels[s.RID]
		if target == "" {
			continue
		}
		if !strings.HasPrefix(target, "xl/") {
			target = "xl/" + target // rels targets are relative to xl/
		}
		refs = append(refs, sheetRef{name: s.Name, target: target})
	}
	return refs, nil
}

func readRels(f *zip.File) (map[string]string, error) {
	out := map[string]string{}
	if f == nil {
		return out, nil
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var doc struct {
		Rel []struct {
			ID     string `xml:"Id,attr"`
			Target string `xml:"Target,attr"`
		} `xml:"Relationship"`
	}
	if err := xml.NewDecoder(rc).Decode(&doc); err != nil {
		return nil, fmt.Errorf("workbook.xml.rels: %w", err)
	}
	for _, r := range doc.Rel {
		out[r.ID] = r.Target
	}
	return out, nil
}

// readWorksheet streams a worksheet's cells into rows of column→text. It resolves
// shared-string ("s") and inline-string cells; other cell types (numbers, etc.)
// use the raw <v> text, which is what we want for phone numbers stored as text.
func readWorksheet(f *zip.File, shared []string) ([]map[string]string, error) {
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	dec := xml.NewDecoder(rc)
	var rows []map[string]string
	var cur map[string]string

	// current-cell state
	var cellRef, cellType string
	var inV, inT bool
	var vbuf, tbuf strings.Builder

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "row":
				cur = map[string]string{}
			case "c":
				cellRef, cellType = "", ""
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "r":
						cellRef = a.Value
					case "t":
						cellType = a.Value
					}
				}
				vbuf.Reset()
				tbuf.Reset()
			case "v":
				inV = true
			case "t":
				inT = true
			}
		case xml.CharData:
			if inV {
				vbuf.Write(t)
			} else if inT {
				tbuf.Write(t)
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "v":
				inV = false
			case "t":
				inT = false
			case "c":
				val := cellValue(cellType, vbuf.String(), tbuf.String(), shared)
				if cur != nil && cellRef != "" && val != "" {
					cur[colLetters(cellRef)] = val
				}
			case "row":
				if cur != nil {
					rows = append(rows, cur)
					cur = nil
				}
			}
		}
	}
	return rows, nil
}

func cellValue(cellType, v, inlineT string, shared []string) string {
	switch cellType {
	case "s": // shared string: v is an index
		idx, err := strconv.Atoi(strings.TrimSpace(v))
		if err == nil && idx >= 0 && idx < len(shared) {
			return shared[idx]
		}
		return ""
	case "inlineStr": // inline string in <is><t>
		return inlineT
	case "str": // formula string result
		return v
	default: // number/bool/date — raw value text
		return v
	}
}

func colLetters(ref string) string {
	return colRe.FindString(ref)
}
