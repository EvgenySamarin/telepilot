/*
File: internal/tui/view_ai_output.go
Description: Tab 3 AI Output renderer displaying generated skill and summary markdown
output with vertical scrolling.
*/

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"telepilot/internal/storage"
)

// renderSkillOutputTab renders Tab 3: free-form AI Skill Output and execution metadata for the selected chat.
func (m *Model) renderSkillOutputTab(rec *storage.SkillOutputRecord, innerW int, visibleRows int) []string {
	ch := m.activeChat()
	chatTitle := "Selected Chat"
	if ch != nil {
		chatTitle = sanitizeTerminalText(ch.Title)
	}

	var out []string
	for _, hl := range wrapTextBlock(m.T("output_tab_hdr", chatTitle), innerW) {
		out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(hl, innerW)))
	}
	out = append(out, "")

	if rec == nil {
		out = append(out, truncateSingleLine(m.T("output_empty"), innerW))
		return out
	}

	cardW := innerW - 4
	if cardW < 20 {
		cardW = 20
	}

	skillHash := rec.SkillHash
	if skillHash == "" {
		skillHash = "custom"
	}
	outFile := rec.OutputPath
	if outFile == "" {
		outFile = "(in-memory)"
	}

	metaBody := m.T(
		"output_meta_body",
		sanitizeTerminalText(rec.ChatTitle),
		rec.ChatID,
		rec.Skill,
		skillHash,
		rec.ModelUsed,
		rec.CreatedAt,
		rec.MessageCount,
		outFile,
	)
	if strings.TrimSpace(rec.SkillDesc) != "" {
		metaBody += "\nDescription:  " + strings.TrimSpace(rec.SkillDesc)
	}
	if strings.TrimSpace(rec.Notes) != "" {
		metaBody += "\n" + m.T("sum_extra_notes") + strings.TrimSpace(rec.Notes)
	}

	out = append(out, renderInteriorCard(m.T("output_card_meta"), metaBody, cardW, colBlue)...)
	out = append(out, "")

	out = append(out, renderInteriorCard(m.T("output_card_result", rec.Skill), rec.Output, cardW, colCyan)...)
	return out
}
