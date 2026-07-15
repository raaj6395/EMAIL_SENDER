package hr

import (
	"regexp"
	"strings"
)

// Contact is one HR/recruiter contact from the spreadsheet, normalized for the
// UI. WhatsApp contacts carry Phone; email contacts carry Email.
type Contact struct {
	Company string `json:"company"`
	Name    string `json:"name"`
	Role    string `json:"role"`
	Email   string `json:"email,omitempty"`
	Phone   string `json:"phone,omitempty"`     // display form, e.g. "+91 63954 86191"
	WaPhone string `json:"waPhone,omitempty"`   // digits only for wa.me, e.g. "916395486191"
	Rank    int    `json:"rank"`                // company importance score (0-100), higher = more important
}

// nonDigit strips everything but digits from a phone number.
var nonDigit = regexp.MustCompile(`\D`)

// LoadWhatsApp reads Sheet1 (columns: Company, Name, Role, Phone Number) and
// returns contacts that have a usable phone number.
func LoadWhatsApp(xlsxPath string) ([]Contact, error) {
	wb, err := readXLSX(xlsxPath)
	if err != nil {
		return nil, err
	}
	sheet := wb.sheetByIndex(0) // Sheet1
	if sheet == nil {
		return nil, nil
	}
	var out []Contact
	for i, row := range sheet.Rows {
		if i == 0 {
			continue // header
		}
		company := strings.TrimSpace(row["A"])
		phoneRaw := strings.TrimSpace(row["D"])
		wa := normalizePhone(phoneRaw)
		if company == "" || wa == "" {
			continue // need at least a company + a dialable number
		}
		out = append(out, Contact{
			Company: company,
			Name:    strings.TrimSpace(row["B"]),
			Role:    strings.TrimSpace(row["C"]),
			Phone:   phoneRaw,
			WaPhone: wa,
		})
	}
	return out, nil
}

// LoadEmail reads Cleaned_Sheet (columns: Company Name, People Name, Contact
// (email), Role) and returns contacts that have a valid-looking email.
func LoadEmail(xlsxPath string) ([]Contact, error) {
	wb, err := readXLSX(xlsxPath)
	if err != nil {
		return nil, err
	}
	sheet := wb.sheetByIndex(1) // Cleaned_Sheet
	if sheet == nil {
		return nil, nil
	}
	var out []Contact
	for i, row := range sheet.Rows {
		if i == 0 {
			continue // header
		}
		company := strings.TrimSpace(row["A"])
		email := strings.TrimSpace(row["C"])
		if company == "" || !strings.Contains(email, "@") {
			continue
		}
		out = append(out, Contact{
			Company: company,
			Name:    strings.TrimSpace(row["B"]),
			Role:    strings.TrimSpace(row["D"]),
			Email:   email,
		})
	}
	return out, nil
}

// normalizePhone converts a display phone (e.g. "+91 63954 86191") into the
// digits-only form wa.me expects (country code + number, no "+" or spaces).
// Returns "" if there aren't enough digits to be a real number.
func normalizePhone(raw string) string {
	d := nonDigit.ReplaceAllString(raw, "")
	if len(d) < 10 {
		return ""
	}
	// A bare 10-digit Indian number (no country code) → prefix 91.
	if len(d) == 10 {
		d = "91" + d
	}
	return d
}

// UniqueCompanies returns the distinct company names (original casing of first
// occurrence) across the given contacts, for ranking.
func UniqueCompanies(contacts []Contact) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range contacts {
		key := strings.ToLower(strings.TrimSpace(c.Company))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, c.Company)
	}
	return out
}
