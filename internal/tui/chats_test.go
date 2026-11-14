/*
File: internal/tui/chats_test.go
Description: TUI unit tests for chat selection, live compose/send, vim search, Finder
media reveal, favourite pinning, and hidden chats.
*/

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"telepilot/internal/config"
	"telepilot/internal/storage"
)

func TestSingleChatSelectionAndPerChatAIConfig(t *testing.T) {
	m := newTestModel(t, "en")

	if len(m.chats) < 2 {
		t.Fatalf("expected at least 2 demo chats, got %d", len(m.chats))
	}
	firstChatID := m.chats[0].ChatID
	secondChatID := m.chats[1].ChatID

	// 1. Initially only the first chat is selected, and NO chat has AI enabled by default
	if m.selectedChatID != firstChatID {
		t.Fatalf("expected initial selectedChatID=%d, got %d", firstChatID, m.selectedChatID)
	}
	for _, c := range m.chats {
		if m.isChatAIEnabled(c.ChatID) {
			t.Errorf("expected no chat to have AI enabled by default, but chat %d has AI enabled", c.ChatID)
		}
	}

	// 2. Move cursor down in Chats sidebar and press Space to select second chat
	m.focus = focusChats
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mUpdated.(*Model)

	if m.selectedChatID != secondChatID {
		t.Fatalf("expected selectedChatID to switch to second chat (%d), got %d", secondChatID, m.selectedChatID)
	}

	// 3. Switch to Tab 2 (AI & Skills), navigate down to Box 3 (Skills Library), press Enter to open it, then Enter to assign skill
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
	m = mUpdated.(*Model)
	m.focus = focusMain
	if m.tab2BoxEntered {
		t.Fatalf("expected Tab 2 to start at box-level navigation (tab2BoxEntered=false)")
	}
	// Navigate down 3 boxes from Box 0 (Shortcuts) -> Box 1 (Per-Chat) -> Box 2 (Common Engine) -> Box 3 (Skills Library)
	for i := 0; i < 3; i++ {
		mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = mUpdated.(*Model)
	}
	if m.tab2BoxCursor != 3 {
		t.Fatalf("expected tab2BoxCursor=3 (Skills Library), got %d", m.tab2BoxCursor)
	}
	// Press Enter once to enter Box 3 (Skills Library)
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mUpdated.(*Model)
	if !m.tab2BoxEntered {
		t.Fatalf("expected pressing Enter on Box 3 to enter Skills Library (tab2BoxEntered=true)")
	}
	// Press Enter inside Skills Library to assign the selected summary skill to the second chat
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mUpdated.(*Model)
	// Press Esc to exit back to box-level navigation
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mUpdated.(*Model)
	if m.tab2BoxEntered {
		t.Fatalf("expected pressing Esc inside Box 3 to exit back to box-level navigation")
	}

	if !m.isChatAIEnabled(secondChatID) {
		t.Errorf("expected AI to be enabled for second chat (%d) after pressing Enter in Tab 2", secondChatID)
	}
	if m.isChatAIEnabled(firstChatID) {
		t.Errorf("expected first chat (%d) to still have AI disabled", firstChatID)
	}

	// Verify sidebar renders [AI] badge for the configured chat
	sidebar := m.renderSidebar(38, 30)
	if !strings.Contains(sidebar, "[AI]") {
		t.Errorf("expected sidebar to highlight AI-enabled chat with [AI] badge, got:\n%s", sidebar)
	}

	// Verify persistence to disk (data/chat_ai_config.json)
	loadedCfgs := storage.LoadChatAIConfigs(m.cfg.DataDir)
	if !loadedCfgs[secondChatID].Enabled {
		t.Errorf("expected chat AI config for %d to be persisted in data/chat_ai_config.json", secondChatID)
	}

	// 4. Press 'x' in Tab 2 to disable AI for the current chat
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = mUpdated.(*Model)
	if m.isChatAIEnabled(secondChatID) {
		t.Errorf("expected AI to be disabled for chat %d after pressing 'x' in Tab 2", secondChatID)
	}
	sidebarAfter := m.renderSidebar(38, 30)
	if strings.Contains(sidebarAfter, "[AI]") {
		t.Errorf("expected [AI] badge to be removed from sidebar after disabling AI, got:\n%s", sidebarAfter)
	}
}

func TestLiveChatComposeAndSend(t *testing.T) {
	m := newTestModel(t, "en")
	initialCount := len(m.liveMessages)

	// Switch focus to Tab 1 and press 'i' to start composing
	m.activeTab = 1
	m.focus = focusMain
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = mUpdated.(*Model)
	if !m.chatComposing {
		t.Fatalf("expected 'i' in Tab 1 to enter chatComposing mode")
	}

	// Type "Hello team" and press Enter
	for _, r := range "Hello team" {
		mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mUpdated.(*Model)
	}
	if m.chatInputBuffer != "Hello team" {
		t.Fatalf("expected chatInputBuffer to be 'Hello team', got %q", m.chatInputBuffer)
	}

	mUpdated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mUpdated.(*Model)
	if m.chatComposing {
		t.Errorf("expected Enter to exit chatComposing mode and send message")
	}
	if cmd == nil {
		t.Fatalf("expected sendLiveMessageCmd to be returned on Enter")
	}
	sentMsg := cmd()
	mUpdated, _ = m.Update(sentMsg)
	m = mUpdated.(*Model)

	if len(m.liveMessages) != initialCount+1 {
		t.Fatalf("expected liveMessages count to increase to %d, got %d", initialCount+1, len(m.liveMessages))
	}
	last := m.liveMessages[len(m.liveMessages)-1]
	if last.Text != "Hello team" || !last.IsOutgoing {
		t.Errorf("unexpected sent message at bottom of live chat: %+v", last)
	}
}

func TestChatsWindowVimSearchBackslash(t *testing.T) {
	m := newTestModel(t, "en")
	if len(m.chats) < 2 {
		t.Fatalf("expected at least 2 demo chats, got %d", len(m.chats))
	}

	targetChat := m.chats[1]
	// Pick a unique substring from the second chat's title
	words := strings.Fields(targetChat.Title)
	query := strings.ToLower(words[0])

	// 1. Start in focusMain and press '\' (backslash) -> should focus Chats window and enter chatSearching mode
	m.focus = focusMain
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'\\'}})
	m = mUpdated.(*Model)
	if !m.chatSearching {
		t.Fatalf("expected pressing '\\' to enter chatSearching mode")
	}
	if m.focus != focusChats {
		t.Fatalf("expected pressing '\\' to switch focus to focusChats, got %v", m.focus)
	}

	// 2. Type query characters and verify live filtering
	for _, r := range query {
		mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		m = mUpdated.(*Model)
	}
	if m.chatSearchQuery != query {
		t.Fatalf("expected chatSearchQuery=%q, got %q", query, m.chatSearchQuery)
	}
	matches := m.filteredChatIndices()
	if len(matches) == 0 {
		t.Fatalf("expected at least 1 matching chat for query %q", query)
	}

	// Verify sidebar renders the active search prompt and never exceeds terminal height
	viewWhileSearching := m.View()
	if len(strings.Split(viewWhileSearching, "\n")) != m.height {
		t.Errorf("expected View() height=%d while searching, got %d", m.height, len(strings.Split(viewWhileSearching, "\n")))
	}
	if !strings.Contains(viewWhileSearching, "\\ "+query) {
		t.Errorf("expected sidebar to display search prompt '\\ %s'", query)
	}

	// 3. Press Enter to confirm search and open the matched chat
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mUpdated.(*Model)
	if m.chatSearching {
		t.Errorf("expected Enter to exit chatSearching input mode")
	}
	if m.selectedChatID != m.chats[matches[0]].ChatID {
		t.Errorf("expected Enter in chat search to select matched chat %d, got %d", m.chats[matches[0]].ChatID, m.selectedChatID)
	}
	if m.chatSearchQuery != query {
		t.Errorf("expected search filter %q to remain active until Esc", query)
	}

	// 4. Press Esc to clear the active search filter and restore all chats
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mUpdated.(*Model)
	if m.chatSearchQuery != "" {
		t.Errorf("expected Esc to clear chatSearchQuery, got %q", m.chatSearchQuery)
	}
	if len(m.filteredChatIndices()) != len(m.chats) {
		t.Errorf("expected all %d chats to be visible after clearing search", len(m.chats))
	}
}

func TestTab1OpenCachedMediaInFinder(t *testing.T) {
	tmpDir := t.TempDir()
	cachedFile := filepath.Join(tmpDir, "test_cached_img.jpg")
	if err := os.WriteFile(cachedFile, []byte("fake-image"), 0o644); err != nil {
		t.Fatalf("failed to create fake cached file: %v", err)
	}

	m := newTestModel(t, "en")
	m.activeTab = 1
	m.liveMessages = []storage.LiveMessage{
		{
			MessageID:     501,
			MediaType:     "image",
			MediaFileName: "test_cached_img.jpg",
			CachedPath:    cachedFile,
			Text:          "Here is the screenshot",
		},
		{
			MessageID:     502,
			MediaType:     "document",
			MediaFileName: "spec.pdf",
			Text:          "Specification file",
		},
	}
	m.liveMsgCursor = 0

	// 1. Check hotkey bar in Tab 1 contains "o" Finder key
	bottomBar := m.renderBottomBar()
	if !strings.Contains(bottomBar, "Finder") && !strings.Contains(bottomBar, "o:") {
		t.Errorf("expected Tab 1 bottomBar to contain Finder hotkey, got:\n%s", bottomBar)
	}

	// 2. Press "o" on cached media
	mUpdated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = mUpdated.(*Model)
	if cmd != nil {
		t.Errorf("expected immediate local action without async command for already cached file")
	}
	if m.statusText != "Opened in Finder" {
		t.Errorf("expected statusText 'Opened in Finder', got %q", m.statusText)
	}
	foundTrace := false
	for _, tr := range m.traceLogs {
		if strings.Contains(tr, "Revealed cached media in Finder") && strings.Contains(tr, cachedFile) {
			foundTrace = true
			break
		}
	}
	if !foundTrace {
		t.Errorf("expected trace log to record opening in Finder, traces=%v", m.traceLogs)
	}

	// 3. Move cursor to un-cached media (cursor=1) and press "o" -> should return download command
	m.liveMsgCursor = 1
	mUpdated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'o'}})
	m = mUpdated.(*Model)
	if cmd == nil {
		t.Errorf("expected async downloadAndRevealLiveMediaInFinderCmd for un-cached media")
	}
	if !strings.Contains(m.statusText, "Finder") && !strings.Contains(m.statusText, "Downloading") {
		t.Errorf("expected downloading status, got %q", m.statusText)
	}

	// 4. Simulate liveMediaDownloadedMsg with revealInFinder=true
	downloadedMsg := liveMediaDownloadedMsg{
		chatID:         m.selectedChatID,
		messageID:      502,
		mediaType:      "document",
		filePath:       cachedFile,
		revealInFinder: true,
		err:            nil,
	}
	mUpdated, _ = m.Update(downloadedMsg)
	m = mUpdated.(*Model)
	if m.liveMessages[1].CachedPath != cachedFile {
		t.Errorf("expected message CachedPath to be updated to %s, got %s", cachedFile, m.liveMessages[1].CachedPath)
	}
	if m.statusText != "Opened in Finder" {
		t.Errorf("expected statusText 'Opened in Finder', got %q", m.statusText)
	}
}

func TestFavoriteChatsPinningAndPersistence(t *testing.T) {
	m := newTestModel(t, "en")
	if len(m.chats) < 3 {
		t.Fatalf("expected at least 3 chats, got %d", len(m.chats))
	}

	// 1. Initial state: no pinned chats
	if len(m.favoriteChats) != 0 {
		t.Fatalf("expected 0 initial favorite chats, got %d", len(m.favoriteChats))
	}

	// Highlight chat at index 2
	m.chatCursor = 2
	targetChat := m.chats[2]
	targetID := targetChat.ChatID
	targetTitle := targetChat.Title

	// 2. Press 'p' in focusChats -> pins chat and moves it to top
	m.focus = focusChats
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = mUpdated.(*Model)

	if !m.isChatPinned(targetID) {
		t.Errorf("expected chat %d to be pinned", targetID)
	}
	if len(m.favoriteChats) != 1 {
		t.Fatalf("expected 1 favorite chat, got %d", len(m.favoriteChats))
	}
	if m.chats[0].ChatID != targetID {
		t.Errorf("expected pinned chat %d to be at index 0, got %d", targetID, m.chats[0].ChatID)
	}
	if m.chatCursor != 0 {
		t.Errorf("expected chatCursor to follow pinned chat to index 0, got %d", m.chatCursor)
	}

	// Verify sidebar rendering contains pin icon
	view := m.View()
	if !strings.Contains(view, "📌") {
		t.Errorf("expected sidebar view to render pin icon 📌, got:\n%s", view)
	}
	if !strings.Contains(view, targetTitle) {
		t.Errorf("expected sidebar view to contain chat title %q", targetTitle)
	}

	// Verify persistence in dataDir
	saved := storage.LoadFavoriteChats(m.cfg.DataDir)
	if len(saved) != 1 || saved[0].ChatID != targetID {
		t.Errorf("expected persisted favorite chat %d, got %v", targetID, saved)
	}

	// 3. Pin a second chat (the one now at index 2)
	m.chatCursor = 2
	secondChat := m.chats[2]
	secondID := secondChat.ChatID

	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
	m = mUpdated.(*Model)

	if !m.isChatPinned(secondID) {
		t.Errorf("expected second chat %d to be pinned", secondID)
	}
	if len(m.favoriteChats) != 2 {
		t.Fatalf("expected 2 favorite chats, got %d", len(m.favoriteChats))
	}
	// Both targetID and secondID must be at indices 0 and 1
	if m.chats[0].ChatID != targetID || m.chats[1].ChatID != secondID {
		t.Errorf("expected pinned chats at top [0=%d, 1=%d], got [0=%d, 1=%d]",
			targetID, secondID, m.chats[0].ChatID, m.chats[1].ChatID)
	}

	// 4. Simulate dialogsLoadedMsg from Telegram MTProto: pinned chats must stay on top!
	freshTelegramDialogs := []storage.ChatInfo{
		{ChatID: 11111, Title: "Random Telegram Group", ChatType: "group"},
		{ChatID: secondID, Title: secondChat.Title, ChatType: secondChat.ChatType},
		{ChatID: 22222, Title: "Another Dialog", ChatType: "dm"},
		{ChatID: targetID, Title: targetTitle, ChatType: targetChat.ChatType},
	}
	mUpdated, _ = m.Update(dialogsLoadedMsg{chats: freshTelegramDialogs})
	m = mUpdated.(*Model)

	if m.chats[0].ChatID != targetID || m.chats[1].ChatID != secondID {
		t.Errorf("expected pinned chats to remain on top after live dialogs load: got [0=%d, 1=%d]",
			m.chats[0].ChatID, m.chats[1].ChatID)
	}

	// 5. Unpin first chat using 'p'
	m.chatCursor = 0
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}})
	m = mUpdated.(*Model)

	if m.isChatPinned(targetID) {
		t.Errorf("expected chat %d to be unpinned", targetID)
	}
	if len(m.favoriteChats) != 1 {
		t.Errorf("expected 1 favorite chat remaining, got %d", len(m.favoriteChats))
	}
	savedAfterUnpin := storage.LoadFavoriteChats(m.cfg.DataDir)
	if len(savedAfterUnpin) != 1 || savedAfterUnpin[0].ChatID != secondID {
		t.Errorf("expected only chat %d persisted, got %v", secondID, savedAfterUnpin)
	}
}

func TestFavoriteChatsConfigWindowManagementAndDeletedCornerCases(t *testing.T) {
	m := newTestModel(t, "en")
	if len(m.chats) == 0 {
		t.Fatalf("expected non-empty chats")
	}

	// Pin active chat (Chat A)
	activeChatID := m.chats[0].ChatID
	m.togglePinChat(activeChatID)

	// Corner case: A chat was pinned earlier, but was subsequently deleted from another Telegram client.
	// Telegram will no longer return it in dialogs, so it is missing from m.chats.
	deletedChatID := int64(888777666)
	deletedRecord := storage.FavoriteChatRecord{
		ChatID:   deletedChatID,
		Title:    "Obsolete Project (Deleted on Mobile)",
		ChatType: "group",
		PinnedAt: "2026-11-01 10:00:00",
	}
	m.favoriteChats = append(m.favoriteChats, deletedRecord)
	m.favoriteChatIDs[deletedChatID] = true
	_ = storage.SaveFavoriteChats(m.cfg.DataDir, m.favoriteChats)

	if len(m.favoriteChats) != 2 {
		t.Fatalf("expected 2 favorite chats, got %d", len(m.favoriteChats))
	}

	// 1. Open Configuration Window with capital 'C'
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = mUpdated.(*Model)

	if m.activeModal != modalGlobalConfig {
		t.Fatalf("expected modalGlobalConfig, got %v", m.activeModal)
	}
	if m.cfgBoxCursor != 0 {
		t.Errorf("expected initial cfgBoxCursor to be 0 (Language block), got %d", m.cfgBoxCursor)
	}
	if m.cfgBoxEntered {
		t.Errorf("expected cfgBoxEntered to be false initially")
	}

	// 2. Render modal: must display deleted corner-case chat and [Deleted / Remote] indicator
	modalView := m.renderGlobalConfigModal()
	if !strings.Contains(modalView, "Obsolete Project (Deleted on Mobile)") {
		t.Errorf("expected modal to show deleted chat title, got:\n%s", modalView)
	}
	if !strings.Contains(modalView, "[Deleted / Remote]") {
		t.Errorf("expected modal to show [Deleted / Remote] badge for missing chat, got:\n%s", modalView)
	}

	// 3. Move selection to Block 1 (Favourite Chats) with 'j' / Down
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = mUpdated.(*Model)
	if m.cfgBoxCursor != 1 {
		t.Errorf("expected 'j' to switch cfgBoxCursor to 1, got %d", m.cfgBoxCursor)
	}
	if m.cfgBoxEntered {
		t.Errorf("expected still at block level before pressing Enter")
	}

	// Press Enter to drill into Block 1 (Favourite Chats)
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mUpdated.(*Model)
	if !m.cfgBoxEntered {
		t.Fatalf("expected Enter on Block 1 to drill in (cfgBoxEntered=true)")
	}
	if m.cfgFavCursor != 0 {
		t.Errorf("expected cfgFavCursor to be 0, got %d", m.cfgFavCursor)
	}

	// Move cursor down to the deleted chat at index 1
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = mUpdated.(*Model)
	if m.cfgFavCursor != 1 {
		t.Errorf("expected cfgFavCursor to be 1, got %d", m.cfgFavCursor)
	}

	// 4. Press 'd' to delete the remotely deleted chat from favorite list
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = mUpdated.(*Model)

	if m.isChatPinned(deletedChatID) {
		t.Errorf("expected deletedChatID %d to be removed from pinned IDs", deletedChatID)
	}
	if len(m.favoriteChats) != 1 {
		t.Fatalf("expected 1 favorite chat remaining after deletion, got %d", len(m.favoriteChats))
	}
	if m.favoriteChats[0].ChatID != activeChatID {
		t.Errorf("expected remaining chat to be %d, got %d", activeChatID, m.favoriteChats[0].ChatID)
	}

	// Check persistence
	persisted := storage.LoadFavoriteChats(m.cfg.DataDir)
	if len(persisted) != 1 || persisted[0].ChatID != activeChatID {
		t.Errorf("persisted favorite chats should only have activeChatID, got %v", persisted)
	}

	// 5. Delete the remaining chat with 'x'
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	m = mUpdated.(*Model)

	if len(m.favoriteChats) != 0 {
		t.Errorf("expected 0 favorite chats after deleting all, got %d", len(m.favoriteChats))
	}

	// 6. Press Esc to exit Block 1 back to block level (modal remains open)
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mUpdated.(*Model)
	if m.cfgBoxEntered {
		t.Errorf("expected Esc to exit block (cfgBoxEntered=false)")
	}
	if m.activeModal != modalGlobalConfig {
		t.Errorf("expected modal to remain open after exiting block with Esc, got %v", m.activeModal)
	}

	// 7. Press Esc at block level to close the modal
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mUpdated.(*Model)
	if m.activeModal != modalNone {
		t.Errorf("expected Esc at block level to close modal, got %v", m.activeModal)
	}
}

func TestHiddenChatsExclusionAndNotice(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{
		ProjectRoot: tmpDir,
		DataDir:     tmpDir,
		Language:    "en",
	}
	m := NewModel(cfg, nil, nil, true)
	if len(m.chats) == 0 {
		t.Fatalf("expected demo chats loaded in model")
	}

	initialCount := len(m.chats)
	targetChat := m.chats[0]

	// 1. Initially no chats hidden and sidebar does not contain notice
	if len(m.hiddenChats) != 0 {
		t.Fatalf("expected 0 hidden chats initially, got %d", len(m.hiddenChats))
	}
	sidebarView := m.renderSidebar(40, 25)
	if strings.Contains(sidebarView, "hidden (check Settings 'C')") {
		t.Errorf("sidebar should not contain hidden notice when 0 hidden chats")
	}

	// 2. Hide chat using hotkey 'H'
	m.focus = focusChats
	m.chatCursor = 0
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'H'}})
	m = mUpdated.(*Model)

	if len(m.chats) != initialCount-1 {
		t.Fatalf("expected visible chats count to decrease to %d, got %d", initialCount-1, len(m.chats))
	}
	if !m.isChatHidden(targetChat.ChatID) {
		t.Errorf("expected targetChat %d to be marked as hidden", targetChat.ChatID)
	}

	// Verify target chat is absent from m.chats
	for _, c := range m.chats {
		if c.ChatID == targetChat.ChatID {
			t.Errorf("hidden chat %d should not be present in visible chats", targetChat.ChatID)
		}
	}

	// Check persistence
	persisted := storage.LoadHiddenChats(m.cfg.DataDir)
	if len(persisted) != 1 || persisted[0].ChatID != targetChat.ChatID {
		t.Errorf("persisted hidden chats mismatch: %+v", persisted)
	}

	// 3. Verify sidebar contains hidden notice
	sidebarView = m.renderSidebar(44, 25)
	if !strings.Contains(sidebarView, "1 chat(s) hidden") {
		t.Errorf("expected sidebar to show hidden notice, got:\n%s", sidebarView)
	}
}

func TestHiddenChatsConfigModalRestore(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{
		ProjectRoot: tmpDir,
		DataDir:     tmpDir,
		Language:    "en",
	}
	m := NewModel(cfg, nil, nil, true)
	if len(m.chats) < 2 {
		t.Fatalf("expected at least 2 demo chats")
	}

	targetChat := m.chats[0]
	initialTotal := len(m.chats)

	// Hide target chat
	m.hideChat(targetChat.ChatID)
	if len(m.chats) != initialTotal-1 {
		t.Fatalf("expected %d visible chats after hiding, got %d", initialTotal-1, len(m.chats))
	}

	// 1. Open Global Configuration modal
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = mUpdated.(*Model)
	if m.activeModal != modalGlobalConfig {
		t.Fatalf("expected activeModal to be modalGlobalConfig, got %v", m.activeModal)
	}

	// 2. Navigate down to Block 2 (Hidden Chats)
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = mUpdated.(*Model)
	if m.cfgBoxCursor != 1 {
		t.Errorf("expected cfgBoxCursor 1, got %d", m.cfgBoxCursor)
	}

	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = mUpdated.(*Model)
	if m.cfgBoxCursor != 2 {
		t.Errorf("expected cfgBoxCursor 2, got %d", m.cfgBoxCursor)
	}

	// 3. Press Enter to drill into Block 2
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mUpdated.(*Model)
	if !m.cfgBoxEntered {
		t.Fatalf("expected Enter to drill into Block 2 (cfgBoxEntered=true)")
	}

	// Verify modal view renders the hidden chat
	modalView := m.renderGlobalConfigModal()
	if !strings.Contains(modalView, "Hidden Chats (1):") {
		t.Errorf("expected modal view to show 'Hidden Chats (1):', got:\n%s", modalView)
	}
	if !strings.Contains(modalView, targetChat.Title) {
		t.Errorf("expected modal view to show hidden chat title %q", targetChat.Title)
	}

	// 4. Press 'u' to unhide chat
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'u'}})
	m = mUpdated.(*Model)

	if m.isChatHidden(targetChat.ChatID) {
		t.Errorf("expected targetChat %d to be unhidden", targetChat.ChatID)
	}
	if len(m.hiddenChats) != 0 {
		t.Errorf("expected 0 hidden chats after unhide, got %d", len(m.hiddenChats))
	}
	if len(m.chats) != initialTotal {
		t.Errorf("expected visible chats to be restored to %d, got %d", initialTotal, len(m.chats))
	}

	// Check persistence
	persisted := storage.LoadHiddenChats(m.cfg.DataDir)
	if len(persisted) != 0 {
		t.Errorf("expected 0 persisted hidden chats after unhide, got %d", len(persisted))
	}

	// 5. Press Esc to exit Block 2 back to block level
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mUpdated.(*Model)
	if m.cfgBoxEntered {
		t.Errorf("expected Esc to exit block (cfgBoxEntered=false)")
	}

	// 6. Press Esc to close modal
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mUpdated.(*Model)
	if m.activeModal != modalNone {
		t.Errorf("expected modal closed, got %v", m.activeModal)
	}

	// 7. Verify sidebar notice is gone
	sidebarView := m.renderSidebar(40, 25)
	if strings.Contains(sidebarView, "hidden (check Settings 'C')") {
		t.Errorf("expected sidebar notice to disappear after unhiding all chats")
	}
}

func TestHiddenChatsSyncExclusion(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{
		ProjectRoot: tmpDir,
		DataDir:     tmpDir,
		Language:    "en",
	}
	m := NewModel(cfg, nil, nil, true)
	if len(m.chats) < 2 {
		t.Fatalf("expected at least 2 demo chats")
	}

	hiddenTarget := m.chats[0]
	visibleTarget := m.chats[1]

	m.hideChat(hiddenTarget.ChatID)

	// 1. loadChatMessagesCmd should return nil for hidden chat
	cmd := m.loadChatMessagesCmd(hiddenTarget)
	if cmd != nil {
		t.Errorf("expected loadChatMessagesCmd to return nil for hidden chat, got %v", cmd)
	}

	// 2. generateAIChatReplyCmd should return nil for hidden chat
	replyCmd := m.generateAIChatReplyCmd(hiddenTarget, nil, "draft")
	if replyCmd != nil {
		t.Errorf("expected generateAIChatReplyCmd to return nil for hidden chat, got %v", replyCmd)
	}

	// 3. liveIncomingMsg for hidden chat should be dropped without altering visible messages
	initLiveCount := len(m.liveMessages)
	mUpdated, _ := m.Update(liveIncomingMsg{
		chatID: hiddenTarget.ChatID,
		msg: storage.LiveMessage{
			MessageID: 99999,
			ChatID:    hiddenTarget.ChatID,
			Sender:    "Hidden Sender",
			Text:      "Spam message from hidden chat",
		},
	})
	m = mUpdated.(*Model)

	if len(m.liveMessages) != initLiveCount {
		t.Errorf("liveMessages should not change when message arrives for hidden chat")
	}

	// 4. liveMessagesLoadedMsg for hidden chat should be ignored
	mUpdated, _ = m.Update(liveMessagesLoadedMsg{
		chatID: hiddenTarget.ChatID,
		messages: []storage.LiveMessage{
			{MessageID: 88888, Text: "Should not load"},
		},
	})
	m = mUpdated.(*Model)
	for _, lm := range m.liveMessages {
		if lm.MessageID == 88888 {
			t.Errorf("hidden chat messages should not be assigned to m.liveMessages")
		}
	}

	// 5. Verify visible target still functions normally
	visibleCmd := m.loadChatMessagesCmd(visibleTarget)
	if visibleCmd == nil {
		t.Errorf("expected non-nil loadChatMessagesCmd for visible chat")
	}
}
