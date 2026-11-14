package ai

import (
	"strings"
	"testing"

	"telepilot/internal/storage"
)

func TestExtractHistoryDirective(t *testing.T) {
	dir, ok := ExtractHistoryDirective(`<load_history count="250" />`)
	if !ok || dir.Count != 250 || dir.Date != "" {
		t.Fatalf("expected count=250, got ok=%v dir=%+v", ok, dir)
	}

	// Count > 1000 is clamped to 1000
	dir, ok = ExtractHistoryDirective(`<load_history count='5000'></load_history>`)
	if !ok || dir.Count != 1000 {
		t.Fatalf("expected count clamped to 1000, got ok=%v dir=%+v", ok, dir)
	}

	// Date directive
	dir, ok = ExtractHistoryDirective(`Need earlier context: <load_history date="2026-10-15" />`)
	if !ok || dir.Date != "2026-10-15" || dir.ParsedDate.IsZero() {
		t.Fatalf("expected date=2026-10-15, got ok=%v dir=%+v", ok, dir)
	}

	// Invalid directive
	if _, ok := ExtractHistoryDirective(`<load_history date="not-a-date" />`); ok {
		t.Fatal("expected invalid date directive to be rejected")
	}
	if _, ok := ExtractHistoryDirective(`plain text without directive`); ok {
		t.Fatal("expected plain text to return false")
	}
}

func TestCapabilitiesPrompt(t *testing.T) {
	msgs := []storage.LiveMessage{
		{MessageID: 1, Timestamp: "2026-11-01 10:00"},
		{MessageID: 2, Timestamp: "2026-11-14 20:00"},
	}

	full := CapabilitiesPrompt(msgs, true, true)
	for _, want := range []string{
		"Loaded chat context: 2 message(s) (from 2026-11-01 10:00 to 2026-11-14 20:00)",
		"<load_history count=\"200\" />",
		"<schedule ",
		"What you CANNOT do:",
		"You MUST NOT invent or guess messages",
	} {
		if !strings.Contains(full, want) {
			t.Fatalf("expected capabilities prompt to contain %q, got:\n%s", want, full)
		}
	}

	restricted := CapabilitiesPrompt(nil, false, false)
	if strings.Contains(restricted, "<load_history") {
		t.Fatalf("did not expect <load_history when allowLoadHistory=false")
	}
	if strings.Contains(restricted, "<schedule") {
		t.Fatalf("did not expect <schedule when allowSchedule=false")
	}
	if !strings.Contains(restricted, "Loaded chat context: 0 messages.") {
		t.Fatalf("expected 0 messages notice, got:\n%s", restricted)
	}
}
