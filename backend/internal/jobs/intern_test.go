package jobs

import "testing"

// The Open list is for full-time fresher roles. Internships come in two ways:
// typed INTERN by the actor, or full-time-typed but an internship by title.
func TestIsInternship(t *testing.T) {
	internships := []Job{
		{Title: "Software Engineer", EmploymentType: "INTERN"},
		{Title: "Software Engineer", EmploymentType: "intern"},
		{Title: "Software Engineer Intern", EmploymentType: "FULL_TIME"},
		{Title: "Backend Development Internship", EmploymentType: "FULL_TIME"},
		{Title: "Graduate Engineer Trainee", EmploymentType: "FULL_TIME"},
		{Title: "Engineering Apprentice", EmploymentType: "FULL_TIME"},
		{Title: "SDE Co-op", EmploymentType: "FULL_TIME"},
		{Title: "Summer Analyst, Technology", EmploymentType: "FULL_TIME"},
	}
	for _, j := range internships {
		if !j.IsInternship() {
			t.Errorf("IsInternship(%q / %q) = false, want true", j.Title, j.EmploymentType)
		}
	}

	// These must NOT be dropped — "intern" appears inside a longer word, or the
	// role is a genuine full-time position.
	fullTime := []Job{
		{Title: "Software Engineer", EmploymentType: "FULL_TIME"},
		{Title: "International Payments Engineer", EmploymentType: "FULL_TIME"},
		{Title: "Internal Tools Developer", EmploymentType: "FULL_TIME"},
		{Title: "Backend Engineer, Internationalization", EmploymentType: "FULL_TIME"},
		{Title: "Associate Software Engineer", EmploymentType: "FULL_TIME"},
	}
	for _, j := range fullTime {
		if j.IsInternship() {
			t.Errorf("IsInternship(%q / %q) = true, want false", j.Title, j.EmploymentType)
		}
	}
}

func TestDefaultRolesExcludeInterns(t *testing.T) {
	for _, r := range DefaultRoles {
		if (Job{Title: r}).IsInternship() {
			t.Errorf("DefaultRoles contains an internship search term: %q", r)
		}
	}
}

func TestInAllowedCity(t *testing.T) {
	for _, locs := range [][]string{
		{"Bengaluru, Karnataka, India"}, {"Gurugram, Haryana, India"},
		{"Mumbai, India", "New Delhi, Delhi, India"},
	} {
		if !inAllowedCity(locs) {
			t.Errorf("%v should be allowed", locs)
		}
	}
	for _, locs := range [][]string{{"India"}, {"Mumbai, Maharashtra, India"}, nil} {
		if inAllowedCity(locs) {
			t.Errorf("%v should be rejected", locs)
		}
	}
}
