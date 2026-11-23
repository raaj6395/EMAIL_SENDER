package httpapi

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

// maxResumeBytes bounds an uploaded resume. A PDF resume is a few hundred KB;
// 10 MB is generous and keeps a runaway upload from filling the data dir.
const maxResumeBytes = 10 << 20

// pdfMagic is the required first bytes of a PDF file.
var pdfMagic = []byte("%PDF-")

// handleResumeUpload replaces a track's resume PDF (multipart field "file",
// track from ?track= or the form). Written temp-then-rename so a failed upload
// can't leave a half-written resume in place of a working one.
func (s *Server) handleResumeUpload(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxResumeBytes)
	if err := r.ParseMultipartForm(maxResumeBytes); err != nil {
		writeError(w, http.StatusBadRequest, "could not read upload (max 10MB): "+err.Error())
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing form field \"file\"")
		return
	}
	defer file.Close()

	// Accept the track from either the query string or the form body.
	rawTrack := r.URL.Query().Get("track")
	if rawTrack == "" {
		rawTrack = r.FormValue("track")
	}
	track := normalizeTrack(rawTrack)
	dest := s.cfg.ResumePathFor(track)

	data, err := io.ReadAll(file)
	if err != nil {
		writeError(w, http.StatusBadRequest, "could not read upload: "+err.Error())
		return
	}
	if !bytes.HasPrefix(data, pdfMagic) {
		writeError(w, http.StatusBadRequest, "not a PDF: "+header.Filename)
		return
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), ".resume-*.pdf")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not write resume: "+err.Error())
		return
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		writeError(w, http.StatusInternalServerError, "could not write resume: "+err.Error())
		return
	}
	if err := tmp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "could not write resume: "+err.Error())
		return
	}
	if err := os.Rename(tmpName, dest); err != nil {
		writeError(w, http.StatusInternalServerError, "could not save resume: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":    true,
		"track": track,
		"path":  dest,
		"bytes": len(data),
	})
}

// handleResumeGet serves the resume PDF on file for ?track=, so the UI can show
// exactly which file goes out with each email.
func (s *Server) handleResumeGet(w http.ResponseWriter, r *http.Request) {
	track := normalizeTrack(r.URL.Query().Get("track"))
	if !s.cfg.HasResumeFor(track) {
		writeError(w, http.StatusNotFound, "no resume on file for this profile")
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Cache-Control", "no-store")
	http.ServeFile(w, r, s.cfg.ResumePathFor(track))
}
