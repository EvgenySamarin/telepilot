package tui

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"

	"telepilot/internal/domain/skills"
)

func (m *Model) handleChatSearchKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.chatSearching = false
		m.chatSearchQuery = ""
		m.chatScrollOffset = 0
		for i := range m.chats {
			if m.chats[i].ChatID == m.selectedChatID {
				m.chatCursor = i
				break
			}
		}
		return m, nil
	case "ctrl+c":
		if m.qrCancel != nil {
			m.qrCancel()
		}
		if m.liveStreamCancel != nil {
			m.liveStreamCancel()
		}
		return m, tea.Quit
	case "ctrl+u":
		m.chatSearchQuery = ""
		m.chatScrollOffset = 0
		if len(m.chats) > 0 {
			m.chatCursor = 0
		}
		return m, nil
	case "up", "ctrl+p", "ctrl+k":
		matches := m.filteredChatIndices()
		if len(matches) > 0 {
			pos := m.filteredChatCursorPos(matches)
			if pos > 0 {
				m.chatCursor = matches[pos-1]
				m.marqueeTick = 0
			}
		}
		return m, nil
	case "down", "ctrl+n", "ctrl+j":
		matches := m.filteredChatIndices()
		if len(matches) > 0 {
			pos := m.filteredChatCursorPos(matches)
			if pos < len(matches)-1 {
				m.chatCursor = matches[pos+1]
				m.marqueeTick = 0
			}
		}
		return m, nil
	case "enter":
		m.chatSearching = false
		matches := m.filteredChatIndices()
		if len(matches) > 0 {
			pos := m.filteredChatCursorPos(matches)
			m.chatCursor = matches[pos]
			return m, m.selectChatAtIndex(m.chatCursor)
		}
		return m, nil
	case "backspace":
		if len(m.chatSearchQuery) > 0 {
			_, sz := utf8.DecodeLastRuneInString(m.chatSearchQuery)
			m.chatSearchQuery = m.chatSearchQuery[:len(m.chatSearchQuery)-sz]
		}
		matches := m.filteredChatIndices()
		if len(matches) > 0 {
			m.chatCursor = matches[0]
			m.marqueeTick = 0
		}
		return m, nil
	default:
		if utf8.RuneCountInString(key) == 1 {
			m.chatSearchQuery += key
		} else if key == "space" || key == " " {
			m.chatSearchQuery += " "
		}
		matches := m.filteredChatIndices()
		if len(matches) > 0 {
			m.chatCursor = matches[0]
			m.marqueeTick = 0
		}
		return m, nil
	}
}

func (m *Model) handleLiveComposeKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		m.chatComposing = false
		return m, nil
	case "ctrl+c":
		if m.qrCancel != nil {
			m.qrCancel()
		}
		if m.liveStreamCancel != nil {
			m.liveStreamCancel()
		}
		return m, tea.Quit
	case "ctrl+u":
		m.chatInputBuffer = ""
		return m, nil
	case "enter":
		text := strings.TrimSpace(m.chatInputBuffer)
		m.chatInputBuffer = ""
		m.chatComposing = false
		if text == "" {
			return m, nil
		}
		ch := m.activeChat()
		if ch == nil {
			return m, nil
		}
		return m, m.sendLiveMessageCmd(*ch, text)
	case "backspace":
		if len(m.chatInputBuffer) > 0 {
			_, sz := utf8.DecodeLastRuneInString(m.chatInputBuffer)
			m.chatInputBuffer = m.chatInputBuffer[:len(m.chatInputBuffer)-sz]
		}
		return m, nil
	default:
		if utf8.RuneCountInString(key) == 1 {
			m.chatInputBuffer += key
		} else if key == "space" || key == " " {
			m.chatInputBuffer += " "
		}
		return m, nil
	}
}

func (m *Model) handleModalKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	switch m.activeModal {
	case modalHelp:
		if key == "esc" || key == "?" || key == "q" || key == "enter" {
			m.activeModal = modalNone
		}
		return m, nil

	case modalConfirmDelete:
		switch key {
		case "esc", "n", "N", "q":
			m.activeModal = modalNone
			return m, nil
		case "h", "left", "k", "up":
			m.deleteBtnFocus = 0
			return m, nil
		case "l", "right", "j", "down":
			m.deleteBtnFocus = 1
			return m, nil
		case "tab", "shift+tab":
			m.deleteBtnFocus = (m.deleteBtnFocus + 1) % 2
			return m, nil
		case "y", "Y", "d":
			m.executeDeleteActivePeriod()
			m.activeModal = modalNone
			return m, nil
		case "enter":
			if m.deleteBtnFocus == 0 {
				m.executeDeleteActivePeriod()
			}
			m.activeModal = modalNone
			return m, nil
		}
		return m, nil

	case modalPeriod:
		switch key {
		case "esc":
			m.activeModal = modalNone
		case "enter":
			val := strings.TrimSpace(m.inputBuffer)
			if val == "" {
				val = "1m"
			}
			m.periodSpec = val
			m.cfg.DefaultPeriod = val
			if cfg, ok := m.activeChatAIConfig(); ok {
				m.saveCurrentChatAIConfig(cfg.Skill, cfg.Notes, val, true)
			}
			m.addTrace("Updated period window for chat to: " + val)
			m.activeModal = modalNone
		case "backspace":
			if len(m.inputBuffer) > 0 {
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-1]
			}
		default:
			if len(key) == 1 {
				m.inputBuffer += key
			}
		}
		return m, nil

	case modalPrompt:
		switch key {
		case "esc":
			m.activeModal = modalNone
		case "enter":
			// Pure Single-Shot: cancels any previous running instruction for this chat and runs fresh
			val := strings.TrimSpace(m.inputBuffer)
			m.inputBuffer = ""
			m.activeModal = modalNone
			if val == "" {
				return m, nil
			}
			m.activeTab = 1
			m.focus = focusMain
			return m, m.runSingleShotInstructionCmd(val, false)
		case "ctrl+a", "ctrl+r":
			// Append Mode: keeps any currently running instruction and appends/queues the new one on top
			val := strings.TrimSpace(m.inputBuffer)
			m.inputBuffer = ""
			m.activeModal = modalNone
			if val == "" {
				return m, nil
			}
			m.activeTab = 1
			m.focus = focusMain
			return m, m.runSingleShotInstructionCmd(val, true)
		case "ctrl+x":
			// Stop / Cancel any running single-shot instruction for this chat
			if ch := m.activeChat(); ch != nil {
				if m.cancelSingleShotForChat(ch.ChatID, ch.Title) {
					m.statusText = "Single-Shot stopped"
				}
			}
			return m, nil
		case "ctrl+u":
			m.inputBuffer = ""
		case "backspace":
			if len(m.inputBuffer) > 0 {
				_, sz := utf8.DecodeLastRuneInString(m.inputBuffer)
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-sz]
			}
		default:
			if utf8.RuneCountInString(key) == 1 {
				m.inputBuffer += key
			} else if key == "space" {
				m.inputBuffer += " "
			}
		}
		return m, nil

	case modalNewSkill:
		switch key {
		case "esc":
			m.activeModal = modalNone
		case "enter":
			name := strings.TrimSpace(m.inputBuffer)
			if name != "" {
				path, err := skills.Create(m.cfg.SkillsDir(), name)
				if err != nil {
					m.addTrace(fmt.Sprintf("Could not create skill %s: %v", name, err))
				} else {
					m.addTrace(fmt.Sprintf("Created new skill: %s", path))
					m.reloadSkills()
					for idx, sk := range m.skillList {
						if sk.Path == path {
							m.skillCursor = idx
							break
						}
					}
					m.activeModal = modalNone
					return m, openEditorCmd(path)
				}
			}
			m.activeModal = modalNone
		case "ctrl+u":
			m.inputBuffer = ""
		case "backspace":
			if len(m.inputBuffer) > 0 {
				_, sz := utf8.DecodeLastRuneInString(m.inputBuffer)
				m.inputBuffer = m.inputBuffer[:len(m.inputBuffer)-sz]
			}
		default:
			if utf8.RuneCountInString(key) == 1 {
				m.inputBuffer += key
			}
		}
		return m, nil

	case modalAIConfig:
		switch key {
		case "esc":
			m.activeModal = modalNone
			return m, nil
		case "tab", "down", "shift+tab", "up":
			m.aiFormFocus = (m.aiFormFocus + 1) % 2
			return m, nil
		case "f2":
			curModel := strings.TrimSpace(m.aiFormFields[1])
			switch curModel {
			case "auto":
				m.aiFormFields[1] = "pro"
			case "pro":
				m.aiFormFields[1] = "flash"
			case "flash":
				m.aiFormFields[1] = "flash_lite"
			default:
				m.aiFormFields[1] = "auto"
			}
			m.saveAIConfigForm()
			m.aiStatusLine = fmt.Sprintf("Switched Antigravity CLI (agy) model to %s. Press Enter or Ctrl+T to test.", m.aiFormFields[1])
			return m, nil
		case "ctrl+t", "enter":
			return m, m.startAITestCmd()
		case "ctrl+s":
			m.saveAIConfigForm()
			m.activeModal = modalNone
			return m, nil
		case "ctrl+u":
			m.aiFormFields[m.aiFormFocus] = ""
			return m, nil
		case "backspace":
			cur := m.aiFormFields[m.aiFormFocus]
			if len(cur) > 0 {
				_, sz := utf8.DecodeLastRuneInString(cur)
				m.aiFormFields[m.aiFormFocus] = cur[:len(cur)-sz]
			}
			return m, nil
		default:
			if utf8.RuneCountInString(key) == 1 {
				m.aiFormFields[m.aiFormFocus] += key
			}
			return m, nil
		}

	case modalLoginForm:
		switch key {
		case "esc":
			m.activeModal = modalNone
			return m, nil
		case "tab", "down":
			m.formFocus = (m.formFocus + 1) % 4
			return m, nil
		case "shift+tab", "up":
			m.formFocus = (m.formFocus + 3) % 4
			return m, nil
		case "f2":
			if strings.HasPrefix(m.formFields[2], "149.154.167.50") {
				m.formFields[2] = "149.154.167.40:443"
			} else {
				m.formFields[2] = "149.154.167.50:443"
			}
			m.saveFormCredentials()
			m.connLines = []string{
				fmt.Sprintf("Switched MTProto server to %s.", m.formFields[2]),
				"Press 't' to test connection or 'g' to generate Phone QR code.",
			}
			return m, nil
		case "t", "ctrl+t":
			if key == "ctrl+t" || m.formFocus == 0 || m.formFocus == 2 {
				return m, m.startConnectionTestCmd()
			}
			m.formFields[m.formFocus] += "t"
			return m, nil
		case "g", "ctrl+g":
			if key == "ctrl+g" || m.formFocus == 0 || m.formFocus == 2 {
				return m, m.startQRLoginCmd()
			}
			m.formFields[m.formFocus] += "g"
			return m, nil
		case "enter":
			return m, m.startConnectionTestCmd()
		case "ctrl+u":
			m.formFields[m.formFocus] = ""
			return m, nil
		case "backspace":
			cur := m.formFields[m.formFocus]
			if len(cur) > 0 {
				_, sz := utf8.DecodeLastRuneInString(cur)
				m.formFields[m.formFocus] = cur[:len(cur)-sz]
			}
			return m, nil
		default:
			if utf8.RuneCountInString(key) == 1 {
				m.formFields[m.formFocus] += key
			}
			return m, nil
		}

	case modalLoginQR:
		switch key {
		case "esc":
			if m.qrCancel != nil {
				m.qrCancel()
				m.qrCancel = nil
			}
			m.activeModal = modalNone
			return m, nil
		case "c":
			if m.qrCancel != nil {
				m.qrCancel()
				m.qrCancel = nil
			}
			m.activeModal = modalLoginForm
			m.formFocus = 2
			return m, nil
		case "r":
			return m, m.startQRLoginCmd()
		case "v":
			if m.qrPNGPath != "" {
				_ = exec.Command("qlmanage", "-p", m.qrPNGPath).Start()
			}
			return m, nil
		}

	case modalSkillPreview:
		switch key {
		case "esc", "q", "v", "enter":
			m.activeModal = modalNone
			return m, nil
		case "j", "down":
			m.previewScroll++
			return m, nil
		case "k", "up":
			if m.previewScroll > 0 {
				m.previewScroll--
			}
			return m, nil
		case "ctrl+d":
			m.previewScroll += 6
			return m, nil
		case "ctrl+u":
			m.previewScroll = maxInt(0, m.previewScroll-6)
			return m, nil
		case "g":
			m.previewScroll = 0
			return m, nil
		case "G":
			m.previewScroll += 1000
			return m, nil
		case "e":
			m.activeModal = modalNone
			if len(m.skillList) > 0 && m.skillCursor >= 0 && m.skillCursor < len(m.skillList) {
				return m, openEditorCmd(m.skillList[m.skillCursor].Path)
			}
			return m, nil
		}
		return m, nil

	case modalGlobalConfig:
		switch key {
		case "esc", "q":
			m.activeModal = modalNone
			return m, nil
		case "tab", "shift+tab", "f2", "h", "l", "left", "right", "j", "k", "up", "down", " ":
			m.cfgLangIndex = (m.cfgLangIndex + 1) % 2
			newLang := "en"
			if m.cfgLangIndex == 1 {
				newLang = "ru"
			}
			m.cfg.SaveLanguage(newLang)
			m.statusText = m.T("status_ready")
			if newLang == "ru" {
				m.addTrace("Язык интерфейса изменен на русский (ru). Настройки сохранены в .env.")
			} else {
				m.addTrace("Interface language switched to English (en). Settings saved to .env.")
			}
			return m, nil
		case "1", "e", "E":
			m.cfgLangIndex = 0
			m.cfg.SaveLanguage("en")
			m.statusText = m.T("status_ready")
			m.addTrace("Interface language switched to English (en). Settings saved to .env.")
			return m, nil
		case "2", "r", "R":
			m.cfgLangIndex = 1
			m.cfg.SaveLanguage("ru")
			m.statusText = m.T("status_ready")
			m.addTrace("Язык интерфейса изменен на русский (ru). Настройки сохранены в .env.")
			return m, nil
		case "enter", "ctrl+s":
			newLang := "en"
			if m.cfgLangIndex == 1 {
				newLang = "ru"
			}
			m.cfg.SaveLanguage(newLang)
			m.statusText = m.T("status_ready")
			m.activeModal = modalNone
			return m, nil
		}
		return m, nil
	}

	return m, nil
}

func (m *Model) handleMainKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "backspace":
		if m.activeTab == 2 && m.tab2BoxEntered {
			m.tab2BoxEntered = false
			return m, nil
		}
		if m.chatSearchQuery != "" {
			m.chatSearchQuery = ""
			m.chatScrollOffset = 0
			return m, nil
		}

	case "\\", "/":
		m.focus = focusChats
		m.chatSearching = true
		m.chatSearchQuery = ""
		m.chatScrollOffset = 0
		return m, nil

	case "q", "ctrl+c":
		if m.qrCancel != nil {
			m.qrCancel()
		}
		if m.liveStreamCancel != nil {
			m.liveStreamCancel()
		}
		return m, tea.Quit

	case "tab", "shift+tab":
		if m.focus == focusChats {
			m.focus = focusMain
		} else {
			m.focus = focusChats
		}
		return m, nil

	case "h", "left":
		m.focus = focusChats
		return m, nil

	case "l", "right":
		m.focus = focusMain
		return m, nil

	case "1":
		m.activeTab = 1
		m.mainScroll = 0
	case "2":
		m.activeTab = 2
		m.tab2BoxCursor = 0
		m.tab2BoxEntered = false
		m.mainScroll = 0
	case "3":
		m.activeTab = 3
		m.mainScroll = 0
	case "4":
		m.activeTab = 4
		m.mainScroll = 0

	case "L":
		m.activeModal = modalLoginForm
		m.formFocus = 2
		return m, m.startConnectionTestCmd()

	case "c":
		m.aiFormFields[0] = m.cfg.AgyCLIBin()
		m.aiFormFields[1] = m.cfg.AgyModel
		if strings.TrimSpace(m.aiFormFields[1]) == "" {
			m.aiFormFields[1] = "auto"
		}
		m.aiFormFocus = 0
		m.activeModal = modalAIConfig

	case "C":
		m.cfgLangIndex = 0
		if m.cfg.IsRussian() {
			m.cfgLangIndex = 1
		}
		m.activeModal = modalGlobalConfig

	case "?":
		m.activeModal = modalHelp

	case "i":
		if m.activeTab == 1 {
			m.focus = focusMain
			m.chatComposing = true
			return m, nil
		}

	case "[":
		if m.activeTab == 3 && len(m.periods) > 1 {
			m.periodCursor = (m.periodCursor - 1 + len(m.periods)) % len(m.periods)
			m.mediaCursor = 0
			m.mainScroll = 0
		} else if m.mainScroll > 0 {
			m.mainScroll--
		}

	case "]":
		if m.activeTab == 3 && len(m.periods) > 1 {
			m.periodCursor = (m.periodCursor + 1) % len(m.periods)
			m.mediaCursor = 0
			m.mainScroll = 0
		} else {
			m.mainScroll++
		}

	case "g":
		switch m.focus {
		case focusChats:
			if matches := m.filteredChatIndices(); len(matches) > 0 {
				m.chatCursor = matches[0]
				m.marqueeTick = 0
			}
		case focusMain:
			switch m.activeTab {
			case 1:
				m.liveMsgCursor = 0
			case 2:
				if !m.tab2BoxEntered {
					m.tab2BoxCursor = 0
					m.mainScroll = 0
				} else if m.tab2BoxCursor == 3 {
					m.skillCursor = 0
					m.scrollTab2ToBox(3)
				} else if m.tab2BoxCursor == 1 {
					m.tab2ChatOptCursor = 0
				}
			case 3:
				m.mediaCursor = 0
			}
			m.mainScroll = 0
		}

	case "G":
		switch m.focus {
		case focusChats:
			if matches := m.filteredChatIndices(); len(matches) > 0 {
				m.chatCursor = matches[len(matches)-1]
				m.marqueeTick = 0
			}
		case focusMain:
			switch m.activeTab {
			case 1:
				if len(m.liveMessages) > 0 {
					m.liveMsgCursor = len(m.liveMessages) - 1
				}
			case 2:
				if !m.tab2BoxEntered {
					m.tab2BoxCursor = 4
					m.scrollTab2ToBox(4)
				} else if m.tab2BoxCursor == 3 && len(m.skillList) > 0 {
					m.skillCursor = len(m.skillList) - 1
					m.scrollTab2ToBox(3)
				} else if m.tab2BoxCursor == 1 {
					m.tab2ChatOptCursor = 3
				}
			default:
				m.mainScroll += 1000
			}
		}

	case "ctrl+d":
		switch m.focus {
		case focusChats:
			if matches := m.filteredChatIndices(); len(matches) > 0 {
				pos := m.filteredChatCursorPos(matches)
				m.chatCursor = matches[minInt(len(matches)-1, pos+6)]
				m.marqueeTick = 0
			}
		case focusMain:
			if m.activeTab == 1 && len(m.liveMessages) > 0 {
				m.liveMsgCursor = minInt(len(m.liveMessages)-1, m.liveMsgCursor+5)
			} else {
				m.mainScroll += 8
			}
		}

	case "ctrl+u":
		switch m.focus {
		case focusChats:
			if matches := m.filteredChatIndices(); len(matches) > 0 {
				pos := m.filteredChatCursorPos(matches)
				m.chatCursor = matches[maxInt(0, pos-6)]
				m.marqueeTick = 0
			}
		case focusMain:
			if m.activeTab == 1 {
				m.liveMsgCursor = maxInt(0, m.liveMsgCursor-5)
			} else {
				m.mainScroll = maxInt(0, m.mainScroll-8)
			}
		}

	case "k", "up":
		switch m.focus {
		case focusChats:
			if matches := m.filteredChatIndices(); len(matches) > 0 {
				pos := m.filteredChatCursorPos(matches)
				if pos > 0 {
					m.chatCursor = matches[pos-1]
					m.marqueeTick = 0
				}
			}
		case focusMain:
			switch m.activeTab {
			case 1:
				if m.liveMsgCursor > 0 {
					m.liveMsgCursor--
				}
			case 2:
				if !m.tab2BoxEntered {
					if m.tab2BoxCursor > 0 {
						m.tab2BoxCursor--
					}
					m.scrollTab2ToBox(m.tab2BoxCursor)
				} else if m.tab2BoxCursor == 1 {
					if m.tab2ChatOptCursor > 0 {
						m.tab2ChatOptCursor--
					}
				} else if m.tab2BoxCursor == 3 {
					if m.skillCursor > 0 {
						m.skillCursor--
					}
					m.scrollTab2ToSkill()
				}
			default:
				if m.mainScroll > 0 {
					m.mainScroll--
				}
			}
		}

	case "j", "down":
		switch m.focus {
		case focusChats:
			if matches := m.filteredChatIndices(); len(matches) > 0 {
				pos := m.filteredChatCursorPos(matches)
				if pos < len(matches)-1 {
					m.chatCursor = matches[pos+1]
					m.marqueeTick = 0
				}
			}
		case focusMain:
			switch m.activeTab {
			case 1:
				if m.liveMsgCursor < len(m.liveMessages)-1 {
					m.liveMsgCursor++
				}
			case 2:
				if !m.tab2BoxEntered {
					if m.tab2BoxCursor < 4 {
						m.tab2BoxCursor++
					}
					m.scrollTab2ToBox(m.tab2BoxCursor)
				} else if m.tab2BoxCursor == 1 {
					if m.tab2ChatOptCursor < 3 {
						m.tab2ChatOptCursor++
					}
				} else if m.tab2BoxCursor == 3 {
					if m.skillCursor < len(m.skillList)-1 {
						m.skillCursor++
					}
					m.scrollTab2ToSkill()
				}
			default:
				m.mainScroll++
			}
		}

	case "J":
		m.mainScroll += 4

	case "K":
		m.mainScroll = maxInt(0, m.mainScroll-4)

	case "a":
		if m.activeTab == 1 {
			if ch := m.activeChat(); ch != nil {
				sendMode := "draft"
				if cfg, ok := m.activeChatAIConfig(); ok && cfg.SendMode != "" {
					sendMode = cfg.SendMode
				}
				return m, m.generateAIChatReplyCmd(*ch, m.liveMessages, sendMode)
			}
		} else if m.activeTab == 2 {
			m.toggleChatAutoReply()
			return m, nil
		}

	case "A":
		if m.activeTab == 1 || m.activeTab == 2 {
			if ch := m.activeChat(); ch != nil {
				sendMode := "draft"
				if cfg, ok := m.activeChatAIConfig(); ok && cfg.SendMode != "" {
					sendMode = cfg.SendMode
				}
				return m, m.generateAIChatReplyCmd(*ch, m.liveMessages, sendMode)
			}
		}

	case "s":
		if m.activeTab == 2 {
			m.toggleChatSendMode()
			return m, nil
		}

	case "x":
		if m.activeTab == 2 {
			ch := m.activeChat()
			if ch != nil {
				m.cancelSingleShotForChat(ch.ChatID, ch.Title)
				m.saveCurrentChatAIConfig("", "", m.periodSpec, false)
				m.addTrace(fmt.Sprintf("Disabled per-chat AI configuration for '%s'", ch.Title))
			}
		} else if m.activeTab == 3 {
			if rec := m.activePeriodRecord(); rec != nil {
				m.deleteBtnFocus = 0
				m.activeModal = modalConfirmDelete
			}
		}

	case "d":
		if m.activeTab == 3 {
			if rec := m.activePeriodRecord(); rec != nil {
				m.deleteBtnFocus = 0
				m.activeModal = modalConfirmDelete
			}
		}

	case " ":
		if m.focus == focusChats {
			return m, m.selectChatAtIndex(m.chatCursor)
		}

	case "enter":
		if m.focus == focusChats {
			cmd := m.selectChatAtIndex(m.chatCursor)
			m.focus = focusMain
			return m, cmd
		}
		if m.activeTab == 1 {
			m.chatComposing = true
			return m, nil
		}
		if m.activeTab == 2 {
			if !m.tab2BoxEntered {
				switch m.tab2BoxCursor {
				case 0:
					m.activeModal = modalHelp
				case 1:
					m.tab2BoxEntered = true
					m.tab2ChatOptCursor = 0
					m.scrollTab2ToBox(1)
				case 2:
					m.aiFormFields[0] = m.cfg.AgyCLIBin()
					m.aiFormFields[1] = m.cfg.AgyModel
					if strings.TrimSpace(m.aiFormFields[1]) == "" {
						m.aiFormFields[1] = "auto"
					}
					m.aiFormFocus = 0
					m.activeModal = modalAIConfig
				case 3:
					m.tab2BoxEntered = true
					m.scrollTab2ToSkill()
				case 4:
					if len(m.periods) > 0 {
						m.activeTab = 3
						m.mainScroll = 0
					} else if !m.statusBusy {
						return m, m.runChatAICmd()
					}
				}
				return m, nil
			}

			// Inside an entered box in Tab 2:
			if m.tab2BoxCursor == 1 {
				switch m.tab2ChatOptCursor {
				case 0:
					// Jump to Skills Library box to pick active skill
					m.tab2BoxCursor = 3
					m.tab2BoxEntered = true
					m.scrollTab2ToSkill()
				case 1:
					m.toggleChatAutoReply()
				case 2:
					m.toggleChatSendMode()
				case 3:
					m.activeModal = modalPrompt
					m.inputBuffer = ""
				}
				return m, nil
			}

			if m.tab2BoxCursor == 3 {
				if m.skillCursor >= 0 && m.skillCursor < len(m.skillList) {
					sel := m.skillList[m.skillCursor]
					if sel.Kind == skills.KindSummary {
						m.activeSkill = sel.Name
						m.saveCurrentChatAIConfig(sel.Name, m.runNotes, m.periodSpec, true)
						if ch := m.activeChat(); ch != nil {
							m.addTrace(fmt.Sprintf("Enabled AI skill '%s' for chat '%s'", sel.Name, ch.Title))
						}
					} else {
						m.addTrace(fmt.Sprintf("Media skill '%s' covers: %s", sel.Name, strings.Join(sel.Media, ", ")))
					}
				}
				return m, nil
			}
			return m, nil
		}

	case "m":
		if m.activeTab == 1 || m.activeTab == 2 || m.activeTab == 3 {
			m.activeModal = modalPrompt
			m.inputBuffer = ""
		}

	case "e":
		if m.activeTab == 2 && len(m.skillList) > 0 {
			if m.skillCursor < 0 || m.skillCursor >= len(m.skillList) {
				m.skillCursor = 0
			}
			sk := m.skillList[m.skillCursor]
			m.addTrace("Opening skill in $EDITOR: " + sk.Path)
			return m, openEditorCmd(sk.Path)
		}

	case "n":
		if m.focus == focusChats && m.chatSearchQuery != "" {
			if matches := m.filteredChatIndices(); len(matches) > 0 {
				pos := m.filteredChatCursorPos(matches)
				m.chatCursor = matches[(pos+1)%len(matches)]
				m.marqueeTick = 0
				return m, m.selectChatAtIndex(m.chatCursor)
			}
			return m, nil
		}
		if m.activeTab == 2 {
			m.inputBuffer = ""
			m.activeModal = modalNewSkill
		} else if m.activeTab == 3 {
			if rec := m.activePeriodRecord(); rec != nil && len(rec.MediaItems) > 0 {
				m.mediaCursor = (m.mediaCursor + 1) % len(rec.MediaItems)
			}
		}

	case "N":
		if m.focus == focusChats && m.chatSearchQuery != "" {
			if matches := m.filteredChatIndices(); len(matches) > 0 {
				pos := m.filteredChatCursorPos(matches)
				m.chatCursor = matches[(pos-1+len(matches))%len(matches)]
				m.marqueeTick = 0
				return m, m.selectChatAtIndex(m.chatCursor)
			}
			return m, nil
		}

	case "p":
		if m.activeTab == 2 || m.activeTab == 3 {
			m.activeModal = modalPeriod
			m.inputBuffer = m.periodSpec
		}

	case "r":
		if m.activeTab == 1 {
			if ch := m.activeChat(); ch != nil {
				m.statusText = "Refreshing chat..."
				return m, m.loadChatMessagesCmd(*ch)
			}
			return m, nil
		}
		if !m.statusBusy {
			return m, m.runChatAICmd()
		}

	case "o":
		if m.activeTab == 3 {
			rec := m.activePeriodRecord()
			if rec != nil && rec.FolderPath != "" {
				_ = exec.Command("open", rec.FolderPath).Start()
				m.addTrace("Opened in Finder: " + rec.FolderPath)
			}
		}

	case "v":
		switch m.activeTab {
		case 1:
			ch := m.activeChat()
			if ch != nil && m.liveMsgCursor >= 0 && m.liveMsgCursor < len(m.liveMessages) {
				lm := m.liveMessages[m.liveMsgCursor]
				if lm.MediaType != "" {
					return m, m.downloadAndOpenLiveMediaCmd(*ch, lm)
				}
			}
		case 2:
			if len(m.skillList) > 0 {
				if m.skillCursor < 0 || m.skillCursor >= len(m.skillList) {
					m.skillCursor = 0
				}
				m.previewScroll = 0
				m.activeModal = modalSkillPreview
			}
		case 3:
			rec := m.activePeriodRecord()
			if rec != nil {
				if len(rec.MediaItems) > 0 {
					if m.mediaCursor < 0 || m.mediaCursor >= len(rec.MediaItems) {
						m.mediaCursor = 0
					}
					it := rec.MediaItems[m.mediaCursor]
					if it.MediaType == "voice" {
						_ = exec.Command("afplay", it.FilePath).Start()
						m.addTrace("Playing audio: " + it.FilePath)
					} else {
						_ = exec.Command("qlmanage", "-p", it.FilePath).Start()
						m.addTrace("QuickLook preview: " + it.FilePath)
					}
				} else {
					mdPath := filepath.Join(rec.FolderPath, "summary.md")
					_ = exec.Command("qlmanage", "-p", mdPath).Start()
					m.addTrace("QuickLook summary: " + mdPath)
				}
			}
		}
	}

	return m, nil
}
