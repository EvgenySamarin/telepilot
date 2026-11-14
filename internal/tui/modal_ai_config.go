/*
File: internal/tui/modal_ai_config.go
Description: Renderer for the AI Engine configuration modal for editing the AGY CLI
binary path and model preset.
*/

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderAIConfigModal() string {
	modalW := 84
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	labels := [3]string{
		m.T("ai_modal_path"),
		m.T("ai_modal_model"),
		m.T("ai_modal_timeout"),
	}

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("ai_modal_title")))
	rows = append(rows, "")

	var banner string
	bannerCol := colYellow
	if m.aiTesting {
		if m.cfg.IsRussian() {
			banner = "⟳ Проверка подключения к Antigravity CLI (agy -p)..."
		} else {
			banner = "⟳ Testing connection to Antigravity CLI (agy -p)..."
		}
		bannerCol = colCyan
	} else if m.aiPassed {
		if m.cfg.IsRussian() {
			banner = "[OK] Antigravity CLI (agy) проверен и сохранен в .env! Нажмите Esc и 'r' для запуска ИИ."
		} else {
			banner = "[OK] Antigravity CLI (agy) verified and saved to .env! Press Esc and 'r' to re-run AI."
		}
		bannerCol = colGreen
	} else {
		if m.cfg.IsRussian() {
			banner = "Настройка пути к бинарному файлу и модели Google Antigravity CLI (agy)."
		} else {
			banner = "Configure Google Antigravity CLI (agy) binary path & model for voice, image & chat summaries."
		}
	}
	for _, bl := range wrapTextBlock(banner, innerW-2) {
		rows = append(rows, lipgloss.NewStyle().Foreground(bannerCol).Bold(true).Render(bl))
	}
	rows = append(rows, "")

	fieldW := innerW - 18
	if fieldW < 20 {
		fieldW = 20
	}

	for i := 0; i < 3; i++ {
		val := m.aiFormFields[i]
		isDisabled := i == 2 && m.aiNoTimeout
		if i == 0 && strings.TrimSpace(val) == "" && m.aiFormFocus != 0 {
			if m.cfg.IsRussian() {
				val = "Авто-поиск agy в ~/.local/bin/agy или PATH"
			} else {
				val = "Auto-detect agy in ~/.local/bin/agy or PATH"
			}
		} else if isDisabled {
			if m.cfg.IsRussian() {
				val = "— (отключен)"
			} else {
				val = "— (disabled)"
			}
		} else if i == 2 && m.aiFormFocus != 2 && strings.TrimSpace(val) == "" {
			val = "90"
		}
		if m.aiFormFocus == i && !isDisabled {
			val += "█"
		}
		fBorder := colBorder
		if m.aiFormFocus == i && !isDisabled {
			fBorder = colCyan
		}
		renderedVal := padOrTruncate(val, fieldW-2)
		if isDisabled {
			renderedVal = lipgloss.NewStyle().Foreground(colMuted).Render(renderedVal)
		}
		fieldBox := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(fBorder).
			Padding(0, 1).
			Width(fieldW).
			Render(renderedVal)

		lbl := lipgloss.NewStyle().Foreground(colBlue).Bold(true).Render("\n" + labels[i])
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, lbl, " ", fieldBox))
	}

	// No Timeout ON/OFF toggle strictly under the Timeout field
	noTimeoutOn := m.aiNoTimeout
	var toggleBadge, toggleHint string
	if noTimeoutOn {
		toggleBadge = lipgloss.NewStyle().Background(colGreen).Foreground(colBg).Bold(true).Render(" ON ")
		if m.cfg.IsRussian() {
			toggleHint = " Ждать ответа ИИ без таймаута (F3 / Space)"
		} else {
			toggleHint = " Wait for AI without timeout (F3 / Space)"
		}
	} else {
		toggleBadge = lipgloss.NewStyle().Background(colBorder).Foreground(colText).Bold(true).Render(" OFF ")
		if m.cfg.IsRussian() {
			toggleHint = " Использовать таймаут выше (F3 / Space)"
		} else {
			toggleHint = " Use timeout above (F3 / Space)"
		}
	}
	toggleBorder := colBorder
	if m.aiFormFocus == 3 {
		toggleBorder = colCyan
	}
	toggleBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(toggleBorder).
		Padding(0, 1).
		Width(fieldW).
		Render(toggleBadge + lipgloss.NewStyle().Foreground(colMuted).Render(padOrTruncate(toggleHint, fieldW-8)))
	toggleLbl := lipgloss.NewStyle().Foreground(colBlue).Bold(true).Render("\n" + m.T("ai_modal_no_timeout"))
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, toggleLbl, " ", toggleBox))

	diagCol := colText
	if strings.Contains(m.aiStatusLine, "[OK]") {
		diagCol = colGreen
	} else if strings.Contains(m.aiStatusLine, "[FAIL]") || strings.Contains(m.aiStatusLine, "[!]") {
		diagCol = colRed
	}
	var diagLines []string
	for _, wl := range wrapTextBlock(m.aiStatusLine, innerW-6) {
		diagLines = append(diagLines, lipgloss.NewStyle().Foreground(diagCol).Render(padOrTruncate(wl, innerW-6)))
	}
	for len(diagLines) < 2 {
		diagLines = append(diagLines, strings.Repeat(" ", innerW-6))
	}

	diagBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(diagLines, "\n"))

	rows = append(rows, "", diagBox, "")

	btnW := (innerW - 5) / 2
	if btnW < 24 {
		btnW = 24
	}
	btnTest := lipgloss.NewStyle().Background(colGreen).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("ai_modal_btn_test"))
	btnCycle := lipgloss.NewStyle().Background(colBlue).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("ai_modal_btn_cycle"))
	btnSaveClose := lipgloss.NewStyle().Background(colHighlight).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render(m.T("ai_modal_btn_save"))
	btnEsc := lipgloss.NewStyle().Background(colBorder).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render(m.T("ai_modal_btn_cancel"))

	btnRow1 := lipgloss.JoinHorizontal(lipgloss.Center, btnTest, "   ", btnCycle)
	btnRow2 := lipgloss.JoinHorizontal(lipgloss.Center, btnSaveClose, "   ", btnEsc)
	rows = append(rows, btnRow1, "", btnRow2)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(rows, "\n"))
}
