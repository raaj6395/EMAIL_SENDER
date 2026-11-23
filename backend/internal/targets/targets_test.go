package targets

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// writeStore seeds a targets cache file for testing.
func writeStore(t *testing.T, cs []Company) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "targets.json")
	s := store{}
	for _, c := range cs {
		s[normName(c.Name)] = c
	}
	b, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func sample() []Company {
	return []Company{
		{Name: "Google", MinLPA: 45, MaxLPA: 60, Category: CatMNC, Tech: []string{"Go", "Kubernetes"}},
		{Name: "Swiggy", MinLPA: 18, MaxLPA: 30, Category: CatHypergrowth, HQ: "Bengaluru"},
		{Name: "Zoho", MinLPA: 12, MaxLPA: 20, Category: CatProduct, HQ: "Chennai"},
		{Name: "PhonePe", MinLPA: 18, MaxLPA: 30, Category: CatFintech, Note: "UPI payments platform"},
		{Name: "TinyCo", MinLPA: 5, MaxLPA: 8, Category: CatOther},
	}
}

func names(cs []Company) []string {
	out := make([]string, len(cs))
	for i, c := range cs {
		out[i] = c.Name
	}
	return out
}

func TestFilterByMinLPA(t *testing.T) {
	path := writeStore(t, sample())

	// Qualification is by band MIDPOINT, so a company is listed only when a
	// typical offer clears the floor: Swiggy/PhonePe 18-30 → 24, Zoho 12-20 → 16.
	cases := []struct {
		floor float64
		want  []string
	}{
		// Sorted by midpoint, descending.
		{12, []string{"Google", "PhonePe", "Swiggy", "Zoho"}},
		// Zoho (mid 16) drops out; 18-30 (mid 24) still counts as a ~20 payer.
		{20, []string{"Google", "PhonePe", "Swiggy"}},
		// mid 24 < 25, so only Google (mid 52.5) survives.
		{25, []string{"Google"}},
		{45, []string{"Google"}},
		// A floor of 0 does NOT mean "no floor": FloorLPA still applies, so
		// TinyCo (mid 6.5) stays out. See TestFloorIsNotUserLowerable.
		{0, []string{"Google", "PhonePe", "Swiggy", "Zoho"}},
	}

	for _, c := range cases {
		got, err := Load(path, Filter{MinLPA: c.floor})
		if err != nil {
			t.Fatalf("floor %v: %v", c.floor, err)
		}
		gotNames := names(got)
		if len(gotNames) != len(c.want) {
			t.Errorf("floor %v: got %v, want %v", c.floor, gotNames, c.want)
			continue
		}
		for i := range gotNames {
			if gotNames[i] != c.want[i] {
				t.Errorf("floor %v: got %v, want %v", c.floor, gotNames, c.want)
				break
			}
		}
	}
}

func TestFilterByCategory(t *testing.T) {
	path := writeStore(t, sample())

	got, err := Load(path, Filter{MinLPA: 12, Categories: []string{CatFintech, CatHypergrowth}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"PhonePe", "Swiggy"}
	if len(got) != 2 || got[0].Name != want[0] || got[1].Name != want[1] {
		t.Errorf("got %v, want %v", names(got), want)
	}
}

func TestFilterByQuery(t *testing.T) {
	path := writeStore(t, sample())

	cases := map[string][]string{
		"payments":   {"PhonePe"},                   // matches note
		"chennai":    {"Zoho"},                      // matches HQ
		"kubernetes": {"Google"},                    // matches tech
		"o":          {"Google", "PhonePe", "Zoho"}, // substring on name
	}
	for q, want := range cases {
		got, err := Load(path, Filter{MinLPA: 12, Query: q})
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != len(want) {
			t.Errorf("q=%q: got %v, want %v", q, names(got), want)
		}
	}
}

func TestMidLPA(t *testing.T) {
	cases := []struct {
		c       Company
		wantMid float64
	}{
		{Company{MinLPA: 18, MaxLPA: 30}, 24},
		{Company{MinLPA: 20, MaxLPA: 0}, 20}, // open-ended band → the min
		{Company{MinLPA: 15, MaxLPA: 15}, 15},
	}
	for _, c := range cases {
		if got := c.c.MidLPA(); got != c.wantMid {
			t.Errorf("MidLPA(%v) = %v, want %v", c.c, got, c.wantMid)
		}
	}
}

func TestUpsertPreservesExistingUnlessOverwrite(t *testing.T) {
	s := store{}
	upsertLocked(s, []Company{{Name: "Swiggy", MinLPA: 18, MaxLPA: 30}}, false)
	original := s[normName("Swiggy")].AddedAt

	// Same name, different case, no overwrite → keeps the original band.
	upsertLocked(s, []Company{{Name: "swiggy", MinLPA: 1, MaxLPA: 2}}, false)
	if got := s[normName("Swiggy")].MinLPA; got != 18 {
		t.Errorf("non-overwrite upsert changed the band to %v", got)
	}
	if len(s) != 1 {
		t.Errorf("case-variant name created a duplicate: %d entries", len(s))
	}

	// With overwrite → new band, but the original discovery time is kept.
	upsertLocked(s, []Company{{Name: "Swiggy", MinLPA: 22, MaxLPA: 35}}, true)
	got := s[normName("Swiggy")]
	if got.MinLPA != 22 {
		t.Errorf("overwrite upsert did not update the band: %v", got.MinLPA)
	}
	if got.AddedAt != original {
		t.Errorf("overwrite lost the original AddedAt")
	}
}

func TestUpsertDefaults(t *testing.T) {
	s := store{}
	upsertLocked(s, []Company{{Name: "X"}, {Name: "  "}}, false)
	if len(s) != 1 {
		t.Fatalf("blank name was stored: %d entries", len(s))
	}
	c := s["x"]
	if c.Category != CatOther || c.Source != "ai" || c.AddedAt == "" {
		t.Errorf("defaults not applied: %+v", c)
	}
}

func TestRemove(t *testing.T) {
	path := writeStore(t, sample())
	if err := Remove(path, "SWIGGY"); err != nil { // case-insensitive
		t.Fatal(err)
	}
	got, err := Load(path, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range got {
		if normName(c.Name) == "swiggy" {
			t.Error("Remove did not delete the company")
		}
	}
	if Count(path) != 4 {
		t.Errorf("Count = %d, want 4", Count(path))
	}
}

func TestLoadMissingFileIsEmpty(t *testing.T) {
	got, err := Load(filepath.Join(t.TempDir(), "nope.json"), Filter{})
	if err != nil {
		t.Fatalf("missing file should not error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %d companies from a missing file", len(got))
	}
}

func TestNormalizeCategory(t *testing.T) {
	cases := map[string]string{
		"hypergrowth": CatHypergrowth,
		"unicorn":     CatHypergrowth,
		"FAANG":       CatMNC,
		"Big Tech":    CatMNC,
		"r&d":         CatGCC,
		"saas":        CatProduct,
		"banking":     CatFintech,
		"nonsense":    CatOther,
	}
	for in, want := range cases {
		if got := normalizeCategory(in); got != want {
			t.Errorf("normalizeCategory(%q) = %q, want %q", in, got, want)
		}
	}
}

// The floor is structural: the model is asked for companies above it but does
// not reliably obey, so no read path may return a company below it — including
// a caller that explicitly passes a lower minLPA.
func TestFloorIsNotUserLowerable(t *testing.T) {
	path := writeStore(t, []Company{
		{Name: "Stripe", MinLPA: 30, MaxLPA: 45, Category: CatFintech},
		{Name: "MidCo", MinLPA: 11, MaxLPA: 15, Category: CatProduct},  // mid 13 — in
		{Name: "LowCo", MinLPA: 6, MaxLPA: 10, Category: CatProduct},   // mid 8 — out
		{Name: "ServicesCo", MinLPA: 3, MaxLPA: 5, Category: CatOther}, // mid 4 — out
	})

	for _, minLPA := range []float64{0, -5, 3, 11.9, FloorLPA} {
		got, err := Load(path, Filter{MinLPA: minLPA})
		if err != nil {
			t.Fatal(err)
		}
		for _, c := range got {
			if c.MidLPA() < FloorLPA {
				t.Errorf("Load(minLPA=%v) returned %s at %.1f LPA, below the %.0f floor",
					minLPA, c.Name, c.MidLPA(), FloorLPA)
			}
		}
		if len(got) != 2 {
			t.Errorf("Load(minLPA=%v) returned %d companies, want 2 (Stripe, MidCo)", minLPA, len(got))
		}
	}

	// A caller can still raise the bar above the floor.
	got, err := Load(path, Filter{MinLPA: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Name != "Stripe" {
		t.Errorf("Load(minLPA=25) = %v, want just Stripe", got)
	}
}
