package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"telepilot/internal/domain/config"
	"telepilot/internal/domain/skills"
	"telepilot/internal/domain/storage"
)

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
			"    [Ctrl+X] Остановить все фоновые задачи (или 'x' для выбранного чата)",
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
			"    [Ctrl+X] Stop all background scheduled tasks (or 'x' for chat)",
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
	var chatSchedCount int
	var nextSchedStr string
	now := time.Now()
	for _, t := range m.scheduledTasks {
		if t.ChatID == m.selectedChatID {
			chatSchedCount++
			if nextSchedStr == "" {
				rem := t.DueAt.Sub(now)
				if rem < 0 {
					rem = 0
				}
				nextSchedStr = formatDurationShort(rem) + " (" + truncateSingleLine(t.TaskPrompt, 18) + ")"
			}
		}
	}
	schedLine := m.T("skills_sched_none")
	if chatSchedCount > 0 {
		schedLine = m.T("skills_sched_active", chatSchedCount, nextSchedStr)
	}
	chatAIBody += "\n" + schedLine

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
	quotaText := m.aiQuotaStatus
	if strings.TrimSpace(quotaText) == "" {
		quotaText = initialAIQuotaStatus(m.cfg)
	}
	box2Col := colGreen
	switch {
	case strings.Contains(quotaText, "[QUOTA REACHED]") || strings.Contains(quotaText, "[ERROR]"):
		box2Col = colRed
	case strings.Contains(quotaText, "[TIMEOUT]") || strings.Contains(quotaText, "[AUTH REQUIRED]") || strings.Contains(quotaText, "[OFF]") || strings.Contains(quotaText, "[ВЫКЛ]"):
		box2Col = colYellow
	}
	aiConfigBody := strings.Join([]string{
		m.T("skills_lbl_engine", apiStatus, m.cfg.ActiveModelLabel(), modelName),
		m.T("skills_lbl_quota", quotaText),
		m.T("skills_lbl_skill", activeSkillLabel, m.cfg.SkillsDir()),
		m.T("skills_lbl_diag", m.aiStatusLine),
	}, "\n")
	b2Start := len(out)
	out = append(out, m.renderTab2Box(2, m.T("skills_card_ai_config"), aiConfigBody, cardW, box2Col)...)
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
