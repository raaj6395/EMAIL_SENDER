package email

import "strings"

import (
	"testing"
)

func TestFillBodiesDerivesTheMissingHalf(t *testing.T) {
	// Every message ships both parts, so whichever half the caller omits must be
	// generated — an empty alternative would render blank for some clients.
	tests := []struct {
		name             string
		inHTML, inText   string
		wantHTMLContains []string
		wantTextContains []string
		wantTextExcludes []string
	}{
		{
			name:             "html only",
			inHTML:           `<div><p>Hi <b>Ankit</b>,</p><p>Line&nbsp;one<br>Line two</p></div>`,
			wantHTMLContains: []string{"<b>Ankit</b>"},
			wantTextContains: []string{"Hi Ankit,", "Line one\nLine two"},
			wantTextExcludes: []string{"<p>", "<br>", "&nbsp;"},
		},
		{
			name:             "html with script and style dropped",
			inHTML:           `<style>p{color:red}</style><p>Real body</p><script>alert(1)</script>`,
			wantTextContains: []string{"Real body"},
			wantTextExcludes: []string{"color:red", "alert(1)"},
		},
		{
			name:             "text only",
			inText:           "Hi there,\n\nA <second> paragraph.",
			wantHTMLContains: []string{"<p", "Hi there,", "A &lt;second&gt; paragraph."},
			wantTextContains: []string{"A <second> paragraph."},
		},
		{
			name:             "both supplied are left alone",
			inHTML:           "<p>H</p>",
			inText:           "T",
			wantHTMLContains: []string{"<p>H</p>"},
			wantTextContains: []string{"T"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotHTML, gotText := FillBodies(tc.inHTML, tc.inText)
			if strings.TrimSpace(gotHTML) == "" || strings.TrimSpace(gotText) == "" {
				t.Fatalf("both parts must be non-empty, got html=%q text=%q", gotHTML, gotText)
			}
			for _, want := range tc.wantHTMLContains {
				if !strings.Contains(gotHTML, want) {
					t.Errorf("html %q missing %q", gotHTML, want)
				}
			}
			for _, want := range tc.wantTextContains {
				if !strings.Contains(gotText, want) {
					t.Errorf("text %q missing %q", gotText, want)
				}
			}
			for _, bad := range tc.wantTextExcludes {
				if strings.Contains(gotText, bad) {
					t.Errorf("text %q should not contain %q", gotText, bad)
				}
			}
		})
	}
}

func TestFillBodiesEmptyStaysEmpty(t *testing.T) {
	// The handler relies on this to reject a bodyless send.
	if h, txt := FillBodies("", ""); h != "" || txt != "" {
		t.Errorf("empty input should stay empty, got html=%q text=%q", h, txt)
	}
}
