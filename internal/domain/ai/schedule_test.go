package ai

import (
	"testing"
	"time"
)

func TestParseScheduleDuration(t *testing.T) {
	tests := []struct {
		input    string
		expected time.Duration
		wantErr  bool
	}{
		{"", 0, false},
		{"none", 0, false},
		{"30s", 30 * time.Second, false},
		{"15m", 15 * time.Minute, false},
		{"2h", 2 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"2d", 48 * time.Hour, false},
		{"hourly", time.Hour, false},
		{"daily", 24 * time.Hour, false},
		{"weekly", 7 * 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"invalid_duration", 0, true},
	}

	for _, tt := range tests {
		dur, err := ParseScheduleDuration(tt.input)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseScheduleDuration(%q) unexpected err: %v", tt.input, err)
		}
		if err == nil && dur != tt.expected {
			t.Errorf("ParseScheduleDuration(%q) = %v, expected %v", tt.input, dur, tt.expected)
		}
	}
}

func TestExtractScheduleDirective(t *testing.T) {
	now := time.Date(2026, 11, 8, 12, 0, 0, 0, time.UTC)

	t.Run("Extract valid one-shot reminder", func(t *testing.T) {
		raw := "Договорились, я напомню через 30 минут!\n\n<schedule in=\"30m\" repeat=\"none\">Проверить пирог</schedule>"
		clean, directive, found := ExtractScheduleDirective(raw, now)
		if !found {
			t.Fatalf("expected schedule directive to be found")
		}
		if directive == nil {
			t.Fatalf("expected directive not nil")
		}
		if directive.InStr != "30m" {
			t.Errorf("expected InStr='30m', got %q", directive.InStr)
		}
		if directive.TaskPrompt != "Проверить пирог" {
			t.Errorf("expected TaskPrompt='Проверить пирог', got %q", directive.TaskPrompt)
		}
		if expectedDue := now.Add(30 * time.Minute); !directive.DueAt.Equal(expectedDue) {
			t.Errorf("expected DueAt=%v, got %v", expectedDue, directive.DueAt)
		}
		if clean != "Договорились, я напомню через 30 минут!" {
			t.Errorf("unexpected clean text: %q", clean)
		}
	})

	t.Run("Extract repeating task with single quotes and daily repeat", func(t *testing.T) {
		raw := "Sure, I'll send you daily updates.\n<schedule in='1h' repeat='daily'>Send daily summary</schedule>\n"
		clean, directive, found := ExtractScheduleDirective(raw, now)
		if !found {
			t.Fatalf("expected schedule directive to be found")
		}
		if directive.RepeatStr != "daily" {
			t.Errorf("expected RepeatStr='daily', got %q", directive.RepeatStr)
		}
		if directive.Interval != 24*time.Hour {
			t.Errorf("expected Interval=24h, got %v", directive.Interval)
		}
		if clean != "Sure, I'll send you daily updates." {
			t.Errorf("unexpected clean text: %q", clean)
		}
	})

	t.Run("No schedule tag present", func(t *testing.T) {
		raw := "Just a normal chat message without any tags."
		clean, directive, found := ExtractScheduleDirective(raw, now)
		if found {
			t.Errorf("expected found=false for normal message")
		}
		if directive != nil {
			t.Errorf("expected nil directive")
		}
		if clean != raw {
			t.Errorf("expected clean text unchanged, got %q", clean)
		}
	})

	t.Run("Extract wall-clock time at=09:00 when now is 08:00 (today)", func(t *testing.T) {
		now8am := time.Date(2026, 11, 8, 8, 0, 0, 0, time.UTC)
		raw := "Буду присылать отчет каждое утро в 9:00!\n<schedule at=\"09:00\" repeat=\"daily\">Ежедневный утренний отчет</schedule>"
		clean, directive, found := ExtractScheduleDirective(raw, now8am)
		if !found || directive == nil {
			t.Fatalf("expected schedule directive to be found")
		}
		expectedDue := time.Date(2026, 11, 8, 9, 0, 0, 0, time.UTC)
		if !directive.DueAt.Equal(expectedDue) {
			t.Errorf("expected DueAt today at 09:00 (%v), got %v", expectedDue, directive.DueAt)
		}
		if directive.Interval != 24*time.Hour {
			t.Errorf("expected 24h interval, got %v", directive.Interval)
		}
		if clean != "Буду присылать отчет каждое утро в 9:00!" {
			t.Errorf("unexpected clean text: %q", clean)
		}
	})

	t.Run("Extract wall-clock time at=09:00 when now is 10:00 (tomorrow)", func(t *testing.T) {
		now10am := time.Date(2026, 11, 8, 10, 0, 0, 0, time.UTC)
		raw := "Напомню завтра в 9:00.\n<schedule at=\"09:00\" repeat=\"daily\">Сводка</schedule>"
		_, directive, found := ExtractScheduleDirective(raw, now10am)
		if !found || directive == nil {
			t.Fatalf("expected schedule directive to be found")
		}
		expectedDue := time.Date(2026, 11, 9, 9, 0, 0, 0, time.UTC)
		if !directive.DueAt.Equal(expectedDue) {
			t.Errorf("expected DueAt tomorrow at 09:00 (%v), got %v", expectedDue, directive.DueAt)
		}
	})
}


