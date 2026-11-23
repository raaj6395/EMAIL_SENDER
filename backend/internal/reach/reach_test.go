package reach

import "testing"

func TestSplitEmailsMultiplePerCell(t *testing.T) {
	// The recruiter column commonly holds two addresses separated by a newline.
	got := splitEmails("kushal@humorstech.com \nKushalVyas@4mirrortech.com")
	want := []string{"kushal@humorstech.com", "kushalvyas@4mirrortech.com"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if len(splitEmails("Talent Search Email / Recruiter Email")) != 0 {
		t.Fatal("header text must not yield an address")
	}
}

func TestQualifiesEnforcesFloorAndRole(t *testing.T) {
	cases := []struct {
		name string
		s    Screen
		want bool
	}{
		{"pays above floor", Screen{HiresSDE: true, Fresher: true, MinLPA: 12, MaxLPA: 18}, true},
		{"band tops the floor but midpoint under it", Screen{HiresSDE: true, Fresher: true, MinLPA: 6, MaxLPA: 14}, false},
		{"pays well but no freshers", Screen{HiresSDE: true, Fresher: false, MinLPA: 20, MaxLPA: 30}, false},
		{"not a tech employer", Screen{HiresSDE: false, Fresher: true, MinLPA: 20, MaxLPA: 30}, false},
	}
	for _, c := range cases {
		if got := c.s.Qualifies(); got != c.want {
			t.Errorf("%s: Qualifies() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestBestEmailPrefersDeliverable(t *testing.T) {
	row := Row{Emails: []string{"dead@nowhere.invalid", "good@example.com"}}
	verdicts := map[string]Verdict{
		"dead@nowhere.invalid": {Email: "dead@nowhere.invalid", Valid: false, Code: "no_mx"},
		"good@example.com":     {Email: "good@example.com", Valid: true, Code: "ok"},
	}
	if got, v := BestEmail(row, verdicts); got != "good@example.com" || v == nil || !v.Valid {
		t.Fatalf("BestEmail = %q, want the deliverable address", got)
	}

	// Every address known bad → no usable address at all.
	allBad := map[string]Verdict{
		"dead@nowhere.invalid": {Valid: false},
		"good@example.com":     {Valid: false},
	}
	if got, _ := BestEmail(row, allBad); got != "" {
		t.Fatalf("BestEmail = %q, want empty when all addresses are bad", got)
	}

	// Unchecked addresses fall back to the first one, pending verification.
	if got, v := BestEmail(row, map[string]Verdict{}); got != "dead@nowhere.invalid" || v != nil {
		t.Fatalf("BestEmail = %q (verdict %v), want the first unchecked address", got, v)
	}
}

func TestBuildDropsFailedScreenAndUndeliverable(t *testing.T) {
	rows := []Row{
		{Company: "GoodCo", Emails: []string{"hr@goodco.com"}},
		{Company: "LowPayCo", Emails: []string{"hr@lowpay.com"}},
		{Company: "DeadCo", Emails: []string{"hr@dead.com"}},
	}
	verdicts := map[string]Verdict{
		"hr@goodco.com": {Valid: true, Code: "ok"},
		"hr@lowpay.com": {Valid: true, Code: "ok"},
		"hr@dead.com":   {Valid: false, Code: "no_mx"},
	}
	screens := map[string]Screen{
		"goodco":   {Company: "GoodCo", HiresSDE: true, Fresher: true, MinLPA: 14, MaxLPA: 20},
		"lowpayco": {Company: "LowPayCo", HiresSDE: true, Fresher: true, MinLPA: 4, MaxLPA: 8},
		"deadco":   {Company: "DeadCo", HiresSDE: true, Fresher: true, MinLPA: 20, MaxLPA: 30},
	}

	got, prog := Build(rows, verdicts, screens, Filter{OnlyReady: true})
	if len(got) != 1 || got[0].Company != "GoodCo" {
		t.Fatalf("Build = %+v, want only GoodCo", got)
	}
	if prog.Shortlisted != 1 || prog.Rows != 3 {
		t.Fatalf("progress = %+v, want 1 shortlisted of 3 rows", prog)
	}

	// A caller cannot lower the floor below FloorLPA.
	if got, _ := Build(rows, verdicts, screens, Filter{MinLPA: 1, OnlyReady: true}); len(got) != 1 {
		t.Fatalf("floor is not enforced: got %d rows, want 1", len(got))
	}
	// Already-emailed contacts are excluded.
	if got, _ := Build(rows, verdicts, screens, Filter{OnlyReady: true, Exclude: map[string]bool{"hr@goodco.com": true}}); len(got) != 0 {
		t.Fatalf("excluded contact still listed: %+v", got)
	}
}
