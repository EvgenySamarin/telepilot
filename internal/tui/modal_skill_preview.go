/*
File: internal/tui/modal_skill_preview.go
Description: Renderer for the scrollable Skill Preview modal.
*/

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"telepilot/internal/skills"
)

func (m *Model) renderSkillPreviewModal() string {
	if len(m.skillList) == 0 || m.skillCursor < 0 || m.skillCursor >= len(m.skillList) {
		return m.renderInputModal("─Skill Preview─", "No skill selected.", "", "Esc: Close")
	}
	sk := m.skillList[m.skillCursor]

	modalW := 96
	if m.width-4 < modalW {
		modalW = m.width - 4
	}
	if modalW < 40 {
		modalW = 40
	}
	innerW := modalW - 4

	modalH := m.height - 4
	if modalH < 14 {
		modalH = 14
	}
	innerH := modalH - 4

	var lines []string
	hdr := fmt.Sprintf("─Skill Preview: %s [%s]─", sk.Name, sk.Kind)
	lines = append(lines, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(hdr, innerW)))
	lines = append(lines, "")

	metaCol := lipgloss.NewStyle().Foreground(colYellow)
	lines = append(lines, metaCol.Render(fmt.Sprintf("Name        : %s  (%s)", sk.Name, sk.Hash)))
	lines = append(lines, metaCol.Render(fmt.Sprintf("Kind        : %s", sk.Kind)))
	if sk.Kind == skills.KindMedia {
		lines = append(lines, metaCol.Render(fmt.Sprintf("Target Media: %s", strings.Join(sk.Media, ", "))))
	}
	lines = append(lines, metaCol.Render(fmt.Sprintf("File Path   : %s", sk.Path)))
	lines = append(lines, metaCol.Render(fmt.Sprintf("Description : %s", sk.Description)))
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Foreground(colBlue).Bold(true).Render("Prompt Instructions (<skill>):"))
	lines = append(lines, lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", innerW)))

	bodyParagraphs := strings.Split(sk.Body, "\n")
	for _, p := range bodyParagraphs {
		if strings.TrimSpace(p) == "" {
			lines = append(lines, "")
			continue
		}
		for _, wl := range wrapTextBlock(p, innerW) {
			lines = append(lines, lipgloss.NewStyle().Foreground(colText).Render(wl))
		}
	}

	maxVisible := innerH - 1
	if maxVisible < 5 {
		maxVisible = 5
	}
	if m.previewScroll >= len(lines) {
		m.previewScroll = maxInt(0, len(lines)-1)
	}

	bodyLines := lines
	if m.previewScroll < len(bodyLines) {
		bodyLines = bodyLines[m.previewScroll:]
	}
	if len(bodyLines) > maxVisible {
		bodyLines = bodyLines[:maxVisible]
	}
	for len(bodyLines) < maxVisible {
		bodyLines = append(bodyLines, strings.Repeat(" ", innerW))
	}

	footer := lipgloss.NewStyle().Foreground(colMuted).Render(
		fmt.Sprintf("Esc/q/v: Close  │  e: $EDITOR  │  O: Mac Editor  │  o: Finder  │  j/k: Scroll [%d/%d]", m.previewScroll+1, maxInt(1, len(lines))),
	)
	bodyLines = append(bodyLines, "", footer)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(bodyLines, "\n"))
}
