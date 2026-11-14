/*
File: internal/presentation/tui/view_trace.go
Description: Tab 4 Trace & Logs renderer and short duration formatter.
*/

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderTraceTab(innerW int) []string {
	var out []string
	headerTitle := "Real-Time Execution & MTProto Diagnostic Trace"
	out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render("▶ "+headerTitle))

	logPath := m.traceLogFilePath()
	relPath := logPath
	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, logPath); err == nil && !strings.HasPrefix(rel, "..") {
			relPath = rel
		}
	}

	actionsBar := fmt.Sprintf("%s %s  %s %s  %s %s  %s %s  %s %s",
		lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render("[o]"),
		lipgloss.NewStyle().Foreground(colText).Render("Finder"),
		lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render("[v]"),
		lipgloss.NewStyle().Foreground(colText).Render("Viewer"),
		lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render("[c]"),
		lipgloss.NewStyle().Foreground(colText).Render("Copy"),
		lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render("[Ctrl+L]"),
		lipgloss.NewStyle().Foreground(colText).Render("Clear"),
		lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render("[Ctrl+X]"),
		lipgloss.NewStyle().Foreground(colText).Render("Stop Tasks"),
	)
	metaLine := fmt.Sprintf("Log File: %s | %d lines | %s", relPath, len(m.traceLogs), actionsBar)
	out = append(out, lipgloss.NewStyle().Foreground(colMuted).Render(truncateSingleLine(metaLine, innerW)))
	out = append(out, lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", maxInt(10, innerW))))

	schedStyle := lipgloss.NewStyle().Foreground(colYellow)
	stopStyle := lipgloss.NewStyle().Foreground(colRed).Bold(true)
	aiStyle := lipgloss.NewStyle().Foreground(colPurple)
	sendStyle := lipgloss.NewStyle().Foreground(colGreen)
	dimStyle := lipgloss.NewStyle().Foreground(colMuted)

	for _, line := range m.traceLogs {
		trimmed := truncateSingleLine(line, innerW-4)
		var styledLine string
		switch {
		case strings.Contains(line, "🛑") || strings.Contains(line, "Stopped") || strings.Contains(line, "Cancelled"):
			styledLine = "  " + stopStyle.Render(trimmed)
		case strings.Contains(line, "⏰") || strings.Contains(line, "Task scheduled") || strings.Contains(line, "Executed scheduled"):
			styledLine = "  " + schedStyle.Render(trimmed)
		case strings.Contains(line, "failed") || strings.Contains(line, "error") || strings.Contains(line, "Failed"):
			styledLine = "  " + stopStyle.Render(trimmed)
		case strings.Contains(line, "Auto-Reply") || strings.Contains(line, "AI "):
			styledLine = "  " + aiStyle.Render(trimmed)
		case strings.Contains(line, "Sent message") || strings.Contains(line, "Live message"):
			styledLine = "  " + sendStyle.Render(trimmed)
		default:
			styledLine = "  " + dimStyle.Render(trimmed)
		}
		out = append(out, styledLine)
	}
	return out
}

func formatDurationShort(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	hours := int(d.Hours())
	mins := int(d.Minutes()) % 60
	if mins > 0 {
		return fmt.Sprintf("%dh%dm", hours, mins)
	}
	return fmt.Sprintf("%dh", hours)
}
