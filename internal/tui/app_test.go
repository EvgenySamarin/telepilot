package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"telepilot/internal/ai"
	"telepilot/internal/config"
	"telepilot/internal/mtproto"
	"telepilot/internal/storage"
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

func TestRussianTitlesLocalization(t *testing.T) {
	m := newTestModel(t, "ru")

	// Verify Top Bar titles
	topBar := m.renderTopBar()
	if !strings.Contains(topBar, "Период:") {
		t.Errorf("renderTopBar() should contain localized 'Период:', got:\n%s", topBar)
	}
	if !strings.Contains(topBar, "Выбрано:") {
		t.Errorf("renderTopBar() should contain localized 'Выбрано:', got:\n%s", topBar)
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

	// Verify Sidebar panel titles
	sidebar := m.renderSidebar(38, 30)
	if !strings.Contains(sidebar, "Чаты") {
		t.Errorf("renderSidebar() should contain 'Чаты', got:\n%s", sidebar)
	}
	if !strings.Contains(sidebar, "Папки периодов") {
		t.Errorf("renderSidebar() should contain 'Папки периодов', got:\n%s", sidebar)
	}

	// Verify Main Panel Tabs
	mainPanel := m.renderMainPanel(80, 30)
	expectedTabs := []string{"Детали", "1:Сводка", "2:Медиафайлы", "3:Стенограмма и голос", "4:Навыки и запуск", "5:Логи: полные"}
	for _, expected := range expectedTabs {
		if !strings.Contains(mainPanel, expected) {
			t.Errorf("renderMainPanel() should contain tab %q, got:\n%s", expected, mainPanel)
		}
	}

	// Verify Tab 1 (Summary) Card Titles
	m.activeTab = 1
	var rec storage.PeriodSummaryRecord
	if len(m.periods) > 0 {
		rec = m.periods[0]
	} else {
		rec = storage.PeriodSummaryRecord{
			ChatTitle:        "Инфраструктура",
			PeriodStart:      "2026-10-01",
			PeriodEnd:        "2026-11-01",
			PeriodSpec:       "1m",
			Status:           "completed",
			ExecutiveSummary: "Краткий отчет готов.",
			KeyDecisions:     "• Решение 1",
			ActionItems:      "[x] Задача 1",
		}
	}
	sumLines := m.renderSummaryTab(&rec, 76)
	sumJoined := strings.Join(sumLines, "\n")
	expectedSumTitles := []string{
		"Сводка за период",
		"Использованный навык и инструкции",
		"Краткое содержание",
		"Ключевые решения и главное",
		"Учет голосовых сообщений, изображений и документов",
		"Задачи и следующие шаги",
	}
	for _, title := range expectedSumTitles {
		if !strings.Contains(sumJoined, title) {
			t.Errorf("renderSummaryTab() should contain %q, got:\n%s", title, sumJoined)
		}
	}

	// Verify Tab 2 (Media Vault)
	m.activeTab = 2
	mediaLines := m.renderMediaVaultTab(&rec, 76, 20)
	mediaJoined := strings.Join(mediaLines, "\n")
	if !strings.Contains(mediaJoined, "Медиафайлы") {
		t.Errorf("renderMediaVaultTab() should contain 'Медиафайлы', got:\n%s", mediaJoined)
	}

	// Verify Tab 3 (Transcript)
	m.activeTab = 3
	trLines := m.renderTranscriptTab(&rec, 76)
	trJoined := strings.Join(trLines, "\n")
	if !strings.Contains(trJoined, "Хронологическая лента сообщений") {
		t.Errorf("renderTranscriptTab() should contain 'Хронологическая лента сообщений', got:\n%s", trJoined)
	}

	// Verify Tab 4 (Skills & Re-Run)
	m.activeTab = 4
	skillLines := m.renderRePromptTab(&rec, 76)
	skillJoined := strings.Join(skillLines, "\n")
	expectedSkillTitles := []string{
		"ИИ-навыки и перезапуск",
		"Текущая конфигурация ИИ и активный навык",
		"Библиотека навыков",
		"Выбранный период и параметры запуска",
		"Действия ИИ и горячие клавиши",
		"Глобальные настройки",
	}
	for _, title := range expectedSkillTitles {
		if !strings.Contains(skillJoined, title) {
			t.Errorf("renderRePromptTab() should contain %q, got:\n%s", title, skillJoined)
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
	// Test that padRightVisual correctly computes terminal visual width
	s1 := "Short"
	padded1 := padRightVisual(s1, 10)
	if len([]rune(padded1)) != 10 {
		t.Errorf("expected 10 runes for ASCII pad, got %d", len([]rune(padded1)))
	}

	// Russian word: 7 runes, 14 bytes in UTF-8
	sRu := "Русский"
	paddedRu := padRightVisual(sRu, 12)
	// Visual width should be exactly 12
	if padOrTruncate(paddedRu, 12) != paddedRu {
		t.Errorf("paddedRu visual width is not 12")
	}

	// Test wrapTextBlock with Cyrillic text
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

	// 1. Verify First-Run Wizard auto-triggers modalLoginForm
	if m.activeModal != modalLoginForm {
		t.Fatalf("expected first-run wizard modalLoginForm on unconfigured start, got %v", m.activeModal)
	}

	// 2. Set an API Hash and test secret masking in modal
	dummySecretHash := "abcdef0123456789abcdef0123456789"
	m.formFields[1] = dummySecretHash
	m.formFocus = 0 // not focused on hash
	rendered := m.renderLoginFormModal()

	// Full hash should NOT appear in plain text
	if strings.Contains(rendered, dummySecretHash) {
		t.Errorf("renderLoginFormModal() leaked raw unmasked API Hash in plain text!")
	}
	// Masked bullets should appear
	if !strings.Contains(rendered, "•") {
		t.Errorf("renderLoginFormModal() expected bullet masking for API Hash")
	}
}
