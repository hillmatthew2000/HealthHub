package main

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
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

func TestHealthcheck(t *testing.T) {
	status := http.StatusOK
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health/ready" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(status)
	}))
	defer srv.Close()

	if err := run([]string{"healthcheck", "-url", srv.URL + "/health/ready"}, nil, io.Discard); err != nil {
		t.Errorf("healthy server: %v", err)
	}
	status = http.StatusServiceUnavailable
	if err := run([]string{"healthcheck", "-url", srv.URL + "/health/ready"}, nil, io.Discard); err == nil {
		t.Error("unready server reported healthy")
	}
	srv.Close()
	if err := run([]string{"healthcheck", "-url", srv.URL + "/health/ready", "-timeout", "1s"}, nil, io.Discard); err == nil {
		t.Error("unreachable server reported healthy")
	}
}

func TestDefaultHealthURL(t *testing.T) {
	t.Setenv("PORT", "9000")
	t.Setenv("TLS_ENABLED", "true")
	if got := defaultHealthURL(); got != "https://localhost:9000/health/ready" {
		t.Errorf("defaultHealthURL = %q", got)
	}
}
