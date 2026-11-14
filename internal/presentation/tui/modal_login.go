/*
File: internal/presentation/tui/modal_login.go
Description: Renderers for the Telegram API credentials form modal and QR login modal.
*/

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderLoginFormModal() string {
	modalW := 86
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	labels := [4]string{
		"API ID:        ",
		"API Hash:      ",
		"MTProto Server:",
		"AGY Model:     ",
	}

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render("─Telegram MTProto Connection Test & Phone QR Login─"))
	rows = append(rows, "")

	statusBanner := "Verify MTProto connection below, then press 'g' to generate Phone QR code."
	bannerCol := colYellow
	if m.connTesting {
		statusBanner = "⟳ Running 3-step MTProto connection check..."
		bannerCol = colCyan
	} else if m.connPassed {
		statusBanner = "[OK] Connection verified! Press 'g' to generate your Phone QR Code."
		bannerCol = colGreen
	}
	rows = append(rows, lipgloss.NewStyle().Foreground(bannerCol).Bold(true).Render(truncateSingleLine(statusBanner, innerW)))
	rows = append(rows, "")

	fieldW := innerW - 18
	if fieldW < 20 {
		fieldW = 20
	}

	for i := 0; i < 4; i++ {
		val := m.formFields[i]
		if i == 3 && strings.TrimSpace(val) == "" && m.formFocus != 3 {
			val = "auto (Antigravity CLI model)"
		}

		displayVal := val
		if i == 1 && len(val) > 0 {
			if m.formFocus == 1 {
				displayVal = strings.Repeat("•", len(val))
			} else {
				if len(val) > 8 {
					displayVal = val[:4] + strings.Repeat("•", len(val)-8) + val[len(val)-4:]
				} else {
					displayVal = strings.Repeat("•", len(val))
				}
			}
		}

		if m.formFocus == i {
			displayVal += "█"
		}
		fBorder := colBorder
		if m.formFocus == i {
			fBorder = colCyan
		}
		fieldBox := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(fBorder).
			Padding(0, 1).
			Width(fieldW).
			Render(padOrTruncate(displayVal, fieldW-2))

		lbl := lipgloss.NewStyle().Foreground(colBlue).Bold(true).Render("\n" + labels[i])
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, lbl, " ", fieldBox))
	}

	var diagLines []string
	for _, cl := range m.connLines {
		col := colText
		if strings.Contains(cl, "[OK]") {
			col = colGreen
		} else if strings.Contains(cl, "[FAIL]") || strings.Contains(cl, "[!]") {
			col = colRed
		}
		for _, wl := range wrapTextBlock(cl, innerW-6) {
			diagLines = append(diagLines, lipgloss.NewStyle().Foreground(col).Render(padOrTruncate(wl, innerW-6)))
		}
	}
	whileLen := 4
	for len(diagLines) < whileLen {
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
	btnTest := lipgloss.NewStyle().Background(colBlue).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render("Test Conn (t / Enter)")
	btnGenQR := lipgloss.NewStyle().Background(colGreen).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render("Generate QR (g)")
	btnSwapIP := lipgloss.NewStyle().Background(colHighlight).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render("Prod/Test IP (F2)")
	btnEsc := lipgloss.NewStyle().Background(colBorder).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render("Cancel / Close (Esc)")

	btnRow1 := lipgloss.JoinHorizontal(lipgloss.Center, btnTest, "   ", btnGenQR)
	btnRow2 := lipgloss.JoinHorizontal(lipgloss.Center, btnSwapIP, "   ", btnEsc)
	rows = append(rows, btnRow1, "", btnRow2)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(rows, "\n"))
}

func (m *Model) renderLoginQRModal() string {
	modalW := 82
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render("─Telegram Phone QR Code Authorization─"))
	rows = append(rows, "")

	stCol := colYellow
	if strings.HasPrefix(m.qrStatus, "[!]") {
		stCol = colRed
	} else if strings.Contains(m.qrStatus, "Scan QR") {
		stCol = colGreen
	}
	rows = append(rows, lipgloss.NewStyle().Foreground(stCol).Bold(true).Render(truncateSingleLine(m.qrStatus, innerW)))
	rows = append(rows, "")

	var qrContent string
	if strings.TrimSpace(m.qrArt) != "" {
		qrContent = lipgloss.PlaceHorizontal(innerW-4, lipgloss.Center, m.qrArt)
	} else {
		qrContent = lipgloss.Place(innerW-4, 14, lipgloss.Center, lipgloss.Center,
			"Connecting to Telegram MTProto DC 2 server & exporting QR login token...")
	}

	qrBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Width(innerW - 2).
		Render(qrContent)

	rows = append(rows, qrBox, "")

	btnW := (innerW - 5) / 2
	if btnW < 24 {
		btnW = 24
	}
	btnRefresh := lipgloss.NewStyle().Background(colBlue).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render("Refresh QR (r)")
	btnQL := lipgloss.NewStyle().Background(colHighlight).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render("QuickLook PNG (v)")
	btnEdit := lipgloss.NewStyle().Background(colYellow).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render("Edit Keys / Conn Test (c)")
	btnEsc := lipgloss.NewStyle().Background(colBorder).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render("Cancel / Close (Esc)")

	btnRow1 := lipgloss.JoinHorizontal(lipgloss.Center, btnRefresh, "   ", btnQL)
	btnRow2 := lipgloss.JoinHorizontal(lipgloss.Center, btnEdit, "   ", btnEsc)
	rows = append(rows, btnRow1, "", btnRow2)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(rows, "\n"))
}
