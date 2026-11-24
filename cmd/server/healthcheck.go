package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// healthcheck probes a running server and fails unless it reports ready.
// It exists so container images without a shell or curl can still define
// a HEALTHCHECK. It reads PORT and TLS_ENABLED like the server does, but
// nothing else, so it works without the server's secrets.
func healthcheck(args []string, stdout io.Writer) error {
	fs := flag.NewFlagSet("healthcheck", flag.ContinueOnError)
	fs.SetOutput(stdout)
	url := fs.String("url", defaultHealthURL(), "health endpoint to probe")
	timeout := fs.Duration("timeout", 3*time.Second, "time to wait for a response")
	if err := fs.Parse(args); err != nil {
		return err
	}

	client := &http.Client{Timeout: *timeout}
	if strings.HasPrefix(*url, "https://localhost") || strings.HasPrefix(*url, "https://127.0.0.1") {
		// The server's certificate is issued for its public name, not for
		// localhost, and the probe never leaves the container.
		client.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}} // #nosec G402 -- loopback probe
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *url, nil)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("healthcheck: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("healthcheck: %s returned %s", *url, resp.Status)
	}
	return nil
}

func defaultHealthURL() string {
	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "8080"
	}
	scheme := "http"
	if tlsOn, _ := strconv.ParseBool(os.Getenv("TLS_ENABLED")); tlsOn {
		scheme = "https"
	}
	return scheme + "://localhost:" + port + "/health/ready"
}
