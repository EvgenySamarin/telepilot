/*
File: internal/tui/view_sidebar.go
Description: Left-hand Chats sidebar renderer with search prompt, pinned/AI badges,
unread indicators, and hidden-chats notice.
*/

package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

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
