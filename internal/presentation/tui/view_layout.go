package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
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
		var activeSchedCount int
		for _, t := range m.scheduledTasks {
			if t.ChatID == ch.ChatID {
				activeSchedCount++
			}
		}
		if activeSchedCount > 0 {
			aiBadge += " " + lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(fmt.Sprintf("[⏰%d]", activeSchedCount))
		}
	}

	left := fmt.Sprintf(" %s │ %s%s",
		lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(icon+" "+m.statusText),
		m.T("active_chat_hdr", lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(chatTitle)),
		aiBadge,
	)

	schedIndicator := ""
	if count := len(m.scheduledTasks); count > 0 {
		schedIndicator = lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(fmt.Sprintf("⏰%d ", count))
	}
	elapsedSec := int(time.Since(m.startTime).Seconds())
	right := fmt.Sprintf("%s%s   %ds ",
		schedIndicator,
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
		case 4:
			keys = append(keys,
				keyPair{"o", m.T("key_finder")},
				keyPair{"v", "Viewer"},
				keyPair{"c", "Copy Trace"},
				keyPair{"Ctrl+L", "Clear"},
				keyPair{"Ctrl+X", "Stop Tasks"},
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
