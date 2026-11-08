package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"telepilot/internal/domain/skills"
)

func (m *Model) renderModalOverlay() string {
	var modalBox string

	switch m.activeModal {
	case modalLoginForm:
		modalBox = m.renderLoginFormModal()
	case modalLoginQR:
		modalBox = m.renderLoginQRModal()
	case modalPrompt:
		modalBox = m.renderSingleShotModal()
	case modalNewSkill:
		modalBox = m.renderInputModal(
			m.T("input_new_skill_title"),
			m.T("input_new_skill_sub"),
			m.inputBuffer,
			m.T("input_new_skill_foot"),
		)
	case modalAIConfig:
		modalBox = m.renderAIConfigModal()
	case modalGlobalConfig:
		modalBox = m.renderGlobalConfigModal()
	case modalHelp:
		modalBox = m.renderHelpModal()
	case modalSkillPreview:
		modalBox = m.renderSkillPreviewModal()
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalBox)
}

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

func (m *Model) renderAIConfigModal() string {
	modalW := 84
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	labels := [2]string{
		m.T("ai_modal_path"),
		m.T("ai_modal_model"),
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
	rows = append(rows, lipgloss.NewStyle().Foreground(bannerCol).Bold(true).Render(truncateSingleLine(banner, innerW)))
	rows = append(rows, "")

	fieldW := innerW - 18
	if fieldW < 20 {
		fieldW = 20
	}

	for i := 0; i < 2; i++ {
		val := m.aiFormFields[i]
		if i == 0 && strings.TrimSpace(val) == "" && m.aiFormFocus != 0 {
			if m.cfg.IsRussian() {
				val = "Авто-поиск agy в ~/.local/bin/agy или PATH"
			} else {
				val = "Auto-detect agy in ~/.local/bin/agy or PATH"
			}
		}
		if m.aiFormFocus == i {
			val += "█"
		}
		fBorder := colBorder
		if m.aiFormFocus == i {
			fBorder = colCyan
		}
		fieldBox := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(fBorder).
			Padding(0, 1).
			Width(fieldW).
			Render(padOrTruncate(val, fieldW-2))

		lbl := lipgloss.NewStyle().Foreground(colBlue).Bold(true).Render("\n" + labels[i])
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, lbl, " ", fieldBox))
	}

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

func (m *Model) renderGlobalConfigModal() string {
	modalW := 88
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("cfg_modal_title")))
	rows = append(rows, "")

	banner := m.T("cfg_modal_banner")
	rows = append(rows, lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(truncateSingleLine(banner, innerW)))
	rows = append(rows, "")

	// Section 1: Language
	langHdrFg := colBlue
	if m.cfgSection == 0 {
		langHdrFg = colCyan
	}
	langLabel := lipgloss.NewStyle().Foreground(langHdrFg).Bold(true).Render(m.T("cfg_modal_lang"))
	rows = append(rows, langLabel)

	optW := innerW - 4
	if optW < 30 {
		optW = 30
	}

	enPrefix := "  "
	ruPrefix := "  "
	enStyle := lipgloss.NewStyle().Width(optW).Padding(0, 1)
	ruStyle := lipgloss.NewStyle().Width(optW).Padding(0, 1)

	if m.cfgLangIndex == 0 {
		enPrefix = "▸ "
		enStyle = enStyle.Background(colHighlight).Foreground(colGreen).Bold(true)
		ruStyle = ruStyle.Background(colPanelBg).Foreground(colMuted)
	} else {
		ruPrefix = "▸ "
		enStyle = enStyle.Background(colPanelBg).Foreground(colMuted)
		ruStyle = ruStyle.Background(colHighlight).Foreground(colGreen).Bold(true)
	}

	btnEn := enStyle.Render(enPrefix + "[1] En — English (Default)")
	btnRu := ruStyle.Render(ruPrefix + "[2] Ru — Русский (Russian)")
	rows = append(rows, btnEn, btnRu, "")

	// Section 2: Favourite / Pinned Chats
	favHdrFg := colBlue
	if m.cfgSection == 1 {
		favHdrFg = colCyan
	}
	favHdr := lipgloss.NewStyle().Foreground(favHdrFg).Bold(true).Render(m.T("cfg_modal_favs_title", len(m.favoriteChats)))
	rows = append(rows, favHdr)

	if len(m.favoriteChats) == 0 {
		emptyRow := lipgloss.NewStyle().Foreground(colMuted).Italic(true).Render(m.T("cfg_modal_favs_empty"))
		rows = append(rows, emptyRow, "")
	} else {
		chatExists := make(map[int64]bool, len(m.chats))
		for _, c := range m.chats {
			chatExists[c.ChatID] = true
		}

		maxVisibleFavs := 4
		startFav := 0
		if m.cfgFavCursor >= maxVisibleFavs {
			startFav = m.cfgFavCursor - maxVisibleFavs + 1
		}
		endFav := minInt(len(m.favoriteChats), startFav+maxVisibleFavs)

		var favRows []string
		for i := startFav; i < endFav; i++ {
			fav := m.favoriteChats[i]
			isCursor := m.cfgSection == 1 && i == m.cfgFavCursor
			prefix := "  "
			if isCursor {
				prefix = "▸ "
			}

			exists := chatExists[fav.ChatID]
			statusStr := ""
			if !exists {
				statusStr = " " + m.T("cfg_modal_favs_deleted")
			}

			actionHint := ""
			if isCursor {
				actionHint = "  [d/x: Remove]"
			}

			titleClean := sanitizeTerminalText(fav.Title)
			rawItem := fmt.Sprintf("%s📌 %s (ID: %d)%s%s", prefix, titleClean, fav.ChatID, statusStr, actionHint)

			itemStyle := lipgloss.NewStyle().Width(optW).Padding(0, 1)
			if isCursor {
				if !exists {
					itemStyle = itemStyle.Background(colHighlight).Foreground(colRed).Bold(true)
				} else {
					itemStyle = itemStyle.Background(colHighlight).Foreground(colCyan).Bold(true)
				}
			} else {
				if !exists {
					itemStyle = itemStyle.Background(colPanelBg).Foreground(colRed)
				} else {
					itemStyle = itemStyle.Background(colPanelBg).Foreground(colText)
				}
			}
			favRows = append(favRows, itemStyle.Render(padOrTruncate(rawItem, optW)))
		}
		if len(m.favoriteChats) > maxVisibleFavs {
			scrollInfo := lipgloss.NewStyle().Foreground(colMuted).Render(
				fmt.Sprintf("  [%d/%d favourite chats — use j/k to navigate]", m.cfgFavCursor+1, len(m.favoriteChats)),
			)
			favRows = append(favRows, scrollInfo)
		}
		rows = append(rows, strings.Join(favRows, "\n"), "")
	}

	infoLines := []string{
		m.T("cfg_modal_info_model", m.cfg.ActiveModelLabel()),
		m.T("cfg_modal_info_skill", m.activeSkill),
		m.T("cfg_modal_info_skills", m.cfg.SkillsDir()),
		m.T("cfg_modal_info_export", m.cfg.DataDir),
		m.T("cfg_modal_info_server", fmt.Sprintf("%s:%d (DC %d)", m.cfg.TelegramServerIP, m.cfg.TelegramServerPort, m.cfg.TelegramDCID)),
	}
	var wrappedInfo []string
	for _, l := range infoLines {
		wrappedInfo = append(wrappedInfo, lipgloss.NewStyle().Foreground(colText).Render(padOrTruncate(l, innerW-6)))
	}
	infoBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(wrappedInfo, "\n"))

	rows = append(rows, infoBox, "")

	btnW := (innerW - 5) / 2
	if btnW < 24 {
		btnW = 24
	}
	btnToggle := lipgloss.NewStyle().Background(colBlue).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("cfg_modal_btn_toggle"))
	btnSave := lipgloss.NewStyle().Background(colGreen).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("cfg_modal_btn_save"))
	btnEsc := lipgloss.NewStyle().Background(colBorder).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render(m.T("cfg_modal_btn_cancel"))

	btnRow1 := lipgloss.JoinHorizontal(lipgloss.Center, btnToggle, "   ", btnSave)
	btnRow2 := lipgloss.JoinHorizontal(lipgloss.Center, btnEsc)
	rows = append(rows, btnRow1, "", btnRow2, "")

	hintText := m.T("cfg_modal_hint")
	if m.cfgSection == 1 {
		hintText = m.T("cfg_modal_favs_hint")
	}
	hint := lipgloss.NewStyle().Foreground(colMuted).Align(lipgloss.Center).Width(innerW).Render(hintText)
	rows = append(rows, hint)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(rows, "\n"))
}

func (m *Model) renderHelpModal() string {
	modalW := 80
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	if modalW < 50 {
		modalW = 50
	}
	innerW := modalW - 6

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("help_modal_title")), "")

	type shortcutItem struct {
		key  string
		desc string
	}

	var items []shortcutItem
	if m.cfg.IsRussian() {
		items = []shortcutItem{
			{"1 - 4", "Вкладки (1:Чат, 2:ИИ и навыки, 3:Результат ИИ, 4:Логи)"},
			{"Tab / h / l", "Переключение фокуса между списком чатов и главным окном"},
			{"j / k или Вверх/Вниз", "Перемещение курсора / прокрутка активного окна"},
			{"g / G, Ctrl+d/Ctrl+u", "Быстрая навигация по списку сообщений или результату ИИ"},
			{"Enter / Space (Чаты)", "Открыть выбранный чат (только 1 активный чат)"},
			{"p / f (Чаты)", "Закрепить/открепить чат наверху списка"},
			{"\\ или /", "Быстрый Vim-поиск по окну чатов (Enter выбрать, Esc сброс)"},
			{"i / Enter (Вкл. 1)", "Написать сообщение в чат (Enter отправить, Esc отмена)"},
			{"v (Вкл. 1 / 2 / 3)", "Прослушать голос / открыть медиа (QuickLook) / навык / отчет"},
			{"Enter (Вкл. 2)", "Включить выбранный ИИ-навык для текущего чата ([AI])"},
			{"x (Вкл. 2)", "Отключить ИИ-инструкции для текущего чата"},
			{"m (Вкл. 1 / 2 / 3)", "Разовая команда ИИ для текущего чата (запуск / стоп)"},
			{"e / O / o (Вкл. 2)", "Редакт. в $EDITOR (e), macOS редакторе (O), открыть в Finder (o)"},
			{"n / I (Вкл. 2)", "Создать новый навык (n) или импортировать через Finder (I)"},
			{"r (Вкл. 1 / 2 / 3)", "Обновить чат (Вкл. 1) или запустить ИИ-навык (Вкл. 2/3)"},
			{"o (Вкл. 1 / 2 / 3)", "Показать кэш-медиа (Вкл. 1), навык (Вкл. 2) или отчет (Вкл. 3) в Finder"},
			{"c / C / L", "ИИ-движок (c), Настройки языка (C), Вход по QR (L)"},
			{"q / Esc", "Выход или закрытие модального окна"},
		}
	} else {
		items = []shortcutItem{
			{"1 - 4", "Switch tabs (1:Live Chat, 2:AI & Skills, 3:AI Output, 4:Trace)"},
			{"Tab / h / l", "Switch focus between Chats sidebar and Main window"},
			{"j / k or Up/Dn", "Move cursor down / up or scroll active view"},
			{"g / G, Ctrl+d/Ctrl+u", "Jump to top/bottom or half-page scroll"},
			{"Enter / Space (Chat)", "Select & open chat in Tab 1 (single-chat selection)"},
			{"p / f (Chat)", "Pin / unpin favourite chat on top of list"},
			{"\\ or /", "Vim-style search in Chats window (Enter open, Esc clear)"},
			{"i / Enter (Tab 1)", "Compose message in live chat (Enter to send, Esc to cancel)"},
			{"v (Tab 1 / 2 / 3)", "Play voice / QuickLook media / Preview skill / QuickLook output"},
			{"Enter (Tab 2)", "Enable highlighted AI skill for active chat ([AI])"},
			{"x (Tab 2)", "Disable/clear AI instructions for active chat"},
			{"m (Tab 1 / 2 / 3)", "Run or stop single-shot AI instruction for active chat"},
			{"e / O / o (Tab 2)", "Edit skill in $EDITOR (e), macOS editor (O), or Finder (o)"},
			{"n / I (Tab 2)", "Create new skill in terminal (n) or import via Finder (I)"},
			{"r (Tab 1 / 2 / 3)", "Refresh live chat (Tab 1) or Run active AI skill (Tab 2/3)"},
			{"o (Tab 1 / 2 / 3)", "Reveal cached media (Tab 1), skill (Tab 2), or output (Tab 3) in Finder"},
			{"c / C / L", "AI Engine config (c), Global settings (C), QR Login (L)"},
			{"q / Esc", "Quit or close modal"},
		}
	}

	keyColW := 0
	for _, it := range items {
		w := lipgloss.Width(it.key)
		if w > keyColW {
			keyColW = w
		}
	}

	prefixW := 2 + keyColW + 3
	descW := innerW - prefixW
	if descW < 20 {
		descW = 20
	}
	indent := strings.Repeat(" ", prefixW)

	for _, it := range items {
		prefix := "  " + padRightVisual(it.key, keyColW) + " : "
		wrapped := wrapTextBlock(it.desc, descW)
		if len(wrapped) == 0 {
			lines = append(lines, prefix)
			continue
		}
		lines = append(lines, prefix+wrapped[0])
		for _, wl := range wrapped[1:] {
			lines = append(lines, indent+wl)
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(lines, "\n"))
}
