/*
File: internal/presentation/tui/keys.go
Description: Keyboard input handlers for chat search, live message composing, modal
dialogs, and main workspace navigation.
*/

package tui

import (
	"fmt"
	"os"
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
				if isLikelyLocalPath(name) {
					srcPath := expandLocalPath(name)
					imported, err := skills.Import(m.cfg.SkillsDir(), srcPath)
					m.activeModal = modalNone
					if err != nil {
						m.statusText = "Skill import error"
						m.addTrace(fmt.Sprintf("Could not import skill from %s: %v", srcPath, err))
					} else if len(imported) > 0 {
						m.reloadSkills()
						for idx, sk := range m.skillList {
							if sk.Name == imported[0].Name || sk.Path == imported[0].Path {
								m.skillCursor = idx
								break
							}
						}
						m.tab2BoxCursor = 3
						m.tab2BoxEntered = true
						m.scrollTab2ToSkill()
						m.statusText = fmt.Sprintf("Imported: %s", imported[0].Name)
						m.addTrace(fmt.Sprintf("Imported %d skill(s) from %s into %s", len(imported), srcPath, m.cfg.SkillsDir()))
					}
					return m, nil
				}
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
			} else if key == "space" || key == " " {
				m.inputBuffer += " "
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
				_ = openFileInViewer(m.qrPNGPath)
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
		case "O":
			m.activeModal = modalNone
			m.openSelectedSkillInMacEditor()
			return m, nil
		case "o":
			m.openSelectedSkillInFinder()
			return m, nil
		}
		return m, nil

	case modalGlobalConfig:
		// Level 2: Inside a box (cfgBoxEntered == true)
		if m.cfgBoxEntered {
			switch key {
			case "esc":
				// Exit back to box selection level (Level 1), exactly like Tab 2!
				m.cfgBoxEntered = false
				return m, nil

			case "ctrl+s":
				// Quick save & exit modal
				m.cfgBoxEntered = false
				m.activeModal = modalNone
				return m, nil

			case "ctrl+r":
				m.resetDefaultSkills()
				return m, nil
			}

			if m.cfgBoxCursor == 0 {
				// Inside Block 0: Interface Language
				switch key {
				case "up", "k", "down", "j", "left", "right", "h", "l", "f2", " ", "tab", "shift+tab":
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

				case "enter":
					// Confirm selection and exit back to block selection level
					m.cfgBoxEntered = false
					return m, nil
				}
				return m, nil
			}

			if m.cfgBoxCursor == 1 {
				// Inside Block 1: Favourite / Pinned Chats
				switch key {
				case "up", "k":
					if m.cfgFavCursor > 0 {
						m.cfgFavCursor--
					}
					return m, nil

				case "down", "j":
					if m.cfgFavCursor < len(m.favoriteChats)-1 {
						m.cfgFavCursor++
					}
					return m, nil

				case "d", "x", "delete", "backspace":
					if m.cfgFavCursor >= 0 && m.cfgFavCursor < len(m.favoriteChats) {
						target := m.favoriteChats[m.cfgFavCursor]
						m.removeFavoriteChat(target.ChatID)
					}
					return m, nil

				case "enter":
					// Enter confirms and exits back to block selection level
					m.cfgBoxEntered = false
					return m, nil
				}
				return m, nil
			}

			if m.cfgBoxCursor == 2 {
				// Inside Block 2: Hidden Chats
				switch key {
				case "up", "k":
					if m.cfgHiddenCursor > 0 {
						m.cfgHiddenCursor--
					}
					return m, nil

				case "down", "j":
					if m.cfgHiddenCursor < len(m.hiddenChats)-1 {
						m.cfgHiddenCursor++
					}
					return m, nil

				case "d", "x", "u", "delete", "backspace", " ":
					if m.cfgHiddenCursor >= 0 && m.cfgHiddenCursor < len(m.hiddenChats) {
						target := m.hiddenChats[m.cfgHiddenCursor]
						m.unhideChat(target.ChatID)
					}
					return m, nil

				case "enter":
					m.cfgBoxEntered = false
					return m, nil
				}
				return m, nil
			}
			return m, nil
		}

		// Level 1: Block selection (cfgBoxEntered == false)
		switch key {
		case "esc", "q":
			// At block level, Esc/q closes the modal window
			m.activeModal = modalNone
			return m, nil

		case "ctrl+r":
			m.resetDefaultSkills()
			return m, nil

		case "ctrl+x":
			stopped := m.cancelAllScheduledTasks()
			if stopped > 0 {
				m.statusText = fmt.Sprintf("Stopped %d scheduled tasks", stopped)
			} else {
				m.statusText = "No scheduled tasks active"
			}
			return m, nil

		case "enter":
			// Drill into the selected block
			m.cfgBoxEntered = true
			return m, nil

		case "up", "k":
			if m.cfgBoxCursor > 0 {
				m.cfgBoxCursor--
			}
			return m, nil

		case "down", "j":
			if m.cfgBoxCursor < 2 {
				m.cfgBoxCursor++
			}
			return m, nil

		case "tab", "shift+tab":
			m.cfgBoxCursor = (m.cfgBoxCursor + 1) % 3
			return m, nil

		case "1", "e", "E":
			// Direct shortcut to switch language
			m.cfgLangIndex = 0
			m.cfg.SaveLanguage("en")
			m.statusText = m.T("status_ready")
			m.addTrace("Interface language switched to English (en). Settings saved to .env.")
			return m, nil

		case "2", "r", "R":
			// Direct shortcut to switch language
			m.cfgLangIndex = 1
			m.cfg.SaveLanguage("ru")
			m.statusText = m.T("status_ready")
			m.addTrace("Язык интерфейса изменен на русский (ru). Настройки сохранены в .env.")
			return m, nil

		case " ":
			// If on Language block, space can toggle
			if m.cfgBoxCursor == 0 {
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
			}
			return m, nil

		case "ctrl+s":
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
		m.marqueeTick = 0
		m.syncSkillsFromDisk()
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

	case "c", "y":
		if m.activeTab == 4 {
			logPath := m.traceLogFilePath()
			if data, err := os.ReadFile(logPath); err == nil && len(data) > 0 {
				_ = copyToClipboard(string(data))
				m.statusText = "Trace log copied to clipboard"
				m.addTrace("Copied trace log to clipboard")
			} else if len(m.traceLogs) > 0 {
				_ = copyToClipboard(strings.Join(m.traceLogs, "\n"))
				m.statusText = "Trace log copied to clipboard"
				m.addTrace("Copied trace log to clipboard")
			}
			return m, nil
		}
		if msg.String() == "c" {
			m.aiFormFields[0] = m.cfg.AgyCLIBin()
			m.aiFormFields[1] = m.cfg.AgyModel
			if strings.TrimSpace(m.aiFormFields[1]) == "" {
				m.aiFormFields[1] = "auto"
			}
			m.aiFormFocus = 0
			m.activeModal = modalAIConfig
		}

	case "C":
		m.cfgBoxCursor = 0
		m.cfgBoxEntered = false
		m.cfgLangIndex = 0
		if m.cfg.IsRussian() {
			m.cfgLangIndex = 1
		}
		m.cfgFavCursor = 0
		m.activeModal = modalGlobalConfig

	case "p", "f":
		targetChatID := m.selectedChatID
		if m.focus == focusChats && len(m.chats) > 0 {
			matches := m.filteredChatIndices()
			if len(matches) > 0 {
				pos := m.filteredChatCursorPos(matches)
				if pos >= 0 && pos < len(matches) {
					targetChatID = m.chats[matches[pos]].ChatID
				}
			} else if m.chatCursor >= 0 && m.chatCursor < len(m.chats) {
				targetChatID = m.chats[m.chatCursor].ChatID
			}
		}
		if targetChatID != 0 {
			m.togglePinChat(targetChatID)
			return m, nil
		}

	case "H":
		targetChatID := m.selectedChatID
		if m.focus == focusChats && len(m.chats) > 0 {
			matches := m.filteredChatIndices()
			if len(matches) > 0 {
				pos := m.filteredChatCursorPos(matches)
				if pos >= 0 && pos < len(matches) {
					targetChatID = m.chats[matches[pos]].ChatID
				}
			} else if m.chatCursor >= 0 && m.chatCursor < len(m.chats) {
				targetChatID = m.chats[m.chatCursor].ChatID
			}
		}
		if targetChatID != 0 {
			m.hideChat(targetChatID)
			return m, nil
		}

	case "?":
		m.activeModal = modalHelp

	case "i":
		if m.activeTab == 1 {
			m.focus = focusMain
			m.chatComposing = true
			return m, nil
		}

	case "[":
		if m.mainScroll > 0 {
			m.mainScroll--
		}

	case "]":
		m.mainScroll++

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
					m.marqueeTick = 0
					m.scrollTab2ToBox(3)
				} else if m.tab2BoxCursor == 1 {
					m.tab2ChatOptCursor = 0
				}
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
					m.marqueeTick = 0
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
						m.marqueeTick = 0
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
						m.marqueeTick = 0
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
		if m.focus == focusChats {
			targetChatID := m.selectedChatID
			if len(m.chats) > 0 {
				matches := m.filteredChatIndices()
				if len(matches) > 0 {
					pos := m.filteredChatCursorPos(matches)
					if pos >= 0 && pos < len(matches) {
						targetChatID = m.chats[matches[pos]].ChatID
					}
				} else if m.chatCursor >= 0 && m.chatCursor < len(m.chats) {
					targetChatID = m.chats[m.chatCursor].ChatID
				}
			}
			if targetChatID != 0 {
				m.hideChat(targetChatID)
				return m, nil
			}
		} else if m.activeTab == 2 {
			ch := m.activeChat()
			if ch != nil {
				m.cancelSingleShotForChat(ch.ChatID, ch.Title)
				m.saveCurrentChatAIConfig("", "", m.periodSpec, false)
				m.addTrace(fmt.Sprintf("Disabled per-chat AI configuration for '%s'", ch.Title))
			}
		}

	case "ctrl+l":
		if m.activeTab == 4 {
			m.traceLogs = nil
			_ = os.WriteFile(m.traceLogFilePath(), []byte{}, 0o644)
			m.statusText = "Trace log cleared"
			m.addTrace("Trace logs cleared by user")
			return m, nil
		}

	case "ctrl+x", "X":
		if m.activeTab == 2 || m.activeTab == 4 {
			stopped := m.cancelAllScheduledTasks()
			if stopped > 0 {
				m.statusText = fmt.Sprintf("Stopped %d scheduled tasks", stopped)
			} else {
				m.statusText = "No scheduled tasks active"
			}
			return m, nil
		}
		if ch := m.activeChat(); ch != nil {
			stopped := m.cancelScheduledTasksForChat(ch.ChatID, ch.Title)
			if stopped > 0 {
				m.statusText = fmt.Sprintf("Stopped %d scheduled tasks for '%s'", stopped, ch.Title)
			} else {
				m.statusText = "No scheduled tasks for this chat"
			}
			return m, nil
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
					m.marqueeTick = 0
					m.scrollTab2ToSkill()
				case 4:
					if m.activeSkillOutputRecord() != nil {
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
					m.marqueeTick = 0
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

	case "I":
		if m.activeTab == 2 {
			return m, m.importSkillFromMacPickerCmd()
		}

	case "O":
		if m.activeTab == 2 {
			m.openSelectedSkillInMacEditor()
			return m, nil
		}

	case "o":
		if m.activeTab == 1 {
			ch := m.activeChat()
			if ch != nil {
				if m.liveMsgCursor >= 0 && m.liveMsgCursor < len(m.liveMessages) {
					lm := m.liveMessages[m.liveMsgCursor]
					if lm.MediaType != "" {
						if lm.CachedPath != "" {
							if _, err := os.Stat(lm.CachedPath); err == nil {
								if isMacOS() {
									_ = exec.Command("open", "-R", lm.CachedPath).Start()
								}
								m.statusText = "Opened in Finder"
								m.addTrace("Revealed cached media in Finder: " + lm.CachedPath)
								return m, nil
							}
						}
						return m, m.downloadAndRevealLiveMediaInFinderCmd(*ch, lm)
					}
				}
				cacheDir := filepath.Join(m.cfg.DataDir, "live_cache", fmt.Sprintf("%d", ch.ChatID))
				if _, err := os.Stat(cacheDir); err == nil {
					if isMacOS() {
						_ = exec.Command("open", cacheDir).Start()
					}
					m.statusText = "Opened in Finder"
					m.addTrace("Opened chat media cache folder in Finder: " + cacheDir)
					return m, nil
				}
			}
			return m, nil
		}
		if m.activeTab == 2 {
			m.openSelectedSkillInFinder()
			return m, nil
		}
		if m.activeTab == 3 {
			rec := m.activeSkillOutputRecord()
			if rec != nil && rec.OutputPath != "" {
				if isMacOS() {
					_ = exec.Command("open", "-R", rec.OutputPath).Start()
				}
				m.statusText = "Opened in Finder"
				m.addTrace("Revealed skill output in Finder: " + rec.OutputPath)
			}
			return m, nil
		}
		if m.activeTab == 4 {
			logPath := m.traceLogFilePath()
			if isMacOS() {
				_ = exec.Command("open", "-R", logPath).Start()
			}
			m.statusText = "Revealed trace.log in Finder"
			m.addTrace("Revealed trace log in Finder: " + logPath)
			return m, nil
		}

	case "v":
		switch m.activeTab {
		case 1:
			ch := m.activeChat()
			if ch != nil && m.liveMsgCursor >= 0 && m.liveMsgCursor < len(m.liveMessages) {
				lm := m.liveMessages[m.liveMsgCursor]
				if lm.MediaType != "" {
					if lm.CachedPath != "" {
						if _, err := os.Stat(lm.CachedPath); err == nil {
							if lm.MediaType == "voice" {
								_ = playVoiceAudio(lm.CachedPath)
								m.addTrace("Playing live voice: " + lm.CachedPath)
							} else {
								_ = openFileInViewer(lm.CachedPath)
								m.addTrace("Opened live media in default viewer: " + lm.CachedPath)
							}
							return m, nil
						}
					}
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
			rec := m.activeSkillOutputRecord()
			if rec != nil && rec.OutputPath != "" {
				_ = openFileInViewer(rec.OutputPath)
				m.addTrace("Opened skill output in viewer: " + rec.OutputPath)
			}
		case 4:
			logPath := m.traceLogFilePath()
			_ = openFileInViewer(logPath)
			m.statusText = "Opened trace.log in viewer"
			m.addTrace("Opened trace log in viewer: " + logPath)
		}
	}

	return m, nil
}
