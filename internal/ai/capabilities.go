/*
File: internal/ai/capabilities.go
Description: Shared AI capability manifest and <load_history> directive parser so all
AI modes (skills, single-shot, auto-reply) know their exact capabilities and limits.
*/

package ai

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"telepilot/internal/storage"
)

// HistoryDirective represents a <load_history .../> request emitted by an AI agent.
type HistoryDirective struct {
	Count      int
	Date       string
	ParsedDate time.Time
}

var (
	historyTagRegex      = regexp.MustCompile(`(?is)<load_history\s+([^>]*?)(?:/>|>\s*</load_history>)`)
	historyAttrCountRe   = regexp.MustCompile(`(?i)count\s*=\s*["']([^"']+)["']`)
	historyAttrDateRegex = regexp.MustCompile(`(?i)date\s*=\s*["']([^"']+)["']`)
)

/*
ExtractHistoryDirective inspects raw LLM output for a <load_history count="N" /> or
<load_history date="YYYY-MM-DD" /> directive.
*/
func ExtractHistoryDirective(rawText string) (*HistoryDirective, bool) {
	match := historyTagRegex.FindStringSubmatch(rawText)
	if len(match) < 2 {
		return nil, false
	}
	attrs := match[1]
	dir := &HistoryDirective{}

	if m := historyAttrCountRe.FindStringSubmatch(attrs); len(m) >= 2 {
		if n, err := strconv.Atoi(strings.TrimSpace(m[1])); err == nil && n > 0 {
			if n > 1000 {
				n = 1000
			}
			dir.Count = n
		}
	}
	if m := historyAttrDateRegex.FindStringSubmatch(attrs); len(m) >= 2 {
		dateStr := strings.TrimSpace(m[1])
		if parsed, err := time.ParseInLocation("2006-01-02", dateStr, time.Local); err == nil {
			dir.Date = dateStr
			dir.ParsedDate = parsed
		}
	}

	if dir.Count == 0 && dir.Date == "" {
		return nil, false
	}
	return dir, true
}

/*
CapabilitiesPrompt builds a concise, accurate manifest of what the AI can and cannot do
for the current chat context, including the currently loaded message window.
*/
func CapabilitiesPrompt(messages []storage.LiveMessage, allowSchedule, allowLoadHistory bool) string {
	var b strings.Builder
	b.WriteString("<capabilities>\n")
	if len(messages) == 0 {
		b.WriteString("Loaded chat context: 0 messages.\n")
	} else {
		oldest := strings.TrimSpace(messages[0].Timestamp)
		newest := strings.TrimSpace(messages[len(messages)-1].Timestamp)
		fmt.Fprintf(&b, "Loaded chat context: %d message(s) (from %s to %s).\n", len(messages), oldest, newest)
	}

	b.WriteString("What you CAN do:\n")
	b.WriteString("- Read all loaded messages, sender names, timestamps, and media metadata/captions in this chat.\n")
	if allowLoadHistory {
		b.WriteString("- Load older chat history if the task/skill requires messages earlier than the loaded window:\n")
		b.WriteString("  Output ONLY <load_history count=\"200\" /> (1..1000 older messages) OR <load_history date=\"YYYY-MM-DD\" /> (jump to a date).\n")
	}
	if allowSchedule {
		b.WriteString("- Schedule future or recurring tasks using <schedule in=\"...\" at=\"...\" repeat=\"...\" count=\"...\" until=\"...\">prompt</schedule>.\n")
	}

	b.WriteString("What you CANNOT do:\n")
	b.WriteString("- You CANNOT access other Telegram chats, contacts, or arbitrary Telegram API endpoints outside this chat.\n")
	b.WriteString("- You CANNOT download uncached media files on your own or execute shell/code actions unless explicitly provided.\n")
	b.WriteString("- You MUST NOT invent or guess messages outside the loaded window; if you need older context")
	if allowLoadHistory {
		b.WriteString(", emit <load_history ... /> instead of guessing.\n")
	} else {
		b.WriteString(", state that only the recent window is available.\n")
	}
	b.WriteString("</capabilities>\n")
	return b.String()
}
