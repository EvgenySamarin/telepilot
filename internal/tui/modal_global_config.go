/*
File: internal/tui/modal_global_config.go
Description: Renderer for the Global Configuration modal managing UI language,
favourite chats, hidden chats, and default skill reset.
*/

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderGlobalConfigModal() string {
	modalW := 90
	if m.width-4 < modalW {
		modalW = m.width - 4
	}
	if modalW < 60 {
		modalW = 60
	}
	innerW := modalW - 6
	cardW := innerW

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("cfg_modal_title")))
	rows = append(rows, "")

	banner := m.T("cfg_modal_banner")
	rows = append(rows, lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(truncateSingleLine(banner, innerW)))
	rows = append(rows, "")

	// -------------------------------------------------------------
	// Block 0: Interface Language
	// -------------------------------------------------------------
	enPrefix := "  ( ) "
	ruPrefix := "  ( ) "
	isEnteredLang := m.cfgBoxCursor == 0 && m.cfgBoxEntered

	var btnEn, btnRu string
	if m.cfgLangIndex == 0 {
		if isEnteredLang {
			enPrefix = "▸ (●) "
			btnEn = lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(enPrefix + "[1] En — English (Default)")
			ruPrefix = "  ( ) "
			btnRu = lipgloss.NewStyle().Foreground(colMuted).Render(ruPrefix + "[2] Ru — Русский (Russian)")
		} else {
			enPrefix = "  (●) "
			btnEn = lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(enPrefix + "[1] En — English (Default)")
			ruPrefix = "  ( ) "
			btnRu = lipgloss.NewStyle().Foreground(colMuted).Render(ruPrefix + "[2] Ru — Русский (Russian)")
		}
	} else {
		if isEnteredLang {
			enPrefix = "  ( ) "
			btnEn = lipgloss.NewStyle().Foreground(colMuted).Render(enPrefix + "[1] En — English (Default)")
			ruPrefix = "▸ (●) "
			btnRu = lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(ruPrefix + "[2] Ru — Русский (Russian)")
		} else {
			enPrefix = "  ( ) "
			btnEn = lipgloss.NewStyle().Foreground(colMuted).Render(enPrefix + "[1] En — English (Default)")
			ruPrefix = "  (●) "
			btnRu = lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(ruPrefix + "[2] Ru — Русский (Russian)")
		}
	}

	langBody := btnEn + "\n" + btnRu
	rows = append(rows, m.renderConfigBox(0, m.T("cfg_modal_lang"), langBody, cardW, colCyan)...)
	rows = append(rows, "")

	// -------------------------------------------------------------
	// Block 1: Favourite / Pinned Chats
	// -------------------------------------------------------------
	var favBody string
	if len(m.favoriteChats) == 0 {
		favBody = lipgloss.NewStyle().Foreground(colMuted).Italic(true).Render(m.T("cfg_modal_favs_empty"))
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
		isEnteredFavs := m.cfgBoxCursor == 1 && m.cfgBoxEntered
		for i := startFav; i < endFav; i++ {
			fav := m.favoriteChats[i]
			isCursor := isEnteredFavs && i == m.cfgFavCursor
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
			innerTextW := cardW - 4
			if lipgloss.Width(rawItem) > innerTextW {
				overflow := lipgloss.Width(rawItem) - innerTextW
				titleRunes := []rune(titleClean)
				if len(titleRunes) > overflow+2 {
					titleClean = string(titleRunes[:len(titleRunes)-overflow-2]) + ".."
				}
				rawItem = fmt.Sprintf("%s📌 %s (ID: %d)%s%s", prefix, titleClean, fav.ChatID, statusStr, actionHint)
			}

			var renderedItem string
			if isCursor {
				if !exists {
					renderedItem = lipgloss.NewStyle().Foreground(colRed).Bold(true).Render(rawItem)
				} else {
					renderedItem = lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(rawItem)
				}
			} else {
				if !exists {
					renderedItem = lipgloss.NewStyle().Foreground(colRed).Render(rawItem)
				} else {
					renderedItem = lipgloss.NewStyle().Foreground(colText).Render(rawItem)
				}
			}
			favRows = append(favRows, renderedItem)
		}
		if len(m.favoriteChats) > maxVisibleFavs {
			scrollInfo := lipgloss.NewStyle().Foreground(colMuted).Render(
				fmt.Sprintf("  [%d/%d favourite chats — use j/k to navigate]", m.cfgFavCursor+1, len(m.favoriteChats)),
			)
			favRows = append(favRows, scrollInfo)
		}
		favBody = strings.Join(favRows, "\n")
	}

	rows = append(rows, m.renderConfigBox(1, m.T("cfg_modal_favs_title", len(m.favoriteChats)), favBody, cardW, colPurple)...)
	rows = append(rows, "")

	// -------------------------------------------------------------
	// Block 2: Hidden Chats
	// -------------------------------------------------------------
	var hiddenBody string
	descLine := lipgloss.NewStyle().Foreground(colMuted).Italic(true).Render(truncateSingleLine(m.T("cfg_modal_hidden_desc"), cardW-4))
	if len(m.hiddenChats) == 0 {
		hiddenBody = descLine + "\n" + lipgloss.NewStyle().Foreground(colMuted).Render(m.T("cfg_modal_hidden_empty"))
	} else {
		maxVisibleHidden := 4
		startHidden := 0
		if m.cfgHiddenCursor >= maxVisibleHidden {
			startHidden = m.cfgHiddenCursor - maxVisibleHidden + 1
		}
		endHidden := minInt(len(m.hiddenChats), startHidden+maxVisibleHidden)

		var hiddenRows []string
		isEnteredHidden := m.cfgBoxCursor == 2 && m.cfgBoxEntered
		for i := startHidden; i < endHidden; i++ {
			hidden := m.hiddenChats[i]
			isCursor := isEnteredHidden && i == m.cfgHiddenCursor
			prefix := "  "
			if isCursor {
				prefix = "▸ "
			}

			actionHint := ""
			if isCursor {
				actionHint = "  [d/x/u: Unhide]"
			}

			titleClean := sanitizeTerminalText(hidden.Title)
			rawItem := fmt.Sprintf("%s👁  %s (ID: %d)%s", prefix, titleClean, hidden.ChatID, actionHint)
			innerTextW := cardW - 4
			if lipgloss.Width(rawItem) > innerTextW {
				overflow := lipgloss.Width(rawItem) - innerTextW
				titleRunes := []rune(titleClean)
				if len(titleRunes) > overflow+2 {
					titleClean = string(titleRunes[:len(titleRunes)-overflow-2]) + ".."
				}
				rawItem = fmt.Sprintf("%s👁  %s (ID: %d)%s", prefix, titleClean, hidden.ChatID, actionHint)
			}

			var renderedItem string
			if isCursor {
				renderedItem = lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(rawItem)
			} else {
				renderedItem = lipgloss.NewStyle().Foreground(colText).Render(rawItem)
			}
			hiddenRows = append(hiddenRows, renderedItem)
		}
		if len(m.hiddenChats) > maxVisibleHidden {
			scrollInfo := lipgloss.NewStyle().Foreground(colMuted).Render(
				fmt.Sprintf("  [%d/%d hidden chats — use j/k to navigate]", m.cfgHiddenCursor+1, len(m.hiddenChats)),
			)
			hiddenRows = append(hiddenRows, scrollInfo)
		}
		hiddenBody = descLine + "\n" + strings.Join(hiddenRows, "\n")
	}

	rows = append(rows, m.renderConfigBox(2, m.T("cfg_modal_hidden_title", len(m.hiddenChats)), hiddenBody, cardW, colYellow)...)
	rows = append(rows, "")

	// -------------------------------------------------------------
	// System Information & Storage Paths Box
	// -------------------------------------------------------------
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
		Width(innerW - 4).
		Render(strings.Join(wrappedInfo, "\n"))

	rows = append(rows, infoBox, "")

	// -------------------------------------------------------------
	// Buttons & Navigation Hint
	// -------------------------------------------------------------
	btnW := (innerW - 4) / 2
	if btnW < 22 {
		btnW = 22
	}
	btnEnter := lipgloss.NewStyle().Background(colBlue).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("cfg_modal_btn_enter"))
	btnSave := lipgloss.NewStyle().Background(colGreen).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("cfg_modal_btn_save"))
	btnReset := lipgloss.NewStyle().Background(colYellow).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("cfg_modal_btn_reset_skills"))
	btnEsc := lipgloss.NewStyle().Background(colBorder).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render(m.T("cfg_modal_btn_cancel"))

	btnRow1 := lipgloss.JoinHorizontal(lipgloss.Center, btnEnter, "  ", btnSave)
	btnRow2 := lipgloss.JoinHorizontal(lipgloss.Center, btnReset, "  ", btnEsc)
	rows = append(rows, btnRow1, "", btnRow2, "")

	var hintText string
	if m.cfgBoxEntered {
		switch m.cfgBoxCursor {
		case 0:
			hintText = m.T("cfg_modal_entered_lang_hint")
		case 1:
			hintText = m.T("cfg_modal_entered_favs_hint")
		case 2:
			hintText = m.T("cfg_modal_entered_hidden_hint")
		}
	} else {
		hintText = m.T("cfg_modal_block_nav_hint")
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
