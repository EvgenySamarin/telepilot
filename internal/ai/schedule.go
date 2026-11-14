/*
File: internal/ai/schedule.go
Description: Parser for <schedule> directives in AI replies, handling relative durations,
wall-clock timestamps, and directive tag extraction.
*/

package ai

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ScheduleDirective represents an extracted schedule instruction from an AI agent's response.
type ScheduleDirective struct {
	InStr      string        `json:"in,omitempty"`
	AtStr      string        `json:"at,omitempty"`
	RepeatStr  string        `json:"repeat,omitempty"`
	MaxRuns    int           `json:"max_runs,omitempty"`
	Until      *time.Time    `json:"until,omitempty"`
	TaskPrompt string        `json:"task_prompt"`
	DueAt      time.Time     `json:"due_at"`
	Interval   time.Duration `json:"interval,omitempty"`
}

var scheduleTagRegex = regexp.MustCompile(`(?is)<\s*schedule\s+([^>]+)>(.*?)<\s*/\s*schedule\s*>`)
var attrInRegex = regexp.MustCompile(`(?i)\bin\s*=\s*["']?([^"'\s>]+)["']?`)
var attrAtRegex = regexp.MustCompile(`(?i)\bat\s*=\s*["']?([^"'\s>]+)["']?`)
var attrRepeatRegex = regexp.MustCompile(`(?i)\brepeat\s*=\s*["']?([^"'\s>]+)["']?`)
var attrCountRegex = regexp.MustCompile(`(?i)\b(?:count|max|times)\s*=\s*["']?(\d+)["']?`)
var attrUntilRegex = regexp.MustCompile(`(?i)\buntil\s*=\s*["']?([^"'\s>]+)["']?`)

/*
ParseScheduleDuration parses user- and agent-friendly duration strings:
e.g. "30s", "15m", "2h", "1d", "3d", "hourly", "daily", "weekly".
*/
func ParseScheduleDuration(s string) (time.Duration, error) {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "none" || s == "never" || s == "0" {
		return 0, nil
	}
	switch s {
	case "hourly":
		return time.Hour, nil
	case "daily":
		return 24 * time.Hour, nil
	case "weekly":
		return 7 * 24 * time.Hour, nil
	}

	if strings.HasSuffix(s, "d") {
		numStr := strings.TrimSuffix(s, "d")
		days, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid day duration %q: %w", s, err)
		}
		return time.Duration(days * float64(24*time.Hour)), nil
	}

	if strings.HasSuffix(s, "w") {
		numStr := strings.TrimSuffix(s, "w")
		weeks, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid week duration %q: %w", s, err)
		}
		return time.Duration(weeks * float64(7*24*time.Hour)), nil
	}

	return time.ParseDuration(s)
}

/*
ParseScheduleWallClock parses wall-clock time strings (e.g. "09:00", "9:00", "14:30")
and returns the target time. If the time has already passed today, it targets tomorrow.
*/
func ParseScheduleWallClock(atStr string, now time.Time) (time.Time, error) {
	atStr = strings.TrimSpace(atStr)
	parts := strings.Split(atStr, ":")
	if len(parts) < 2 || len(parts) > 3 {
		return time.Time{}, fmt.Errorf("invalid wall-clock format %q (expected HH:MM)", atStr)
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil || hour < 0 || hour > 23 {
		return time.Time{}, fmt.Errorf("invalid hour in %q", atStr)
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil || minute < 0 || minute > 59 {
		return time.Time{}, fmt.Errorf("invalid minute in %q", atStr)
	}

	second := 0
	if len(parts) == 3 {
		second, err = strconv.Atoi(parts[2])
		if err != nil || second < 0 || second > 59 {
			return time.Time{}, fmt.Errorf("invalid second in %q", atStr)
		}
	}

	target := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, second, 0, now.Location())
	if !target.After(now) {
		target = target.AddDate(0, 0, 1)
	}
	return target, nil
}

/*
ExtractScheduleDirective inspects the raw text produced by an AI agent for a <schedule> tag.
If found, it parses the delay, interval, and inner task prompt, and returns clean text
(with the schedule tag stripped) ready for user display or chat delivery.
*/
func ExtractScheduleDirective(rawText string, now time.Time) (string, *ScheduleDirective, bool) {
	match := scheduleTagRegex.FindStringSubmatch(rawText)
	if len(match) < 3 {
		return rawText, nil, false
	}

	rawTag := match[0]
	attrsStr := match[1]
	taskPrompt := strings.TrimSpace(match[2])

	atMatch := attrAtRegex.FindStringSubmatch(attrsStr)
	var atStr string
	if len(atMatch) >= 2 {
		atStr = strings.TrimSpace(atMatch[1])
	}

	inMatch := attrInRegex.FindStringSubmatch(attrsStr)
	var inStr string
	if len(inMatch) >= 2 {
		inStr = strings.TrimSpace(inMatch[1])
	}

	repeatMatch := attrRepeatRegex.FindStringSubmatch(attrsStr)
	var repeatStr string
	if len(repeatMatch) >= 2 {
		repeatStr = strings.TrimSpace(repeatMatch[1])
	}
	var repeatDur time.Duration
	if repeatStr != "" && repeatStr != "none" {
		repeatDur, _ = ParseScheduleDuration(repeatStr)
	}

	var maxRuns int
	if countMatch := attrCountRegex.FindStringSubmatch(attrsStr); len(countMatch) >= 2 {
		maxRuns, _ = strconv.Atoi(countMatch[1])
	}

	var untilTime *time.Time
	if untilMatch := attrUntilRegex.FindStringSubmatch(attrsStr); len(untilMatch) >= 2 {
		uStr := strings.TrimSpace(untilMatch[1])
		if uDur, err := ParseScheduleDuration(uStr); err == nil && uDur > 0 {
			t := now.Add(uDur)
			untilTime = &t
		} else if parsedAt, err := ParseScheduleWallClock(uStr, now); err == nil {
			untilTime = &parsedAt
		}
	}

	var dueAt time.Time
	if atStr != "" {
		if parsedAt, err := ParseScheduleWallClock(atStr, now); err == nil {
			dueAt = parsedAt
		}
	}

	if dueAt.IsZero() {
		if inStr == "" {
			inStr = "1m"
		}
		inDur, err := ParseScheduleDuration(inStr)
		if err != nil || inDur <= 0 {
			inDur = 1 * time.Minute
		}
		dueAt = now.Add(inDur)
	}

	directive := &ScheduleDirective{
		InStr:      inStr,
		AtStr:      atStr,
		RepeatStr:  repeatStr,
		MaxRuns:    maxRuns,
		Until:      untilTime,
		TaskPrompt: taskPrompt,
		DueAt:      dueAt,
		Interval:   repeatDur,
	}

	// Remove the tag from rawText and collapse multiple trailing blank lines
	clean := strings.Replace(rawText, rawTag, "", 1)
	clean = strings.TrimRight(clean, " \t\r\n")

	return clean, directive, true
}

