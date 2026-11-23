package httpapi

import (
	"os"
	"path/filepath"
	"testing"
)

// resumePath comes from an API caller, so the validator is what stops it from
// becoming a way to mail an arbitrary file off disk.
func TestValidateResumePath(t *testing.T) {
	dir := t.TempDir()

	good := filepath.Join(dir, "resume.pdf")
	if err := os.WriteFile(good, []byte("%PDF-1.4 ..."), 0o644); err != nil {
		t.Fatal(err)
	}
	empty := filepath.Join(dir, "empty.pdf")
	if err := os.WriteFile(empty, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	notPDF := filepath.Join(dir, "secrets.txt")
	if err := os.WriteFile(notPDF, []byte("hunter2"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := validateResumePath(good); err != nil {
		t.Errorf("a real PDF should pass: %v", err)
	}
	// Extension matching is case-insensitive (an upload named .PDF is still a PDF).
	upper := filepath.Join(dir, "Upper.PDF")
	if err := os.WriteFile(upper, []byte("%PDF-1.4 ..."), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := validateResumePath(upper); err != nil {
		t.Errorf(".PDF should be accepted: %v", err)
	}
	for name, path := range map[string]string{
		"non-pdf extension": notPDF,
		"empty file":        empty,
		"directory":         dir,
		"missing file":      filepath.Join(dir, "nope.pdf"),
	} {
		if err := validateResumePath(path); err == nil {
			t.Errorf("%s should be rejected: %s", name, path)
		}
	}
}
