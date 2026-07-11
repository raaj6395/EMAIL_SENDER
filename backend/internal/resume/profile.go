package resume

import (
	"encoding/json"
	"os"
)

// Profile is the user's context used to personalize outgoing emails.
// It is extracted (best-effort) from the resume and then edited by the user
// in the UI before any email is sent.
type Profile struct {
	Name       string   `json:"name"`
	Email      string   `json:"email"`      // the user's own contact email (for signature)
	Phone      string   `json:"phone"`      // optional, for signature
	TargetRole string   `json:"targetRole"` // e.g. "Backend Engineer"
	Skills     []string `json:"skills"`
	Pitch      string   `json:"pitch"`     // one/two-line value proposition
	LinkedIn   string   `json:"linkedin"`  // full URL
	GitHub     string   `json:"github"`    // full URL
	Portfolio  string   `json:"portfolio"` // full URL / website
}

// DefaultProfile returns the pre-filled profile used to seed a fresh install,
// so the user doesn't start from a blank form. Values are editable in the UI.
func DefaultProfile() *Profile {
	return &Profile{
		Name:       "Ankit Raj",
		Email:      "ankitraj224020@gmail.com",
		Phone:      "9680905523",
		TargetRole: "Backend Engineer",
		Skills: []string{
			"Go", "Python", "C++", "JavaScript", "SQL", "gRPC", "Protobuf", "FastAPI",
			"Django", "Express.js", "Node.js", "PostgreSQL", "MySQL", "MongoDB", "Redis",
			"BigQuery", "Prisma ORM", "Docker", "Git", "Prometheus", "Grafana", "Postman",
			"REST APIs", "Microservices", "Authentication & Authorization", "JWT", "bcrypt",
			"RBAC", "Email OTP Verification", "Database Indexing", "Query Optimization",
			"Background Workers", "Full-Stack Development", "System Design",
			"Distributed Systems", "Data Structures & Algorithms", "Operating Systems",
			"DBMS", "Computer Networks", "Object-Oriented Programming (OOP)",
			"Analytics Backend Development", "Team Leadership", "Problem Solving",
			"Competitive Programming",
		},
		Pitch: "I am a final year B.Tech student in ECE at NIT Allahabad. I have worked as a " +
			"Backend Engineer Intern at Carousell and Propel, where I built backend features, " +
			"optimized database queries, built background workers, and contributed to service " +
			"migrations on large-scale production systems. I also led the development of the " +
			"official MNNIT Library Book Allotment System used by thousands of students. I enjoy " +
			"building reliable backend applications that handle real users and large amounts of data.",
		LinkedIn:  "https://www.linkedin.com/in/ankit-raj-572943230/",
		GitHub:    "https://github.com/raaj6395",
		Portfolio: "",
	}
}

// LoadProfile reads a saved profile from disk. Returns (nil, nil) if none exists yet.
func LoadProfile(path string) (*Profile, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var p Profile
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// SaveProfile writes the profile to disk as pretty-printed JSON.
func SaveProfile(path string, p *Profile) error {
	b, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
