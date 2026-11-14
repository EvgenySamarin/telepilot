package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
