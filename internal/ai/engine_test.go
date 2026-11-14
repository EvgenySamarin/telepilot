/*
File: internal/ai/engine_test.go
Description: Unit tests for AgyCLIError capture, stderr/stdout trace details, and timeout reporting.
*/

package ai

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"telepilot/internal/config"
)

func TestCallAgyCLICapturesExactStderrAndTraceDetails(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "fake-agy.sh")
	body := `#!/bin/sh
echo "partial stdout line"
echo "Error: 429 Resource Exhausted from model backend" >&2
echo "Detail: quota exceeded for gemini-4-pro-preview in region us-central1" >&2
exit 3
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write fake agy: %v", err)
	}

	eng := NewEngine(&config.AppConfig{
		ProjectRoot:   tmp,
		AgyCLIBinPath: script,
	})

	_, err := eng.callAgyCLI(context.Background(), "hello", "")
	if err == nil {
		t.Fatalf("expected error from fake agy, got nil")
	}

	var cliErr *AgyCLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *AgyCLIError, got %T: %v", err, err)
	}
	if cliErr.TimedOut {
		t.Fatalf("expected TimedOut=false")
	}
	if !strings.Contains(err.Error(), "Detail: quota exceeded for gemini-4-pro-preview in region us-central1") {
		t.Fatalf("expected full error message without 50-char truncation, got: %s", err.Error())
	}

	details := cliErr.TraceDetails()
	joined := strings.Join(details, "\n")
	if !strings.Contains(joined, "agy stderr: Error: 429 Resource Exhausted from model backend") {
		t.Fatalf("expected stderr line 1 in trace details, got:\n%s", joined)
	}
	if !strings.Contains(joined, "agy stderr: Detail: quota exceeded for gemini-4-pro-preview in region us-central1") {
		t.Fatalf("expected stderr line 2 in trace details, got:\n%s", joined)
	}
	if !strings.Contains(joined, "agy stdout: partial stdout line") {
		t.Fatalf("expected stdout line in trace details, got:\n%s", joined)
	}
}

func TestCallAgyCLIReportsTimeout(t *testing.T) {
	tmp := t.TempDir()
	script := filepath.Join(tmp, "slow-agy.sh")
	body := `#!/bin/sh
echo "waiting for model response..." >&2
sleep 5
`
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatalf("write slow agy: %v", err)
	}

	eng := NewEngine(&config.AppConfig{
		ProjectRoot:   tmp,
		AgyCLIBinPath: script,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	_, err := eng.callAgyCLI(ctx, "hello", "")
	if err == nil {
		t.Fatalf("expected timeout error, got nil")
	}

	var cliErr *AgyCLIError
	if !errors.As(err, &cliErr) {
		t.Fatalf("expected *AgyCLIError, got %T: %v", err, err)
	}
	if !cliErr.TimedOut {
		t.Fatalf("expected TimedOut=true, got false (err=%v)", err)
	}
	if !strings.Contains(err.Error(), "agy CLI timed out") {
		t.Fatalf("expected timeout message, got: %s", err.Error())
	}
}
