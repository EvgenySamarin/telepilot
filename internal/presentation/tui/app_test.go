package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"telepilot/internal/domain/ai"
	"telepilot/internal/domain/config"
	"telepilot/internal/domain/skills"
	"telepilot/internal/domain/storage"
	"telepilot/internal/network/mtproto"
)

func newTestModel(t *testing.T, lang string) *Model {
	t.Helper()
	tempDir := t.TempDir()
	cfg := AppConfigStub(tempDir, lang)
	aiEngine := ai.NewEngine(cfg)
	mtSvc := mtproto.NewService(cfg, aiEngine)
	m := NewModel(cfg, aiEngine, mtSvc, true)
	m.width = 120
	m.height = 36
	return m
}

func AppConfigStub(root, lang string) *config.AppConfig {
	if lang == "" {
		lang = "en"
	}
	return &config.AppConfig{
		ProjectRoot:        root,
		TelegramAPIID:      12345,
		TelegramAPIHash:    "dummy_hash",
		TelegramDCID:       2,
		TelegramServerIP:   "149.154.167.50",
		TelegramServerPort: 443,
		AgyModel:           "auto",
		DefaultPeriod:      "1m",
		DefaultSkill:       "period-summary",
		Language:           lang,
		ExportsDir:         root + "/exports",
		DataDir:            root + "/data",
	}
}

func TestGlobalConfigHotkeyAndLanguageSwitch(t *testing.T) {
	m := newTestModel(t, "en")

	// 1. Initial state is English
	if m.cfg.Language != "en" {
		t.Fatalf("expected initial language to be 'en', got %q", m.cfg.Language)
	}
	if m.activeModal != modalNone {
		t.Fatalf("expected activeModal to be modalNone, got %v", m.activeModal)
	}

	// 2. Press lowercase 'c' -> should open AI config, not global config
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = mUpdated.(*Model)
	if m.activeModal != modalAIConfig {
		t.Errorf("lowercase 'c' should open modalAIConfig, got %v", m.activeModal)
	}
	// Close modal with esc
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mUpdated.(*Model)
	if m.activeModal != modalNone {
		t.Errorf("esc should close modal, got %v", m.activeModal)
	}

	// 3. Press capital 'C' -> should open Global Config window
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = mUpdated.(*Model)
	if m.activeModal != modalGlobalConfig {
		t.Fatalf("capital 'C' must open modalGlobalConfig, got %v", m.activeModal)
	}
	if m.cfgLangIndex != 0 {
		t.Errorf("expected cfgLangIndex to be 0 for 'en', got %d", m.cfgLangIndex)
	}

	// 4. Toggle language with Space
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = mUpdated.(*Model)
	if m.cfgLangIndex != 1 {
		t.Errorf("Space should toggle cfgLangIndex to 1 (Ru), got %d", m.cfgLangIndex)
	}
	if m.cfg.Language != "ru" {
		t.Errorf("m.cfg.Language should immediately update to 'ru', got %q", m.cfg.Language)
	}
	if !m.cfg.IsRussian() {
		t.Errorf("m.cfg.IsRussian() should be true")
	}

	// 5. Press Enter to confirm and close modal
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mUpdated.(*Model)
	if m.activeModal != modalNone {
		t.Errorf("Enter should close modalGlobalConfig, got %v", m.activeModal)
	}
	if m.cfg.Language != "ru" {
		t.Errorf("m.cfg.Language should remain 'ru' after closing, got %q", m.cfg.Language)
	}

	// 6. Re-open with capital 'C' and switch back using key '1'
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = mUpdated.(*Model)
	if m.activeModal != modalGlobalConfig {
		t.Fatalf("capital 'C' should open modalGlobalConfig again, got %v", m.activeModal)
	}
	if m.cfgLangIndex != 1 {
		t.Errorf("cfgLangIndex should be 1 for Russian, got %d", m.cfgLangIndex)
	}

	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
	m = mUpdated.(*Model)
	if m.cfgLangIndex != 0 || m.cfg.Language != "en" {
		t.Errorf("key '1' should select En, got index=%d, lang=%s", m.cfgLangIndex, m.cfg.Language)
	}

	// Close with Esc
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = mUpdated.(*Model)
	if m.activeModal != modalNone {
		t.Errorf("Esc should close modal, got %v", m.activeModal)
	}
}

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

func TestRussianTitlesLocalization(t *testing.T) {
	m := newTestModel(t, "ru")

	// Verify Top Bar titles
	topBar := m.renderTopBar()
	if !strings.Contains(topBar, "Чат:") {
		t.Errorf("renderTopBar() should contain localized 'Чат:', got:\n%s", topBar)
	}

	// Verify Bottom Bar hotkey titles
	bottomBar := m.renderBottomBar()
	if !strings.Contains(bottomBar, "Выход") {
		t.Errorf("renderBottomBar() should contain 'Выход', got:\n%s", bottomBar)
	}
	if !strings.Contains(bottomBar, "Навигация") {
		t.Errorf("renderBottomBar() should contain 'Навигация', got:\n%s", bottomBar)
	}
	if !strings.Contains(bottomBar, "Настройки") {
		t.Errorf("renderBottomBar() should contain 'Настройки' for hotkey C, got:\n%s", bottomBar)
	}
	if !strings.Contains(bottomBar, "ИИ-движок") {
		t.Errorf("renderBottomBar() should contain 'ИИ-движок' for hotkey c, got:\n%s", bottomBar)
	}

	// Verify Full-Height Sidebar panel title
	sidebar := m.renderSidebar(38, 30)
	if !strings.Contains(sidebar, "Чаты") {
		t.Errorf("renderSidebar() should contain 'Чаты', got:\n%s", sidebar)
	}

	// Verify Main Panel 4 Tabs
	mainPanel := m.renderMainPanel(80, 30)
	expectedTabs := []string{"1:Чат", "2:ИИ и навыки", "3:Результат ИИ", "4:Логи: полные"}
	for _, expected := range expectedTabs {
		if !strings.Contains(mainPanel, expected) {
			t.Errorf("renderMainPanel() should contain tab %q, got:\n%s", expected, mainPanel)
		}
	}

	// Verify Tab 2 (Per-Chat AI & Skills)
	m.activeTab = 2
	rec := m.activeSkillOutputRecord()
	skillLines := m.renderRePromptTab(rec, 76)
	skillJoined := strings.Join(skillLines, "\n")
	expectedSkillTitles := []string{
		"ИИ-навыки для чата",
		"Инструкции ИИ для выбранного чата",
		"Текущая конфигурация ИИ и активный навык",
		"Библиотека навыков",
		"Последний результат ИИ-навыка для чата",
		"Действия ИИ и горячие клавиши",
	}
	for _, title := range expectedSkillTitles {
		if !strings.Contains(skillJoined, title) {
			t.Errorf("renderRePromptTab() should contain %q, got:\n%s", title, skillJoined)
		}
	}

	// Verify Tab 3 (AI Skill Output)
	m.activeTab = 3
	outputLines := m.renderSkillOutputTab(rec, 76, 30)
	outputJoined := strings.Join(outputLines, "\n")
	expectedOutputTitles := []string{
		"Результат ИИ-навыка",
		"Использованный навык и параметры запуска",
		"Результат работы ИИ-навыка",
	}
	for _, title := range expectedOutputTitles {
		if !strings.Contains(outputJoined, title) {
			t.Errorf("renderSkillOutputTab() should contain %q, got:\n%s", title, outputJoined)
		}
	}

	// Verify Global Config Modal render
	m.activeModal = modalGlobalConfig
	m.cfgLangIndex = 1
	cfgModal := m.renderGlobalConfigModal()
	if !strings.Contains(cfgModal, "Глобальные настройки") {
		t.Errorf("renderGlobalConfigModal() should contain 'Глобальные настройки', got:\n%s", cfgModal)
	}
	if !strings.Contains(cfgModal, "Ru — Русский") {
		t.Errorf("renderGlobalConfigModal() should contain 'Ru — Русский', got:\n%s", cfgModal)
	}

	// Verify Full View does not crash or corrupt
	m.activeModal = modalNone
	view := m.View()
	if len(view) == 0 {
		t.Errorf("View() returned empty string")
	}
}

func TestTUIFormattingNoCorruption(t *testing.T) {
	s1 := "Short"
	padded1 := padRightVisual(s1, 10)
	if len([]rune(padded1)) != 10 {
		t.Errorf("expected 10 runes for ASCII pad, got %d", len([]rune(padded1)))
	}

	sRu := "Русский"
	paddedRu := padRightVisual(sRu, 12)
	if padOrTruncate(paddedRu, 12) != paddedRu {
		t.Errorf("paddedRu visual width is not 12")
	}

	cyrillicText := "Это длинный текст на русском языке для проверки корректного переноса строк без повреждения разметки терминала."
	lines := wrapTextBlock(cyrillicText, 30)
	for i, l := range lines {
		if len([]rune(l)) > 30 {
			t.Errorf("line %d exceeds max width 30: %q (len %d)", i, l, len([]rune(l)))
		}
	}
}

func TestFirstRunWizardAndSecretMasking(t *testing.T) {
	tempDir := t.TempDir()
	emptyCfg := &config.AppConfig{
		ProjectRoot:        tempDir,
		TelegramAPIID:      0,
		TelegramAPIHash:    "",
		TelegramServerIP:   "149.154.167.50",
		TelegramServerPort: 443,
		Language:           "en",
		ExportsDir:         tempDir + "/exports",
		DataDir:            tempDir + "/data",
	}

	aiEngine := ai.NewEngine(emptyCfg)
	mtSvc := mtproto.NewService(emptyCfg, aiEngine)
	m := NewModel(emptyCfg, aiEngine, mtSvc, false)

	if m.activeModal != modalLoginForm {
		t.Fatalf("expected first-run wizard modalLoginForm on unconfigured start, got %v", m.activeModal)
	}

	dummySecretHash := "abcdef0123456789abcdef0123456789"
	m.formFields[1] = dummySecretHash
	m.formFocus = 0
	rendered := m.renderLoginFormModal()

	if strings.Contains(rendered, dummySecretHash) {
		t.Errorf("renderLoginFormModal() leaked raw unmasked API Hash in plain text!")
	}
	if !strings.Contains(rendered, "•") {
		t.Errorf("renderLoginFormModal() expected bullet masking for API Hash")
	}
}

func TestViewHeightNeverExceedsTerminalHeight(t *testing.T) {
	sizes := [][2]int{
		{126, 34}, // exact user screenshot dimensions
		{80, 24},
		{100, 30},
		{140, 38},
	}
	for _, lang := range []string{"en", "ru"} {
		for _, sz := range sizes {
			m := newTestModel(t, lang)
			mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: sz[0], Height: sz[1]})
			m = mUpdated.(*Model)

			for tab := 1; tab <= 4; tab++ {
				m.activeTab = tab
				view := m.View()
				lines := strings.Split(view, "\n")
				if len(lines) != sz[1] {
					t.Errorf("lang=%s size=%dx%d tab=%d: expected View() height %d lines, got %d lines (topBar would scroll off screen!)",
						lang, sz[0], sz[1], tab, sz[1], len(lines))
				}
				// Verify first line is the topBar (contains Chat: / Чат:)
				if !strings.Contains(lines[0], "Chat:") && !strings.Contains(lines[0], "Чат:") {
					t.Errorf("lang=%s size=%dx%d tab=%d: expected topBar on line 0, got: %q",
						lang, sz[0], sz[1], tab, lines[0])
				}
			}
		}
	}
}

func TestAIChatMessageControlAndAutoReply(t *testing.T) {
	m := newTestModel(t, "en")
	ch := m.activeChat()
	if ch == nil {
		t.Fatalf("expected active chat")
	}

	// Switch to Tab 2 and press 'a' to enable AI Auto-Reply for the selected chat
	m.activeTab = 2
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	m = mUpdated.(*Model)

	cfg, ok := m.activeChatAIConfig()
	if !ok || !cfg.AutoReply {
		t.Fatalf("expected AutoReply to be enabled after pressing 'a' in Tab 2, got ok=%v cfg=%+v", ok, cfg)
	}
	if cfg.SendMode != "draft" {
		t.Fatalf("expected default SendMode to be 'draft', got %q", cfg.SendMode)
	}

	// Press 's' in Tab 2 to switch SendMode to 'auto_send'
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'s'}})
	m = mUpdated.(*Model)
	cfg, _ = m.activeChatAIConfig()
	if cfg.SendMode != "auto_send" {
		t.Fatalf("expected SendMode to switch to 'auto_send' after pressing 's', got %q", cfg.SendMode)
	}

	// Test Draft Mode behavior: aiChatReplyGeneratedMsg with sendMode="draft" populates compose box in Tab 1
	mUpdated, cmd := m.Update(aiChatReplyGeneratedMsg{
		chat:     *ch,
		reply:    "Sounds great, let's order pizza!",
		sendMode: "draft",
	})
	m = mUpdated.(*Model)
	if cmd != nil {
		t.Errorf("expected no immediate send cmd in draft mode")
	}
	if m.activeTab != 1 || !m.chatComposing || m.chatInputBuffer != "Sounds great, let's order pizza!" {
		t.Errorf("expected draft mode to open Tab 1 compose box with AI reply, got tab=%d composing=%v buf=%q",
			m.activeTab, m.chatComposing, m.chatInputBuffer)
	}

	// Test Auto-Send Mode behavior: aiChatReplyGeneratedMsg with sendMode="auto_send" immediately dispatches sendLiveMessageCmd
	m.chatComposing = false
	m.chatInputBuffer = ""
	initialCount := len(m.liveMessages)
	mUpdated, cmd = m.Update(aiChatReplyGeneratedMsg{
		chat:     *ch,
		reply:    "On my way!",
		sendMode: "auto_send",
	})
	m = mUpdated.(*Model)
	if cmd == nil {
		t.Fatalf("expected sendLiveMessageCmd to be returned in auto_send mode")
	}
	sentMsg := cmd()
	mUpdated, _ = m.Update(sentMsg)
	m = mUpdated.(*Model)
	if len(m.liveMessages) != initialCount+1 {
		t.Fatalf("expected message count to increase to %d after auto_send, got %d", initialCount+1, len(m.liveMessages))
	}
}

func TestSingleShotReplaceVsAppendAndCancel(t *testing.T) {
	m := newTestModel(t, "en")
	ch := m.activeChat()
	if ch == nil {
		t.Fatalf("expected active chat")
	}

	// Simulate an active running single-shot instruction on the selected chat
	r := m.getChatSingleShotRunner(ch.ChatID)
	cancelledFirst := false
	r.mu.Lock()
	r.running = true
	r.currentInstr = "first instruction"
	r.history = []string{"first instruction"}
	r.cancel = func() { cancelledFirst = true }
	r.mu.Unlock()

	// 1. Open 'm' modal and press Ctrl+A to APPEND a second instruction without cancelling the first
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = mUpdated.(*Model)
	m.inputBuffer = "second appended instruction"
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlA})
	m = mUpdated.(*Model)

	if cancelledFirst {
		t.Errorf("expected Ctrl+A (Append) NOT to cancel the currently running instruction")
	}
	r.mu.Lock()
	if len(r.queue) != 1 || r.queue[0] != "second appended instruction" {
		t.Errorf("expected second instruction to be queued in Append mode, got queue=%v", r.queue)
	}
	r.mu.Unlock()

	// 2. Open 'm' modal and press Ctrl+X to stop/cancel the running single-shot instruction
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	m = mUpdated.(*Model)
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlX})
	m = mUpdated.(*Model)

	if !cancelledFirst {
		t.Errorf("expected Ctrl+X to cancel the running single-shot instruction")
	}
	r.mu.Lock()
	if r.running || len(r.queue) != 0 {
		t.Errorf("expected runner to be stopped and queue cleared after Ctrl+X, got running=%v queue=%v", r.running, r.queue)
	}
	r.mu.Unlock()
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

func TestMacOSSkillsLiveReloadAndImport(t *testing.T) {
	m := newTestModel(t, "en")
	m.activeTab = 2
	m.focus = focusMain

	initialCount := len(m.skillList)

	// 1. Simulate user adding a new skill in Finder inside .agents/skills/
	newSkillDir := filepath.Join(m.cfg.SkillsDir(), "finder-added-skill")
	if err := os.MkdirAll(newSkillDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(newSkillDir, skills.FileName), []byte("Summarize sprint velocity and blockers."), 0o644); err != nil {
		t.Fatal(err)
	}

	// Trigger live auto-reload via syncSkillsFromDisk
	if !m.syncSkillsFromDisk() {
		t.Fatalf("expected syncSkillsFromDisk() to detect newly added Finder skill")
	}
	if len(m.skillList) != initialCount+1 {
		t.Fatalf("expected skillList count %d, got %d", initialCount+1, len(m.skillList))
	}
	if m.skillList[m.skillCursor].Name != "finder-added-skill" {
		t.Errorf("expected cursor to jump to newly detected Finder skill 'finder-added-skill', got %q", m.skillList[m.skillCursor].Name)
	}

	// 2. Simulate importing an external .md file via modalNewSkill ('n') path paste/drag
	extDir := t.TempDir()
	extSkillFile := filepath.Join(extDir, "incident-postmortem.md")
	if err := os.WriteFile(extSkillFile, []byte("Create a blameless postmortem timeline and root cause analysis."), 0o644); err != nil {
		t.Fatal(err)
	}

	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	m = mUpdated.(*Model)
	if m.activeModal != modalNewSkill {
		t.Fatalf("expected 'n' in Tab 2 to open modalNewSkill")
	}
	m.inputBuffer = extSkillFile
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = mUpdated.(*Model)

	if m.activeModal != modalNone {
		t.Errorf("expected modalNewSkill to close after importing path")
	}
	if len(m.skillList) != initialCount+2 {
		t.Fatalf("expected skillList count %d after path import, got %d", initialCount+2, len(m.skillList))
	}
	if m.skillList[m.skillCursor].Name != "incident-postmortem" {
		t.Errorf("expected imported skill 'incident-postmortem' to be selected, got %q", m.skillList[m.skillCursor].Name)
	}
}

func TestWindowResizePreservesTab2NavigationHighlights(t *testing.T) {
	m := newTestModel(t, "en")
	m.activeTab = 2
	m.focus = focusMain
	m.tab2BoxCursor = 3 // Highlight Box 3: Skills Library

	// Resize to 116x41 (exact dimensions from user screenshot)
	mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: 116, Height: 41})
	m = mUpdated.(*Model)

	view := m.View()
	if !strings.Contains(view, "[Enter: Open]") {
		t.Errorf("expected selected Box 3 navigation highlight '[Enter: Open]' to be visible and not cropped at 116x41, got:\n%s", view)
	}
	if !strings.Contains(view, "I:Import") {
		t.Errorf("expected Box 3 shortcuts ('I:Import') to wrap cleanly instead of being cropped at 116x41, got:\n%s", view)
	}

	// Enter Box 3 and resize to 90x28 -> verify [ACTIVE — ...] highlight is preserved
	m.tab2BoxEntered = true
	mUpdated, _ = m.Update(tea.WindowSizeMsg{Width: 90, Height: 28})
	m = mUpdated.(*Model)

	viewEntered := m.View()
	if !strings.Contains(viewEntered, "Esc:Back]") {
		t.Errorf("expected entered Box 3 navigation highlight 'Esc:Back]' to be visible and not cropped at 90x28, got:\n%s", viewEntered)
	}
}

func TestSkillOutputTabAndObsoleteArchiveHotkeysRemoved(t *testing.T) {
	m := newTestModel(t, "en")
	m.activeTab = 3
	m.focus = focusMain

	// 1. Verify Tab 3 header is "3:AI Output" and Period is removed from top bar
	view := m.View()
	if !strings.Contains(view, "[3:AI Output]") {
		t.Errorf("expected Tab 3 to be labeled '[3:AI Output]', got:\n%s", view)
	}
	if strings.Contains(m.renderTopBar(), "Period:") {
		t.Errorf("expected 'Period:' to be removed from top bar, got:\n%s", m.renderTopBar())
	}

	// 2. Verify obsolete archive/period hotkeys ('p', 'd') no longer open modals
	for _, key := range []rune{'p', 'd'} {
		mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{key}})
		m = mUpdated.(*Model)
		if m.activeModal != modalNone {
			t.Errorf("expected obsolete hotkey %q not to open any modal, got activeModal=%v", string(key), m.activeModal)
		}
	}

	// 3. Simulate completion of a custom free-form AI skill run on the selected chat
	ch := m.activeChat()
	if ch == nil {
		t.Fatal("expected active chat")
	}
	customRec := &storage.SkillOutputRecord{
		ChatID:       ch.ChatID,
		ChatTitle:    ch.Title,
		Skill:        "custom-security-audit",
		SkillHash:    "abc123456789",
		ModelUsed:    "agy-cli/auto",
		MessageCount: len(m.liveMessages),
		Output:       "Security Audit Result:\n- Zero plaintext credentials detected in chat.\n- 2 action items assigned to SRE.",
		CreatedAt:    "2026-11-08 00:00:00 UTC",
	}
	m.activeTab = 2
	mUpdated, _ := m.Update(parseDoneMsg{record: customRec})
	m = mUpdated.(*Model)

	if m.activeTab != 3 {
		t.Errorf("expected completion of AI skill to switch to Tab 3 (AI Output), got tab %d", m.activeTab)
	}
	renderedTab3 := m.View()
	if !strings.Contains(renderedTab3, "custom-security-audit") || !strings.Contains(renderedTab3, "Security Audit Result:") {
		t.Errorf("expected Tab 3 to render free-form skill output, got:\n%s", renderedTab3)
	}
	savedRec := m.activeSkillOutputRecord()
	if savedRec == nil || savedRec.OutputPath == "" {
		t.Fatalf("expected skill output record to be persisted with OutputPath, got %+v", savedRec)
	}
	if _, err := os.Stat(savedRec.OutputPath); err != nil {
		t.Errorf("expected markdown output file to exist at %s: %v", savedRec.OutputPath, err)
	}
}
