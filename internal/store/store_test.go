package store

import (
	"testing"
)

func TestMigrationsAreOrderedAndNamed(t *testing.T) {
	ms, err := Migrations()
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) == 0 {
		t.Fatal("no migrations embedded")
	}
	for i, m := range ms {
		if m.Name == "" || m.SQL == "" {
			t.Errorf("migration %d is empty", m.Version)
		}
		if i > 0 && ms[i-1].Version >= m.Version {
			t.Errorf("migrations out of order: %d before %d", ms[i-1].Version, m.Version)
		}
	}
}

func TestNormaliseTelecom(t *testing.T) {
	tests := []struct{ system, in, want string }{
		{"phone", "+1 (555) 010-0200", "+15550100200"},
		{"phone", "555 0100", "5550100"},
		{"sms", "1+2", "12"},
		{"email", "  Jane.Doe@Example.COM ", "jane.doe@example.com"},
		{"url", "HTTPS://example.org", "https://example.org"},
	}
	for _, tt := range tests {
		if got := NormaliseTelecom(tt.system, tt.in); got != tt.want {
			t.Errorf("NormaliseTelecom(%q, %q) = %q, want %q", tt.system, tt.in, got, tt.want)
		}
	}
}

func TestNormaliseName(t *testing.T) {
	if got := NormaliseName("  van der   BERG "); got != "van der berg" {
		t.Errorf("NormaliseName = %q", got)
	}
}
