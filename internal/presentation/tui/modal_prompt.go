package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderSingleShotModal() string {
	modalW := 82
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	ch := m.activeChat()
	chatTitle := "Selected Chat"
	var isRunning bool
	var curInstr string
	var queued []string
	var history []string
	if ch != nil {
		chatTitle = sanitizeTerminalText(ch.Title)
		r := m.getChatSingleShotRunner(ch.ChatID)
		r.mu.Lock()
		isRunning = r.running
		curInstr = r.currentInstr
		queued = append([]string{}, r.queue...)
		history = append([]string{}, r.history...)
		r.mu.Unlock()
	}

	var statusLines []string
	statusBorder := colBorder
	if isRunning {
		statusBorder = colYellow
		if m.cfg.IsRussian() {
			statusLines = append(statusLines, lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(
				fmt.Sprintf("⟳ Статус: [ВЫПОЛНЯЕТСЯ в чате '%s']", chatTitle),
			))
			statusLines = append(statusLines, fmt.Sprintf("Активная команда : %s", curInstr))
			if len(queued) > 0 {
				statusLines = append(statusLines, fmt.Sprintf("В очереди (%d)    : %s", len(queued), strings.Join(queued, " -> ")))
			}
		} else {
			statusLines = append(statusLines, lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(
				fmt.Sprintf("⟳ Status: [RUNNING in '%s']", chatTitle),
			))
			statusLines = append(statusLines, fmt.Sprintf("Active Instruction : %s", curInstr))
			if len(queued) > 0 {
				statusLines = append(statusLines, fmt.Sprintf("Queued Next (%d)    : %s", len(queued), strings.Join(queued, " -> ")))
			}
		}
	} else {
		if m.cfg.IsRussian() {
			statusLines = append(statusLines, lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(
				fmt.Sprintf("◉ Статус: [ОЖИДАНИЕ — чат '%s']", chatTitle),
			))
			if len(history) > 0 {
				statusLines = append(statusLines, fmt.Sprintf("Предыдущая команда: %s", strings.Join(history, " + ")))
			} else {
				statusLines = append(statusLines, "Нет активных разовых команд для этого чата.")
			}
		} else {
			statusLines = append(statusLines, lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(
				fmt.Sprintf("◉ Status: [IDLE — chat '%s']", chatTitle),
			))
			if len(history) > 0 {
				statusLines = append(statusLines, fmt.Sprintf("Previous Instruction: %s", strings.Join(history, " + ")))
			} else {
				statusLines = append(statusLines, "No active single-shot instruction running for this chat.")
			}
		}
	}

	var wrappedStatus []string
	for _, sl := range statusLines {
		wrappedStatus = append(wrappedStatus, wrapTextBlock(sl, innerW-4)...)
	}
	statusBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(statusBorder).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(wrappedStatus, "\n"))

	wrappedInput := wrapTextBlock(m.inputBuffer+"█", innerW-4)
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colCyan).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(wrappedInput, "\n"))

	lines := []string{
		lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("input_notes_title")),
		"",
		statusBox,
		"",
		lipgloss.NewStyle().Foreground(colText).Render(m.T("input_notes_sub")),
		"",
		inputBox,
		"",
		lipgloss.NewStyle().Foreground(colMuted).Render(m.T("input_notes_foot")),
	}

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(lines, "\n"))
}

func (m *Model) renderInputModal(title, subtitle, value, footer string) string {
	modalW := 76
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	wrappedInput := wrapTextBlock(value+"█", innerW-4)
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colCyan).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(wrappedInput, "\n"))

	lines := []string{
		lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(title),
		"",
		lipgloss.NewStyle().Foreground(colText).Render(subtitle),
		"",
		inputBox,
		"",
		lipgloss.NewStyle().Foreground(colMuted).Render(footer),
	}

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(lines, "\n"))
}
