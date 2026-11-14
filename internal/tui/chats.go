/*
File: internal/tui/chats.go
Description: Chat list management including active chat lookup, selection, pinning
favourites, hiding/unhiding, and search filtering.
*/

package tui

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"telepilot/internal/storage"
)

func (m *Model) activeChat() *storage.ChatInfo {
	for i := range m.chats {
		if m.chats[i].ChatID == m.selectedChatID {
			return &m.chats[i]
		}
	}
	if len(m.chats) > 0 {
		if m.chatCursor >= 0 && m.chatCursor < len(m.chats) {
			return &m.chats[m.chatCursor]
		}
		return &m.chats[0]
	}
	return nil
}

func reorderChatsWithPinnedList(chats []storage.ChatInfo, favIDs map[int64]bool, favList []storage.FavoriteChatRecord) []storage.ChatInfo {
	if len(chats) == 0 || len(favIDs) == 0 {
		return chats
	}
	byID := make(map[int64]storage.ChatInfo, len(chats))
	for _, c := range chats {
		byID[c.ChatID] = c
	}
	var pinned []storage.ChatInfo
	seen := make(map[int64]bool, len(chats))
	for _, fav := range favList {
		if c, ok := byID[fav.ChatID]; ok {
			pinned = append(pinned, c)
			seen[fav.ChatID] = true
		}
	}
	for _, c := range chats {
		if favIDs[c.ChatID] && !seen[c.ChatID] {
			pinned = append(pinned, c)
			seen[c.ChatID] = true
		}
	}
	var unpinned []storage.ChatInfo
	for _, c := range chats {
		if !seen[c.ChatID] {
			unpinned = append(unpinned, c)
		}
	}
	return append(pinned, unpinned...)
}

func (m *Model) isChatPinned(chatID int64) bool {
	if m.favoriteChatIDs == nil {
		return false
	}
	return m.favoriteChatIDs[chatID]
}

func (m *Model) reorderChatsWithPinned(chats []storage.ChatInfo) []storage.ChatInfo {
	return reorderChatsWithPinnedList(chats, m.favoriteChatIDs, m.favoriteChats)
}

func (m *Model) togglePinChat(chatID int64) {
	if chatID == 0 {
		return
	}
	if m.favoriteChatIDs == nil {
		m.favoriteChatIDs = make(map[int64]bool)
	}

	if m.favoriteChatIDs[chatID] {
		// Unpin
		delete(m.favoriteChatIDs, chatID)
		var updated []storage.FavoriteChatRecord
		var chatTitle string
		for _, f := range m.favoriteChats {
			if f.ChatID == chatID {
				chatTitle = f.Title
			} else {
				updated = append(updated, f)
			}
		}
		m.favoriteChats = updated
		_ = storage.SaveFavoriteChats(m.cfg.DataDir, m.favoriteChats)
		m.chats = m.reorderChatsWithPinned(m.chats)
		for i, c := range m.chats {
			if c.ChatID == chatID {
				m.chatCursor = i
				break
			}
		}
		if chatTitle == "" {
			chatTitle = fmt.Sprintf("ID %d", chatID)
		}
		m.statusText = fmt.Sprintf("Unpinned chat '%s'", chatTitle)
		m.addTrace(fmt.Sprintf("Unpinned chat '%s' (id=%d)", chatTitle, chatID))
	} else {
		// Pin
		var title string
		var username string
		var chatType string
		for _, c := range m.chats {
			if c.ChatID == chatID {
				title = c.Title
				username = c.Username
				chatType = c.ChatType
				break
			}
		}
		if title == "" {
			title = fmt.Sprintf("Chat %d", chatID)
		}
		rec := storage.FavoriteChatRecord{
			ChatID:   chatID,
			Title:    title,
			Username: username,
			ChatType: chatType,
			PinnedAt: time.Now().Format("2006-01-02 15:04:05"),
		}
		m.favoriteChats = append(m.favoriteChats, rec)
		m.favoriteChatIDs[chatID] = true
		_ = storage.SaveFavoriteChats(m.cfg.DataDir, m.favoriteChats)
		m.chats = m.reorderChatsWithPinned(m.chats)
		for i, c := range m.chats {
			if c.ChatID == chatID {
				m.chatCursor = i
				break
			}
		}
		m.statusText = fmt.Sprintf("Pinned chat '%s' to top", title)
		m.addTrace(fmt.Sprintf("Pinned chat '%s' (id=%d) to top", title, chatID))
	}
}

func (m *Model) removeFavoriteChat(chatID int64) {
	if chatID == 0 {
		return
	}
	if m.favoriteChatIDs != nil {
		delete(m.favoriteChatIDs, chatID)
	}
	var updated []storage.FavoriteChatRecord
	var chatTitle string
	for _, f := range m.favoriteChats {
		if f.ChatID == chatID {
			chatTitle = f.Title
		} else {
			updated = append(updated, f)
		}
	}
	m.favoriteChats = updated
	_ = storage.SaveFavoriteChats(m.cfg.DataDir, m.favoriteChats)
	m.chats = m.reorderChatsWithPinned(m.chats)
	if m.cfgFavCursor >= len(m.favoriteChats) {
		m.cfgFavCursor = maxInt(0, len(m.favoriteChats)-1)
	}
	if len(m.favoriteChats) == 0 {
		m.cfgFavCursor = 0
	}
	if chatTitle == "" {
		chatTitle = fmt.Sprintf("ID %d", chatID)
	}
	m.statusText = fmt.Sprintf("Removed '%s' from favourites", chatTitle)
	m.addTrace(fmt.Sprintf("Removed chat '%s' (id=%d) from favourite chats list", chatTitle, chatID))
}

func (m *Model) isChatHidden(chatID int64) bool {
	m.hiddenMu.RLock()
	defer m.hiddenMu.RUnlock()
	if m.hiddenChatIDs == nil {
		return false
	}
	return m.hiddenChatIDs[chatID]
}

func (m *Model) refreshVisibleChats() {
	var visible []storage.ChatInfo
	for _, c := range m.rawChats {
		if !m.isChatHidden(c.ChatID) {
			visible = append(visible, c)
		}
	}
	m.chats = m.reorderChatsWithPinned(visible)
	if m.chatCursor >= len(m.chats) {
		m.chatCursor = maxInt(0, len(m.chats)-1)
	}
	if len(m.chats) > 0 {
		found := false
		for _, c := range m.chats {
			if c.ChatID == m.selectedChatID {
				found = true
				break
			}
		}
		if !found {
			m.selectedChatID = m.chats[m.chatCursor].ChatID
			m.syncCurrentChatAISettings()
		}
	} else {
		m.selectedChatID = 0
	}
}

func (m *Model) hideChat(chatID int64) {
	if chatID == 0 {
		return
	}
	if m.isChatPinned(chatID) {
		m.removeFavoriteChat(chatID)
	}

	m.hiddenMu.Lock()
	if m.hiddenChatIDs == nil {
		m.hiddenChatIDs = make(map[int64]bool)
	}
	m.hiddenChatIDs[chatID] = true
	m.hiddenMu.Unlock()

	var title, username, chatType string
	for _, c := range m.rawChats {
		if c.ChatID == chatID {
			title = c.Title
			username = c.Username
			chatType = c.ChatType
			break
		}
	}
	if title == "" {
		for _, c := range m.chats {
			if c.ChatID == chatID {
				title = c.Title
				username = c.Username
				chatType = c.ChatType
				break
			}
		}
	}
	if title == "" {
		title = fmt.Sprintf("Chat %d", chatID)
	}

	exists := false
	for _, h := range m.hiddenChats {
		if h.ChatID == chatID {
			exists = true
			break
		}
	}
	if !exists {
		rec := storage.HiddenChatRecord{
			ChatID:   chatID,
			Title:    title,
			Username: username,
			ChatType: chatType,
			HiddenAt: time.Now().Format("2006-01-02 15:04:05"),
		}
		m.hiddenChats = append(m.hiddenChats, rec)
	}

	_ = storage.SaveHiddenChats(m.cfg.DataDir, m.hiddenChats)
	m.refreshVisibleChats()
	m.statusText = fmt.Sprintf("Chat '%s' hidden", title)
	m.addTrace(fmt.Sprintf("Hidden chat '%s' (id=%d)", title, chatID))
}

func (m *Model) unhideChat(chatID int64) {
	if chatID == 0 {
		return
	}
	m.hiddenMu.Lock()
	if m.hiddenChatIDs != nil {
		delete(m.hiddenChatIDs, chatID)
	}
	m.hiddenMu.Unlock()

	var updated []storage.HiddenChatRecord
	var chatTitle string
	for _, h := range m.hiddenChats {
		if h.ChatID == chatID {
			chatTitle = h.Title
		} else {
			updated = append(updated, h)
		}
	}
	m.hiddenChats = updated
	_ = storage.SaveHiddenChats(m.cfg.DataDir, m.hiddenChats)

	if m.cfgHiddenCursor >= len(m.hiddenChats) {
		m.cfgHiddenCursor = maxInt(0, len(m.hiddenChats)-1)
	}
	if len(m.hiddenChats) == 0 {
		m.cfgHiddenCursor = 0
	}

	m.refreshVisibleChats()
	if chatTitle == "" {
		chatTitle = fmt.Sprintf("ID %d", chatID)
	}
	m.statusText = fmt.Sprintf("Restored chat '%s'", chatTitle)
	m.addTrace(fmt.Sprintf("Restored hidden chat '%s' (id=%d)", chatTitle, chatID))
}

func (m *Model) selectChatAtIndex(idx int) tea.Cmd {
	if idx < 0 || idx >= len(m.chats) {
		return nil
	}
	m.chatCursor = idx
	ch := m.chats[idx]
	m.selectedChatID = ch.ChatID
	m.chatComposing = false
	m.chatInputBuffer = ""
	m.mainScroll = 0
	m.syncCurrentChatAISettings()
	m.liveMessages = storage.DemoLiveMessages(ch)
	m.liveMsgCursor = maxInt(0, len(m.liveMessages)-1)
	m.addTrace(fmt.Sprintf("Selected chat '%s' (id=%d)", ch.Title, ch.ChatID))
	return m.loadChatMessagesCmd(ch)
}

func (m *Model) filteredChatIndices() []int {
	q := strings.ToLower(strings.TrimSpace(m.chatSearchQuery))
	indices := make([]int, 0, len(m.chats))
	for i, c := range m.chats {
		if q == "" {
			indices = append(indices, i)
			continue
		}
		if strings.Contains(strings.ToLower(c.Title), q) ||
			strings.Contains(strings.ToLower(c.Username), q) ||
			strings.Contains(strings.ToLower(c.ChatType), q) ||
			((q == "ai" || q == "[ai]") && m.isChatAIEnabled(c.ChatID)) {
			indices = append(indices, i)
		}
	}
	return indices
}

func (m *Model) filteredChatCursorPos(matches []int) int {
	for pos, chatIdx := range matches {
		if chatIdx == m.chatCursor {
			return pos
		}
	}
	return 0
}
