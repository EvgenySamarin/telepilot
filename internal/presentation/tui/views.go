package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"telepilot/internal/domain/config"
	"telepilot/internal/domain/skills"
	"telepilot/internal/domain/storage"
)

// View renders the entire responsive terminal layout.
func (m *Model) View() string {
	if m.width < 60 || m.height < 18 {
		return "Terminal window too small — please resize to at least 80x24."
	}

	if m.activeModal != modalNone {
		return clampViewToTerminal(m.renderModalOverlay(), m.width, m.height)
	}

	topBar := m.renderTopBar()
	bottomBar := m.renderBottomBar()

	topH := lipgloss.Height(topBar)
	bottomH := lipgloss.Height(bottomBar)
	bodyHeight := m.height - topH - bottomH
	if bodyHeight < 6 {
		bodyHeight = 6
	}

	sidebarWidth := 38
	if m.width >= 150 {
		sidebarWidth = 42
	} else if m.width < 100 {
		sidebarWidth = 30
	}
	if sidebarWidth > m.width-20 {
		sidebarWidth = maxInt(12, m.width-20)
	}
	mainWidth := m.width - sidebarWidth - 1
	if mainWidth < 10 {
		mainWidth = 10
	}

	sidebar := m.renderSidebar(sidebarWidth, bodyHeight)
	mainPane := m.renderMainPanel(mainWidth, bodyHeight)

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", mainPane)
	rawView := lipgloss.JoinVertical(lipgloss.Left, topBar, body, bottomBar)
	return clampViewToTerminal(rawView, m.width, m.height)
}

func (m *Model) renderTopBar() string {
	icon := "◉"
	statusColor := colGreen
	if m.statusBusy {
		icon = "⟳"
		statusColor = colYellow
	}
	chatTitle := "None"
	aiBadge := ""
	if ch := m.activeChat(); ch != nil {
		chatTitle = sanitizeTerminalText(ch.Title)
		if cfg, ok := m.activeChatAIConfig(); ok {
			aiBadge = " " + lipgloss.NewStyle().Foreground(colPurple).Bold(true).Render("[AI:"+cfg.Skill+"]")
		}
	}

	left := fmt.Sprintf(" %s │ %s%s",
		lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(icon+" "+m.statusText),
		m.T("active_chat_hdr", lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(chatTitle)),
		aiBadge,
	)

	elapsedSec := int(time.Since(m.startTime).Seconds())
	right := fmt.Sprintf("%s   %ds ",
		lipgloss.NewStyle().Foreground(colPurple).Render(m.cfg.ActiveModelLabel()),
		elapsedSec,
	)

	style := lipgloss.NewStyle().
		Width(m.width).
		Background(colPanelBg).
		Foreground(colText)

	if lipgloss.Width(left)+lipgloss.Width(right)+1 <= m.width {
		gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
		return style.Render(left + strings.Repeat(" ", gap) + right)
	}

	line1 := padOrTruncate(left, m.width)
	line2 := padOrTruncate(" "+strings.TrimSpace(right), m.width)
	return style.Render(line1 + "\n" + line2)
}

func (m *Model) renderBottomBar() string {
	type keyPair struct {
		k string
		v string
	}

	// Context-aware hotkeys tailored to the active tab + common global shortcuts
	var keys []keyPair
	if m.chatSearching {
		keys = []keyPair{
			{"Enter", m.T("key_select_chat")},
			{"Up/Down", m.T("key_vim_nav")},
			{"Ctrl+U", "Clear"},
			{"Esc", m.T("key_cancel")},
		}
	} else if m.activeTab == 1 && m.chatComposing {
		keys = []keyPair{
			{"Enter", m.T("key_send")},
			{"Esc", m.T("key_cancel")},
			{"Ctrl+U", "Clear"},
		}
	} else {
		keys = []keyPair{
			{"1-4", m.T("key_views")},
			{"Tab/h/l", m.T("key_focus")},
			{"j/k", m.T("key_vim_nav")},
			{"\\", m.T("key_search_chats")},
		}
		if m.focus == focusChats {
			keys = append(keys,
				keyPair{"Enter/Space", m.T("key_select_chat")},
				keyPair{"p/f", m.T("key_pin_chat")},
				keyPair{"H", m.T("key_hide_chat")},
			)
		}
		switch m.activeTab {
		case 1:
			keys = append(keys,
				keyPair{"i/Enter", m.T("key_compose")},
				keyPair{"m", m.T("key_notes")},
				keyPair{"a", m.T("key_ai_reply")},
				keyPair{"v", m.T("key_quicklook")},
				keyPair{"o", m.T("key_finder")},
				keyPair{"r", m.T("key_refresh")},
			)
		case 2:
			keys = append(keys,
				keyPair{"Enter", m.T("key_activate_skill")},
				keyPair{"a", m.T("key_auto_reply")},
				keyPair{"s", m.T("key_send_mode")},
				keyPair{"A", m.T("key_ai_reply")},
				keyPair{"x", m.T("key_disable_ai")},
				keyPair{"m", m.T("key_notes")},
				keyPair{"v", m.T("key_preview")},
				keyPair{"e", m.T("key_edit_skill")},
				keyPair{"O", m.T("key_mac_edit")},
				keyPair{"o", m.T("key_finder")},
				keyPair{"n", m.T("key_new_skill")},
				keyPair{"I", m.T("key_import_skill")},
				keyPair{"r", m.T("key_run_ai")},
			)
		case 3:
			keys = append(keys,
				keyPair{"r", m.T("key_run_ai")},
				keyPair{"m", m.T("key_notes")},
				keyPair{"v", m.T("key_quicklook")},
				keyPair{"o", m.T("key_finder")},
			)
		}
		keys = append(keys,
			keyPair{"c", m.T("key_ai_config")},
			keyPair{"C", m.T("key_global_config")},
			keyPair{"L", m.T("key_login")},
			keyPair{"?", m.T("key_help")},
			keyPair{"q", m.T("key_quit")},
		)
	}

	innerW := m.width - 2
	if innerW < 20 {
		innerW = 20
	}

	var items []string
	for _, kv := range keys {
		kStr := lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(kv.k)
		vStr := lipgloss.NewStyle().Foreground(colMuted).Render(":" + kv.v)
		items = append(items, kStr+vStr)
	}

	var lines []string
	curLine := " "
	for _, item := range items {
		sep := "  "
		if curLine == " " {
			sep = ""
		}
		candidate := curLine + sep + item
		if lipgloss.Width(candidate) <= innerW || curLine == " " {
			curLine = candidate
		} else {
			lines = append(lines, padOrTruncate(curLine, innerW))
			curLine = " " + item
		}
	}
	if strings.TrimSpace(curLine) != "" {
		lines = append(lines, padOrTruncate(curLine, innerW))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		Width(innerW).
		Render(strings.Join(lines, "\n"))
}

// renderSidebar renders the full-height single-select Chats window with colored highlights for chats with active AI.
func (m *Model) renderSidebar(width, totalHeight int) string {
	chatsH := totalHeight
	if chatsH < 6 {
		chatsH = 6
	}

	innerW := width - 2
	if innerW < 12 {
		innerW = 12
	}

	chatsBorderCol := colBorder
	if m.focus == focusChats {
		chatsBorderCol = colBorderFoc
	}

	matches := m.filteredChatIndices()
	cursorPos := m.filteredChatCursorPos(matches)
	showSearchBar := m.chatSearching || m.chatSearchQuery != ""
	hasHidden := len(m.hiddenChats) > 0

	maxChatRows := chatsH - 3
	if showSearchBar {
		maxChatRows = chatsH - 4
	}
	if hasHidden {
		maxChatRows--
	}
	if maxChatRows < 1 {
		maxChatRows = 1
	}

	if cursorPos < m.chatScrollOffset {
		m.chatScrollOffset = cursorPos
	}
	if cursorPos >= m.chatScrollOffset+maxChatRows {
		m.chatScrollOffset = cursorPos - maxChatRows + 1
	}
	if m.chatScrollOffset < 0 {
		m.chatScrollOffset = 0
	}

	displayIdx := 0
	if len(matches) > 0 {
		displayIdx = cursorPos + 1
	}
	chatHdr := m.T("sidebar_chats_hdr", displayIdx, len(matches))
	var chatLines []string
	chatLines = append(chatLines, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(chatHdr, innerW)))

	if showSearchBar {
		var searchLine string
		if m.chatSearching {
			rawSearch := m.T("sidebar_search_active", sanitizeTerminalText(m.chatSearchQuery), displayIdx, len(matches))
			searchLine = lipgloss.NewStyle().Foreground(colYellow).Bold(true).Background(colHighlight).Width(innerW).Render(padOrTruncate(rawSearch, innerW))
		} else {
			rawSearch := m.T("sidebar_search_filter", sanitizeTerminalText(m.chatSearchQuery), displayIdx, len(matches))
			searchLine = lipgloss.NewStyle().Foreground(colYellow).Render(padOrTruncate(rawSearch, innerW))
		}
		chatLines = append(chatLines, searchLine)
	}

	maxChatLines := chatsH - 2
	if hasHidden {
		maxChatLines--
	}

	if len(matches) == 0 {
		emptyRow := lipgloss.NewStyle().Foreground(colMuted).Render(padOrTruncate(m.T("sidebar_search_no_match"), innerW))
		chatLines = append(chatLines, emptyRow)
	} else {
		for pos := m.chatScrollOffset; pos < len(matches) && len(chatLines) < maxChatLines; pos++ {
			chatIdx := matches[pos]
			c := m.chats[chatIdx]
			isSelected := c.ChatID == m.selectedChatID
			isFocusedRow := m.focus == focusChats && m.chatCursor == chatIdx
			hasAI := m.isChatAIEnabled(c.ChatID)
			isPinned := m.isChatPinned(c.ChatID)

			selMark := "( )"
			if isSelected {
				selMark = "(●)"
			}
			cursor := "  "
			if isFocusedRow {
				cursor = "▸ "
			}
			pinTag := ""
			if isPinned {
				pinTag = "📌 "
			}
			aiTag := ""
			if hasAI {
				aiTag = "[AI] "
			}
			cleanTitle := sanitizeTerminalText(c.Title)
			rawRow := fmt.Sprintf("%s%s %s%s%s (%d)", cursor, selMark, pinTag, aiTag, cleanTitle, c.UnreadCount)

			formatted := renderRunningLine(rawRow, innerW, isFocusedRow, m.marqueeTick)
			if isFocusedRow {
				fg := colCyan
				if hasAI {
					fg = colPurple
				}
				formatted = lipgloss.NewStyle().Foreground(fg).Bold(true).Background(colHighlight).Width(innerW).Render(formatted)
			} else {
				switch {
				case hasAI && isSelected:
					formatted = lipgloss.NewStyle().Foreground(colPurple).Bold(true).Width(innerW).Render(formatted)
				case hasAI:
					formatted = lipgloss.NewStyle().Foreground(colPurple).Width(innerW).Render(formatted)
				case isSelected:
					formatted = lipgloss.NewStyle().Foreground(colGreen).Bold(true).Width(innerW).Render(formatted)
				default:
					formatted = lipgloss.NewStyle().Foreground(colMuted).Width(innerW).Render(formatted)
				}
			}
			chatLines = append(chatLines, formatted)
		}
	}

	if hasHidden {
		for len(chatLines) < chatsH-3 {
			chatLines = append(chatLines, strings.Repeat(" ", innerW))
		}
		noticeStr := m.T("sidebar_hidden_notice", len(m.hiddenChats))
		noticeLine := lipgloss.NewStyle().
			Foreground(colYellow).
			Width(innerW).
			Render(padOrTruncate(noticeStr, innerW))
		chatLines = append(chatLines, noticeLine)
	}

	chatLines = clampLinesToWidth(chatLines, innerW)
	if len(chatLines) > chatsH-2 {
		chatLines = chatLines[:chatsH-2]
	}
	for len(chatLines) < chatsH-2 {
		chatLines = append(chatLines, strings.Repeat(" ", innerW))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(chatsBorderCol).
		Width(innerW).
		Height(chatsH - 2).
		Render(strings.Join(chatLines, "\n"))
}

func (m *Model) renderMainPanel(width, height int) string {
	innerW := width - 2
	if innerW < 20 {
		innerW = 20
	}
	innerH := height - 2
	if innerH < 8 {
		innerH = 8
	}

	borderCol := colBorder
	if m.focus == focusMain {
		borderCol = colBorderFoc
	}

	tabs := []struct {
		id    int
		label string
	}{
		{1, m.T("tab_1")},
		{2, m.T("tab_2")},
		{3, m.T("tab_3")},
		{4, m.T("tab_4")},
	}
	var tabTokens []string
	tabTokens = append(tabTokens, lipgloss.NewStyle().Foreground(colText).Bold(true).Render(m.T("tab_details")))
	for _, t := range tabs {
		if m.activeTab == t.id {
			tabTokens = append(tabTokens, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render("["+t.label+"]"))
		} else {
			tabTokens = append(tabTokens, lipgloss.NewStyle().Foreground(colMuted).Render("["+t.label+"]"))
		}
	}

	var headerLines []string
	curHdr := ""
	for _, tok := range tabTokens {
		sep := " "
		if curHdr == "" {
			sep = ""
		}
		cand := curHdr + sep + tok
		if lipgloss.Width(cand) <= innerW || curHdr == "" {
			curHdr = cand
		} else {
			headerLines = append(headerLines, padOrTruncate(curHdr, innerW))
			curHdr = "  " + tok
		}
	}
	if strings.TrimSpace(curHdr) != "" {
		headerLines = append(headerLines, padOrTruncate(curHdr, innerW))
	}

	visibleRows := innerH - len(headerLines) - 1
	if visibleRows < 1 {
		visibleRows = 1
	}
	m.lastVisibleRows = visibleRows

	if m.activeTab == 1 {
		m.resizePending = false
		liveLines := clampLinesToWidth(m.renderLiveChatTab(innerW, visibleRows), innerW)
		if len(liveLines) > visibleRows {
			liveLines = liveLines[:visibleRows]
		}
		for len(liveLines) < visibleRows {
			liveLines = append(liveLines, strings.Repeat(" ", innerW))
		}
		allLines := append([]string{}, headerLines...)
		allLines = append(allLines, strings.Repeat(" ", innerW))
		allLines = append(allLines, liveLines...)
		allLines = clampLinesToWidth(allLines, innerW)
		if len(allLines) > innerH {
			allLines = allLines[:innerH]
		}
		for len(allLines) < innerH {
			allLines = append(allLines, strings.Repeat(" ", innerW))
		}
		return lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(borderCol).
			Width(innerW).
			Height(innerH).
			Render(strings.Join(allLines, "\n"))
	}

	var contentLines []string
	rec := m.activeSkillOutputRecord()

	switch m.activeTab {
	case 2:
		contentLines = m.renderRePromptTab(rec, innerW)
		if m.resizePending {
			m.resizePending = false
			if m.tab2BoxEntered && m.tab2BoxCursor == 3 {
				m.scrollTab2ToSkill()
			} else {
				m.scrollTab2ToBox(m.tab2BoxCursor)
			}
		}
	case 3:
		contentLines = m.renderSkillOutputTab(rec, innerW, visibleRows)
		m.resizePending = false
	case 4:
		contentLines = m.renderTraceTab(innerW)
		m.resizePending = false
	}
	contentLines = clampLinesToWidth(contentLines, innerW)

	maxScroll := maxInt(0, len(contentLines)-visibleRows)
	if m.mainScroll > maxScroll {
		m.mainScroll = maxScroll
	}

	sliced := contentLines
	if m.mainScroll < len(sliced) {
		sliced = sliced[m.mainScroll:]
	}
	if len(sliced) > visibleRows {
		sliced = sliced[:visibleRows]
	}
	for len(sliced) < visibleRows {
		sliced = append(sliced, strings.Repeat(" ", innerW))
	}

	allLines := append([]string{}, headerLines...)
	allLines = append(allLines, strings.Repeat(" ", innerW))
	allLines = append(allLines, sliced...)
	allLines = clampLinesToWidth(allLines, innerW)
	if len(allLines) > innerH {
		allLines = allLines[:innerH]
	}
	for len(allLines) < innerH {
		allLines = append(allLines, strings.Repeat(" ", innerW))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderCol).
		Width(innerW).
		Height(innerH).
		Render(strings.Join(allLines, "\n"))
}

// renderLiveChatTab renders Tab 1: real-time message stream + bottom inline compose box.
func (m *Model) renderLiveChatTab(innerW, visibleRows int) []string {
	ch := m.activeChat()
	chatTitle := "Telegram Chat"
	chatType := "group"
	if ch != nil {
		chatTitle = sanitizeTerminalText(ch.Title)
		chatType = ch.ChatType
	}

	hdrText := m.T("chat_hdr", chatTitle, chatType, len(m.liveMessages))
	if cfg, ok := m.activeChatAIConfig(); ok {
		modeTag := "Draft"
		if cfg.SendMode == "auto_send" {
			modeTag = "Auto-Send"
		}
		if cfg.AutoReply {
			hdrText += fmt.Sprintf("  │  [AI: %s • Auto-Reply • %s]", cfg.Skill, modeTag)
		} else {
			hdrText += fmt.Sprintf("  │  [AI: %s • Manual('a') • %s]", cfg.Skill, modeTag)
		}
	}
	var hdrLines []string
	for _, hl := range wrapTextBlock(hdrText, innerW) {
		hdrLines = append(hdrLines, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(hl, innerW)))
	}

	inputBoxLines := m.renderLiveInputBox(chatTitle, innerW)
	hdrHeight := len(hdrLines) + 1
	msgViewportH := visibleRows - hdrHeight - len(inputBoxLines)
	if msgViewportH < 1 {
		msgViewportH = 1
	}

	textW := innerW - 12
	if textW < 16 {
		textW = 16
	}

	var msgLines []string
	selStart := 0
	selEnd := 0

	if len(m.liveMessages) == 0 {
		msgLines = append(msgLines, "  "+m.T("chat_empty"))
	} else {
		if m.liveMsgCursor < 0 {
			m.liveMsgCursor = 0
		}
		if m.liveMsgCursor >= len(m.liveMessages) {
			m.liveMsgCursor = len(m.liveMessages) - 1
		}

		prevDate := ""
		prevSender := ""

		for idx, lm := range m.liveMessages {
			isSelected := idx == m.liveMsgCursor
			msgDate, msgTime := splitDateAndTime(lm.Timestamp)
			sender := sanitizeTerminalText(lm.Sender)
			if sender == "" {
				sender = "Unknown"
			}

			accentCol := colBlue
			if lm.IsOutgoing {
				accentCol = colGreen
			}

			// 1. Date divider when the calendar day changes
			dateChanged := msgDate != "" && msgDate != prevDate
			if dateChanged {
				if len(msgLines) > 0 {
					msgLines = append(msgLines, "")
				}
				msgLines = append(msgLines, renderDateSeparator(msgDate, innerW))
				prevDate = msgDate
				prevSender = ""
			}

			// 2. Grouped sender header (only shown when sender changes or after a date divider)
			if sender != prevSender {
				if len(msgLines) > 0 && !dateChanged {
					msgLines = append(msgLines, "")
				}
				senderLine := "   " + lipgloss.NewStyle().Foreground(accentCol).Bold(true).Render(truncateSingleLine(sender, innerW-4))
				msgLines = append(msgLines, senderLine)
				prevSender = sender
			}

			msgStart := len(msgLines)

			// 3. Wrap message text & media attachments with clean left time column + vertical accent bar
			var bodyLines []string
			if strings.TrimSpace(lm.Text) != "" {
				bodyLines = append(bodyLines, wrapTextBlock(lm.Text, textW)...)
			}
			var mediaLineIdx = -1
			if lm.MediaType != "" {
				mTag := strings.ToUpper(lm.MediaType)
				if mTag == "DOCUMENT" {
					mTag = "DOC"
				}
				hint := m.T("chat_media_hint")
				if lm.CachedPath != "" {
					hint = m.T("chat_media_cached", filepath.Base(lm.CachedPath))
				}
				sizeKB := lm.MediaSize / 1024
				if sizeKB == 0 && lm.MediaSize > 0 {
					sizeKB = 1
				}
				mediaLine := fmt.Sprintf("📎 [%s] %s (%d KB) %s", mTag, lm.MediaFileName, sizeKB, hint)
				mediaLineIdx = len(bodyLines)
				bodyLines = append(bodyLines, truncateSingleLine(mediaLine, textW))
			}
			if len(bodyLines) == 0 {
				bodyLines = append(bodyLines, "(empty message)")
			}

			for i, bl := range bodyLines {
				var prefix string
				timeStr := padOrTruncate(msgTime, 5)
				if i > 0 {
					timeStr = "     "
				}
				if isSelected {
					cursorGlyph := "▸ "
					if i > 0 {
						cursorGlyph = "  "
					}
					prefix = lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(cursorGlyph + timeStr + " ┃ ")
				} else {
					tStyled := lipgloss.NewStyle().Foreground(colMuted).Render("  " + timeStr + " ")
					barStyled := lipgloss.NewStyle().Foreground(accentCol).Render("│ ")
					prefix = tStyled + barStyled
				}

				textCol := colText
				boldText := false
				if i == mediaLineIdx {
					textCol = colYellow
				}
				if isSelected {
					textCol = colCyan
					boldText = true
				}
				styledBody := lipgloss.NewStyle().Foreground(textCol).Bold(boldText).Render(bl)
				msgLines = append(msgLines, prefix+styledBody)
			}

			if isSelected {
				selStart = maxInt(0, msgStart-1)
				selEnd = len(msgLines)
			}
		}
	}

	if selStart < m.liveScrollOffset {
		m.liveScrollOffset = selStart
	}
	if selEnd > m.liveScrollOffset+msgViewportH {
		m.liveScrollOffset = selEnd - msgViewportH
	}
	if m.liveScrollOffset < 0 {
		m.liveScrollOffset = 0
	}

	sliced := msgLines
	if m.liveScrollOffset < len(sliced) {
		sliced = sliced[m.liveScrollOffset:]
	}
	if len(sliced) > msgViewportH {
		sliced = sliced[:msgViewportH]
	}
	for len(sliced) < msgViewportH {
		sliced = append(sliced, strings.Repeat(" ", innerW))
	}

	var out []string
	out = append(out, hdrLines...)
	out = append(out, strings.Repeat(" ", innerW))
	out = append(out, sliced...)
	out = append(out, inputBoxLines...)
	return out
}

func (m *Model) renderLiveInputBox(chatTitle string, innerW int) []string {
	boxW := innerW - 4
	if boxW < 20 {
		boxW = 20
	}
	borderCol := colBorder
	var promptHdr string
	var inputContent string

	if m.chatComposing {
		borderCol = colGreen
		promptHdr = " " + lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(truncateSingleLine(m.T("chat_input_active", chatTitle), boxW))
		inputContent = lipgloss.NewStyle().Foreground(colText).Render(padOrTruncate(m.chatInputBuffer+"█", boxW-4))
	} else {
		idleText := padOrTruncate(m.T("chat_input_idle", chatTitle), boxW-4)
		inputContent = lipgloss.NewStyle().Foreground(colMuted).Render(idleText)
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderCol).
		Padding(0, 1).
		Width(boxW - 2).
		Render(inputContent)

	var out []string
	if promptHdr != "" {
		out = append(out, promptHdr)
	}
	for _, l := range strings.Split(box, "\n") {
		out = append(out, " "+l)
	}
	return out
}

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

func (m *Model) tab2VisibleRows() int {
	if m.lastVisibleRows > 0 {
		return m.lastVisibleRows
	}
	return maxInt(6, m.height-10)
}

func (m *Model) scrollTab2ToBox(boxIdx int) {
	if boxIdx < 0 || boxIdx >= len(m.tab2BoxBounds) {
		return
	}
	if boxIdx == 0 {
		m.mainScroll = 0
		return
	}
	bStart := m.tab2BoxBounds[boxIdx][0]
	bEnd := m.tab2BoxBounds[boxIdx][1]
	if bEnd <= bStart {
		return
	}
	visibleRows := m.tab2VisibleRows()
	if bStart < m.mainScroll {
		m.mainScroll = maxInt(0, bStart)
	} else if bEnd > m.mainScroll+visibleRows {
		target := bEnd - visibleRows
		if target > bStart {
			target = bStart
		}
		// Avoid slicing an earlier box in half at the top of the viewport if snapping to the next box header still shows boxIdx
		for k := 0; k < boxIdx; k++ {
			kStart := m.tab2BoxBounds[k][0]
			kEnd := m.tab2BoxBounds[k][1]
			if target > kStart && target <= kEnd+1 {
				nextBoxStart := m.tab2BoxBounds[k+1][0]
				if nextBoxStart <= bStart {
					target = nextBoxStart
				}
				break
			}
		}
		m.mainScroll = maxInt(0, target)
	}
}

func (m *Model) scrollTab2ToSkill() {
	bStart := m.tab2BoxBounds[3][0]
	if bStart <= 0 {
		return
	}
	visibleRows := m.tab2VisibleRows()
	offset := m.tab2SkillOffset
	if offset < 2 {
		offset = 2
	}
	skillLine := bStart + offset + m.skillCursor
	if m.skillCursor == 0 && bStart < m.mainScroll {
		m.mainScroll = maxInt(0, bStart)
		return
	}
	if skillLine < m.mainScroll+1 {
		m.mainScroll = maxInt(0, skillLine-offset)
	} else if skillLine >= m.mainScroll+visibleRows-2 {
		m.mainScroll = maxInt(0, skillLine-visibleRows+3)
	}
}

func renderNavBox(isSelectedBox, isEnteredBox bool, title string, body string, cardW int, defaultTitleCol lipgloss.Color, isRussian bool, indentLeading bool) []string {
	borderCol := colBorder
	titleCol := defaultTitleCol
	prefix := "  "
	badge := ""

	if isEnteredBox {
		borderCol = colGreen
		titleCol = colGreen
		prefix = "▼ "
		if isRussian {
			badge = "[АКТИВНО — j/k:Выбор, Enter:Применить, Esc:Назад]"
		} else {
			badge = "[ACTIVE — j/k:Select, Enter:Apply, Esc:Back]"
		}
	} else if isSelectedBox {
		borderCol = colBorderFoc
		titleCol = colCyan
		prefix = "▸ "
		if isRussian {
			badge = "[Enter: Открыть]"
		} else {
			badge = "[Enter: Open]"
		}
	}

	var displayTitle string
	if badge == "" {
		displayTitle = prefix + truncateSingleLine(title, maxInt(10, cardW-lipgloss.Width(prefix)))
	} else {
		candidate := prefix + title + " " + badge
		if lipgloss.Width(candidate) <= cardW {
			displayTitle = candidate
		} else {
			availTitleW := cardW - lipgloss.Width(prefix) - 1 - lipgloss.Width(badge)
			if availTitleW >= 20 {
				displayTitle = prefix + truncateSingleLine(title, availTitleW) + " " + badge
			} else {
				line1 := prefix + truncateSingleLine(title, maxInt(10, cardW-lipgloss.Width(prefix)))
				line2 := "  " + truncateSingleLine(badge, maxInt(10, cardW-2))
				displayTitle = line1 + "\n" + line2
			}
		}
	}

	innerTextW := cardW - 4
	if innerTextW < 10 {
		innerTextW = 10
	}

	wrappedLines := wrapTextBlock(body, innerTextW)
	var paddedLines []string
	for _, l := range wrappedLines {
		padded := padOrTruncate(l, innerTextW)
		paddedLines = append(paddedLines, renderColorizedCardLine(padded))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderCol).
		Padding(0, 1).
		Width(innerTextW).
		Render(strings.Join(paddedLines, "\n"))

	var out []string
	leadSpace := ""
	if indentLeading {
		leadSpace = " "
	}
	if strings.TrimSpace(displayTitle) != "" {
		for _, tl := range wrapTextBlock(displayTitle, cardW) {
			tLine := leadSpace + lipgloss.NewStyle().Foreground(titleCol).Bold(true).Render(padOrTruncate(tl, cardW))
			out = append(out, tLine)
		}
	}
	for _, bl := range strings.Split(box, "\n") {
		out = append(out, leadSpace+bl)
	}
	return out
}

func (m *Model) renderTab2Box(boxIdx int, title string, body string, cardW int, defaultTitleCol lipgloss.Color) []string {
	isSelectedBox := m.focus == focusMain && m.activeTab == 2 && m.tab2BoxCursor == boxIdx
	isEnteredBox := isSelectedBox && m.tab2BoxEntered
	return renderNavBox(isSelectedBox, isEnteredBox, title, body, cardW, defaultTitleCol, m.cfg.IsRussian(), true)
}

func (m *Model) renderConfigBox(boxIdx int, title string, body string, cardW int, defaultTitleCol lipgloss.Color) []string {
	isSelectedBox := m.cfgBoxCursor == boxIdx
	isEnteredBox := isSelectedBox && m.cfgBoxEntered
	return renderNavBox(isSelectedBox, isEnteredBox, title, body, cardW, defaultTitleCol, m.cfg.IsRussian(), false)
}

// renderRePromptTab renders Tab 2: Per-Chat AI Configuration & Skills Library with 2-level box navigation.
func (m *Model) renderRePromptTab(rec *storage.SkillOutputRecord, innerW int) []string {
	cardW := innerW - 4
	if cardW < 20 {
		cardW = 20
	}
	var out []string
	for _, hl := range wrapTextBlock(m.T("skills_tab_hdr"), innerW) {
		out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(hl, innerW)))
	}
	out = append(out, "")

	ch := m.activeChat()
	chatTitle := "Selected Chat"
	if ch != nil {
		chatTitle = sanitizeTerminalText(ch.Title)
	}

	// Box 0 (Top): AI Actions & Shortcuts Card
	var actionsBody string
	if m.cfg.IsRussian() {
		actionsBody = strings.Join([]string{
			"Навыки и результат ИИ:",
			"    [Enter] Войти в выбранный блок / применить действие (Esc — выйти из блока)",
			"    [x]     Отключить ИИ для текущего чата и сбросить настройки",
			"    [v / e] Просмотр навыка (v) или редактирование SKILL.md в $EDITOR (e)",
			"    [o / O] Показать навык в Finder (o) или открыть в редакторе macOS (O)",
			"    [n / I] Создать навык в терминале (n) или импортировать через Finder (I)",
			"    [r]     Запустить активный ИИ-навык для выбранного чата (Вкл. 3)",
			"Управление живым чатом ИИ:",
			"    [a]     Автоответ ИИ ([ВКЛ] / [ВЫКЛ]) на входящие сообщения в чате",
			"    [s]     Режим отправки ИИ: В черновик поля ввода (Enter) или Сразу в чат",
			"    [A]     Сгенерировать ответ ИИ сейчас (во вкладке 1 — клавиша 'a')",
			"    [m]     Разовая команда ИИ (запуск / дополнение / стоп)",
		}, "\n")
	} else {
		actionsBody = strings.Join([]string{
			"Skills & AI Output:",
			"    [Enter] Enter highlighted box / apply option (Esc to exit back to boxes)",
			"    [x]     Disable AI for selected chat & clear settings",
			"    [v / e] Preview skill prompt (v) or edit SKILL.md in $EDITOR (e)",
			"    [o / O] Reveal skill in macOS Finder (o) or open in macOS GUI editor (O)",
			"    [n / I] Create new skill in terminal (n) or import skill via Finder (I)",
			"    [r]     Run active AI skill on selected chat & view result in Tab 3",
			"Live Chat AI Control:",
			"    [a]     Toggle AI Auto-Reply ([ON] / [OFF]) for incoming messages",
			"    [s]     Toggle AI Send Mode (Draft to Compose Box vs Auto-Send)",
			"    [A]     Generate AI reply right now (or press 'a' in Tab 1 Live Chat)",
			"    [m]     Single-Shot AI instruction (run / append / stop)",
		}, "\n")
	}
	b0Start := len(out)
	out = append(out, m.renderTab2Box(0, m.T("skills_card_actions"), actionsBody, cardW, colPurple)...)
	m.tab2BoxBounds[0] = [2]int{b0Start, len(out)}
	out = append(out, "")

	// Box 1: Per-Chat AI Instructions Card
	chatCfg, chatAIEnabled := m.activeChatAIConfig()
	runNotesText := strings.TrimSpace(chatCfg.Notes)
	if ch != nil {
		r := m.getChatSingleShotRunner(ch.ChatID)
		r.mu.Lock()
		if r.running {
			if m.cfg.IsRussian() {
				runNotesText = fmt.Sprintf("[ВКЛ] [ВЫПОЛНЯЕТСЯ] %s", r.currentInstr)
			} else {
				runNotesText = fmt.Sprintf("[ON] [RUNNING] %s", r.currentInstr)
			}
			if len(r.queue) > 0 {
				runNotesText += fmt.Sprintf(" (+%d queued)", len(r.queue))
			}
		} else if len(r.history) > 0 {
			if m.cfg.IsRussian() {
				runNotesText = fmt.Sprintf("[ВКЛ] %s", strings.Join(r.history, " + "))
			} else {
				runNotesText = fmt.Sprintf("[ON] %s", strings.Join(r.history, " + "))
			}
		}
		r.mu.Unlock()
	}
	if runNotesText == "" {
		runNotesText = m.T("skills_notes_none")
	}
	autoReplyLabel := m.T("skills_autoreply_off")
	if chatCfg.AutoReply {
		autoReplyLabel = m.T("skills_autoreply_on")
	}
	sendModeLabel := m.T("skills_sendmode_draft")
	if chatCfg.SendMode == "auto_send" {
		sendModeLabel = m.T("skills_sendmode_auto")
	}
	var chatAIBody string
	var chatAICol lipgloss.Color
	if chatAIEnabled {
		chatAICol = colPurple
		chatAIBody = m.T("skills_chat_enabled", chatCfg.Skill, autoReplyLabel, sendModeLabel, runNotesText)
	} else {
		chatAICol = colYellow
		chatAIBody = m.T("skills_chat_disabled", config.DefaultSkillName)
	}
	if m.tab2BoxEntered && m.tab2BoxCursor == 1 {
		chatAIBody = highlightBox1EnteredOption(chatAIBody, m.tab2ChatOptCursor)
	}
	b1Start := len(out)
	out = append(out, m.renderTab2Box(1, m.T("skills_card_chat_ai", chatTitle), chatAIBody, cardW, chatAICol)...)
	m.tab2BoxBounds[1] = [2]int{b1Start, len(out)}
	out = append(out, "")

	// Box 2: Common AI Engine Configuration Card
	cliPath := m.cfg.AgyCLIBin()
	apiStatus := "[OFF] [NOT CONNECTED] agy CLI not found — press 'c'"
	if m.cfg.IsRussian() {
		apiStatus = "[ВЫКЛ] [НЕ ПОДКЛЮЧЕН] agy CLI не найден — нажмите 'c'"
	}
	if cliPath != "" {
		if m.cfg.IsRussian() {
			apiStatus = fmt.Sprintf("[ВКЛ] [ПОДКЛЮЧЕН] %s", cliPath)
		} else {
			apiStatus = fmt.Sprintf("[ON] [CONNECTED] %s", cliPath)
		}
	}
	modelName := strings.TrimSpace(m.cfg.AgyModel)
	if modelName == "" {
		modelName = "auto"
	}
	activeSkillLabel := chatCfg.Skill
	if !chatAIEnabled {
		activeSkillLabel = config.DefaultSkillName + " (default)"
	}
	aiConfigBody := strings.Join([]string{
		m.T("skills_lbl_engine", apiStatus, m.cfg.ActiveModelLabel(), modelName),
		m.T("skills_lbl_skill", activeSkillLabel, m.cfg.SkillsDir()),
		m.T("skills_lbl_diag", m.aiStatusLine),
	}, "\n")
	b2Start := len(out)
	out = append(out, m.renderTab2Box(2, m.T("skills_card_ai_config"), aiConfigBody, cardW, colGreen)...)
	m.tab2BoxBounds[2] = [2]int{b2Start, len(out)}
	out = append(out, "")

	// Box 3: Skills Library Card (cursor highlight inside only shown when user pressed Enter to enter Box 3)
	inSkillsBox := m.focus == focusMain && m.activeTab == 2 && m.tab2BoxEntered && m.tab2BoxCursor == 3
	innerTextW := cardW - 4
	if innerTextW < 24 {
		innerTextW = 24
	}
	var skillRows []string
	if len(m.skillList) == 0 {
		skillRows = append(skillRows, m.T("skills_empty_lib"))
	} else {
		nameW := 18
		kindW := 13
		if innerTextW < 68 {
			nameW = 14
			kindW = 11
		}
		for idx, sk := range m.skillList {
			prefix := "  "
			isHovered := inSkillsBox && idx == m.skillCursor
			if isHovered {
				prefix = "▸ "
			}
			activeBadge := "         "
			isChatActiveSkill := chatAIEnabled && sk.Name == chatCfg.Skill && sk.Kind == skills.KindSummary
			if isChatActiveSkill {
				activeBadge = m.T("skills_badge_active")
			}
			activeBadge = padRightVisual(activeBadge, 10)

			nameStr := padRightVisual(truncateSingleLine(sk.Name, nameW), nameW)

			kindBadge := fmt.Sprintf("[%s]", strings.Join(sk.Media, ","))
			if sk.Kind == skills.KindSummary {
				kindBadge = m.T("skills_badge_summary")
			}
			kindBadge = padRightVisual(truncateSingleLine(kindBadge, kindW), kindW)

			// Total non-desc width: prefix(2) + activeBadge(10) + space(1) + nameW + space(1) + kindW + space(1)
			fixedW := 2 + 10 + 1 + nameW + 1 + kindW + 1
			descW := maxInt(10, innerTextW-fixedW)
			isSkillActive := m.activeTab == 2 && idx == m.skillCursor && (inSkillsBox || m.tab2BoxCursor == 3 || m.focus == focusMain)
			desc := renderRunningLine(sk.Description, descW, isSkillActive, m.marqueeTick)

			row := fmt.Sprintf("%s%s %s %s %s", prefix, activeBadge, nameStr, kindBadge, desc)
			skillRows = append(skillRows, truncateSingleLine(row, innerTextW))
		}
	}

	skillsTitle := m.T("skills_card_catalog", minInt(m.skillCursor+1, len(m.skillList)), len(m.skillList))
	skillCardLines := append([]string{}, skillRows...)
	skillCardLines = append(skillCardLines, "", "  "+m.T("skills_catalog_keys"))
	b3Start := len(out)
	box3Lines := m.renderTab2Box(3, skillsTitle, strings.Join(skillCardLines, "\n"), cardW, colCyan)
	// Title lines (1 or 2) + 1 top border line before skill row 0:
	titleLineCount := strings.Count(skillsTitle, "\n") + 1
	if m.focus == focusMain && m.activeTab == 2 && m.tab2BoxCursor == 3 && m.tab2BoxEntered && cardW < 85 {
		titleLineCount = 2
	}
	m.tab2SkillOffset = titleLineCount + 1
	out = append(out, box3Lines...)
	m.tab2BoxBounds[3] = [2]int{b3Start, len(out)}
	out = append(out, "")

	// Box 4: Selected Chat Latest AI Skill Output Card
	b4Start := len(out)
	if rec != nil {
		preview := truncateSingleLine(strings.TrimSpace(rec.Output), maxInt(24, cardW-8))
		outputLines := []string{
			m.T("skills_output_meta", rec.CreatedAt, rec.Skill, rec.ModelUsed, rec.MessageCount),
			m.T("skills_output_preview", preview),
			m.T("skills_period_notes", runNotesText),
		}
		out = append(out, m.renderTab2Box(4, m.T("skills_card_run"), strings.Join(outputLines, "\n"), cardW, colBlue)...)
	} else {
		noOutputBody := m.T("skills_output_none", runNotesText)
		out = append(out, m.renderTab2Box(4, m.T("skills_card_run"), noOutputBody, cardW, colBlue)...)
	}
	m.tab2BoxBounds[4] = [2]int{b4Start, len(out)}
	return out
}

func (m *Model) renderTraceTab(innerW int) []string {
	var out []string
	for _, hl := range wrapTextBlock(m.T("trace_hdr"), innerW) {
		out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(hl, innerW)))
	}
	out = append(out, "")
	for _, line := range m.traceLogs {
		out = append(out, "  "+truncateSingleLine(line, innerW-4))
	}
	return out
}
