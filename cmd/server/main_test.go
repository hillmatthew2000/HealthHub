package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunRejectsUnknownCommands(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"frobnicate"}, strings.NewReader(""), &out); err == nil {
		t.Error("unknown command accepted")
	}
	if err := run([]string{"help"}, strings.NewReader(""), &out); err != nil || !strings.Contains(out.String(), "create-admin") {
		t.Errorf("help = %q, %v", out.String(), err)
	}
}

// TestCreateAdminValidatesInput covers the checks made before the
// configuration is loaded or the database is opened.
func TestCreateAdminValidatesInput(t *testing.T) {
	const good = "a long enough password"
	args := []string{"-email", "Admin@Example.org", "-first-name", "Ada", "-last-name", "Admin"}
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{"no password", args, "", "no password"},
		{"short password", args, "short\n", "password must be"},
		{"missing names", []string{"-email", "a@example.org"}, good + "\n", "-first-name is invalid"},
		{"bad email", []string{"-email", "nope", "-first-name", "A", "-last-name", "B"}, good + "\n", "-email is invalid"},
		{"extra argument", append(args, "now"), good + "\n", "unexpected argument"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(adminPasswordEnv, "")
			err := run(append([]string{"create-admin"}, tt.args...), strings.NewReader(tt.stdin), &bytes.Buffer{})
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want it to mention %q", err, tt.want)
			}
		})
	}
}

func TestReadPassword(t *testing.T) {
	t.Setenv(adminPasswordEnv, "from the environment")
	if p, err := readPassword(strings.NewReader("ignored\n")); err != nil || p != "from the environment" {
		t.Errorf("readPassword = %q, %v", p, err)
	}
}
