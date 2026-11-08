package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"telepilot/internal/domain/ai"
	"telepilot/internal/domain/config"
	"telepilot/internal/domain/skills"
	"telepilot/internal/domain/storage"
	"telepilot/internal/network/mtproto"
	"telepilot/internal/presentation/i18n"
)

type focusArea int

const (
	focusChats focusArea = iota
	focusMain
)

type modalKind int

const (
	modalNone modalKind = iota
	modalLoginForm
	modalLoginQR
	modalPrompt
	modalHelp
	modalAIConfig
	modalNewSkill
	modalSkillPreview
	modalGlobalConfig
)

// Bubble Tea Messages
type tickMsg time.Time
type liveSyncTickMsg time.Time

// skillEditedMsg is sent when the external editor for a SKILL.md exits.
type skillEditedMsg struct {
	path string
	err  error
}

// skillImportedMsg is sent when the native macOS skill import picker finishes.
type skillImportedMsg struct {
	imported []skills.Skill
	canceled bool
	err      error
}

type connTestUpdateMsg struct {
	lines []string
	done  bool
	ok    bool
}

type aiTestDoneMsg struct {
	ok   bool
	line string
}

type qrRenderedMsg struct {
	qrArt    string
	loginURL string
	pngPath  string
}

type qrStatusMsg string

type qrDoneMsg struct {
	user string
	err  error
}

type dialogsLoadedMsg struct {
	chats []storage.ChatInfo
	err   error
}

type liveMessagesLoadedMsg struct {
	chatID   int64
	messages []storage.LiveMessage
	err      error
}

type liveIncomingMsg struct {
	chatID int64
	msg    storage.LiveMessage
}

type liveMessageSentMsg struct {
	chatID int64
	msg    storage.LiveMessage
	err    error
}

type liveMediaDownloadedMsg struct {
	chatID         int64
	messageID      int
	mediaType      string
	filePath       string
	revealInFinder bool
	err            error
}

type aiChatReplyGeneratedMsg struct {
	chat     storage.ChatInfo
	reply    string
	sendMode string
	err      error
}

type chatSingleShotRunner struct {
	mu           sync.Mutex
	cancel       context.CancelFunc
	running      bool
	currentInstr string
	queue        []string
	history      []string
}

type parseLogMsg string

type parseDoneMsg struct {
	record *storage.SkillOutputRecord
	err    error
}

type Model struct {
	cfg     *config.AppConfig
	ai      *ai.Engine
	mtproto *mtproto.Service

	width  int
	height int

	focus       focusArea
	activeTab   int // 1:Live Chat, 2:AI & Skills, 3:AI Output, 4:Trace
	activeModal modalKind

	// Single-chat selection in full-height sidebar
	chats            []storage.ChatInfo
	selectedChatID   int64
	chatCursor       int
	chatScrollOffset int
	chatSearching    bool
	chatSearchQuery  string

	// Per-chat AI configurations (persisted in data/chat_ai_config.json)
	chatAIConfigs map[int64]storage.ChatAIConfig

	// Per-chat single-shot AI instruction runners (supports cancel/replace vs append)
	singleShotRunners map[int64]*chatSingleShotRunner

	// Tab 1: Live Chat state
	liveMessages     []storage.LiveMessage
	liveMsgCursor    int
	liveScrollOffset int
	chatComposing    bool
	chatInputBuffer  string
	lastLiveSync     time.Time
	liveStreamCancel context.CancelFunc

	// Tab 3: Per-chat AI skill execution outputs (persisted in data/chat_skill_outputs.json)
	skillOutputs map[int64]storage.SkillOutputRecord

	mainScroll int

	statusText  string
	statusBusy  bool
	authUser    string
	periodSpec  string
	runNotes    string // extra instructions for current chat
	jobDone     int
	jobTotal    int
	startTime   time.Time
	marqueeTick int

	traceLogs []string

	// Login / Connection Test Modal State
	formFields  [4]string // 0: API ID, 1: API Hash, 2: Server IP:Port, 3: AGY Model
	formFocus   int
	connTesting bool
	connPassed  bool
	connLines   []string

	qrArt     string
	qrURL     string
	qrPNGPath string
	qrStatus  string
	qrCancel  context.CancelFunc
	uiEventCh chan tea.Msg

	// Prompt & New Skill modal input buffer
	inputBuffer string

	// AI Engine Configuration modal state
	aiFormFields [2]string // 0: AGY CLI Path, 1: AGY Model
	aiFormFocus  int
	aiTesting    bool
	aiPassed     bool
	aiStatusLine string

	// Skills (Tab 2): loaded from <project>/.agents/skills
	skillList         []skills.Skill
	skillErrs         []error
	skillCursor       int
	skillScrollOffset int
	activeSkill       string // fallback/default summary skill
	previewScroll     int
	tab2BoxCursor     int       // 0..4 box index in Tab 2
	tab2BoxEntered    bool      // true when user pressed Enter to drill into a box (e.g. Box 1 or Box 3)
	tab2ChatOptCursor int       // 0..3 option index when inside Box 1 (Per-Chat AI Instructions)
	tab2BoxBounds     [5][2]int // [startLine, endLine] for each of the 5 boxes in Tab 2
	tab2SkillOffset   int       // line offset from tab2BoxBounds[3][0] to skill row 0
	lastVisibleRows   int       // most recent main panel viewport height
	resizePending     bool      // true right after WindowSizeMsg to realign scroll offsets

	// Global Configuration modal state with 2-level box navigation (matching Tab 2 pattern)
	cfgBoxCursor  int  // 0: Language, 1: Favourite Chats
	cfgBoxEntered bool // true when user pressed Enter to drill into a box
	cfgLangIndex  int  // 0: En, 1: Ru
	cfgFavCursor  int  // selected index in favoriteChats list

	// Favourite / pinned chats (persisted in data/favorite_chats.json)
	favoriteChats   []storage.FavoriteChatRecord
	favoriteChatIDs map[int64]bool
}

func (m *Model) T(key string, args ...any) string {
	return i18n.T(m.cfg.Language, key, args...)
}

func NewModel(cfg *config.AppConfig, aiEngine *ai.Engine, mtSvc *mtproto.Service, forceDemo bool) *Model {
	chats := make([]storage.ChatInfo, len(storage.DemoChats))
	copy(chats, storage.DemoChats)

	favChats := storage.LoadFavoriteChats(cfg.DataDir)
	favIDs := make(map[int64]bool, len(favChats))
	for _, f := range favChats {
		favIDs[f.ChatID] = true
	}
	chats = reorderChatsWithPinnedList(chats, favIDs, favChats)

	var selChatID int64
	var initLiveMsgs []storage.LiveMessage
	if len(chats) > 0 {
		selChatID = chats[0].ChatID
		initLiveMsgs = storage.DemoLiveMessages(chats[0])
	}

	chatAICfgs := storage.LoadChatAIConfigs(cfg.DataDir)
	skillOutputs := storage.LoadChatSkillOutputs(cfg.DataDir)
	if forceDemo && len(skillOutputs) == 0 {
		for _, c := range chats {
			skillOutputs[c.ChatID] = storage.DemoSkillOutput(c, cfg.Language)
		}
		_ = storage.SaveChatSkillOutputs(cfg.DataDir, skillOutputs)
	}

	initLangIdx := 0
	if cfg.IsRussian() {
		initLangIdx = 1
	}

	serverStr := fmt.Sprintf("%s:%d", cfg.TelegramServerIP, cfg.TelegramServerPort)
	initModal := modalNone
	initConnLines := []string{
		"Press 't' to run 3-step MTProto connection check before generating QR.",
		"Press 'F2' to switch between Prod (149.154.167.50:443) & Test (149.154.167.40:443).",
	}
	if !cfg.HasTelegramCredentials() && !forceDemo {
		initModal = modalLoginForm
		if cfg.IsRussian() {
			initConnLines = []string{
				"Добро пожаловать в TelePilot! Укажите ваши Telegram API ID и API Hash.",
				"Получить их можно на https://my.telegram.org/apps",
				"Данные сохраняются локально в .env (защищено и добавлено в .gitignore).",
			}
		} else {
			initConnLines = []string{
				"Welcome to TelePilot! Please enter your Telegram API ID & API Hash.",
				"Get yours from https://my.telegram.org/apps",
				"Credentials are saved locally to .env (protected and gitignored).",
			}
		}
	}

	m := &Model{
		cfg:               cfg,
		ai:                aiEngine,
		mtproto:           mtSvc,
		width:             140,
		height:            38,
		focus:             focusChats,
		activeTab:         1,
		activeModal:       initModal,
		chats:             chats,
		selectedChatID:    selChatID,
		chatAIConfigs:     chatAICfgs,
		singleShotRunners: make(map[int64]*chatSingleShotRunner),
		liveMessages:      initLiveMsgs,
		liveMsgCursor:     maxInt(0, len(initLiveMsgs)-1),
		lastLiveSync:      time.Now(),
		skillOutputs:      skillOutputs,
		statusText:        i18n.T(cfg.Language, "status_ready"),
		periodSpec:        cfg.DefaultPeriod,
		runNotes:          cfg.DefaultNotes,
		activeSkill:       cfg.DefaultSkill,
		cfgBoxCursor:      0,
		cfgBoxEntered:     false,
		cfgLangIndex:      initLangIdx,
		cfgFavCursor:      0,
		favoriteChats:     favChats,
		favoriteChatIDs:   favIDs,
		jobDone:           1,
		jobTotal:          1,
		startTime:         time.Now(),
		uiEventCh:         make(chan tea.Msg, 64),
		formFields: [4]string{
			func() string {
				if cfg.TelegramAPIID > 0 {
					return strconv.Itoa(cfg.TelegramAPIID)
				}
				return ""
			}(),
			cfg.TelegramAPIHash,
			serverStr,
			cfg.AgyModel,
		},
		connLines: initConnLines,
		aiFormFields: [2]string{
			cfg.AgyCLIBin(),
			cfg.AgyModel,
		},
		aiStatusLine: initialAIStatusLine(cfg),
	}

	_ = skills.EnsureDefaults(cfg.SkillsDir())
	m.reloadSkills()
	m.syncCurrentChatAISettings()

	m.addTrace(fmt.Sprintf("TelePilot Go TUI initialized (root=%s, model=%s)", cfg.ProjectRoot, cfg.ActiveModelLabel()))
	m.addTrace(fmt.Sprintf("Configured MTProto server: %s (DC %d, api_id=%d)", serverStr, cfg.TelegramDCID, cfg.TelegramAPIID))
	return m
}

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

func (m *Model) isChatAIEnabled(chatID int64) bool {
	if m.chatAIConfigs == nil {
		return false
	}
	cfg, ok := m.chatAIConfigs[chatID]
	return ok && cfg.Enabled
}

func (m *Model) activeChatAIConfig() (storage.ChatAIConfig, bool) {
	ch := m.activeChat()
	if ch == nil || m.chatAIConfigs == nil {
		return storage.ChatAIConfig{}, false
	}
	cfg, ok := m.chatAIConfigs[ch.ChatID]
	if !ok || !cfg.Enabled {
		return storage.ChatAIConfig{
			ChatID:     ch.ChatID,
			ChatTitle:  ch.Title,
			Enabled:    false,
			Skill:      config.DefaultSkillName,
			Notes:      "",
			PeriodSpec: m.cfg.DefaultPeriod,
		}, false
	}
	if cfg.PeriodSpec == "" {
		cfg.PeriodSpec = m.cfg.DefaultPeriod
	}
	if cfg.Skill == "" {
		cfg.Skill = config.DefaultSkillName
	}
	if cfg.SendMode == "" {
		cfg.SendMode = "draft"
	}
	return cfg, true
}

func (m *Model) syncCurrentChatAISettings() {
	if cfg, ok := m.activeChatAIConfig(); ok {
		m.activeSkill = cfg.Skill
		m.runNotes = cfg.Notes
		if cfg.PeriodSpec != "" {
			m.periodSpec = cfg.PeriodSpec
		}
	} else {
		m.activeSkill = m.cfg.DefaultSkill
		if m.activeSkill == "" {
			m.activeSkill = config.DefaultSkillName
		}
		m.runNotes = ""
		if m.cfg.DefaultPeriod != "" {
			m.periodSpec = m.cfg.DefaultPeriod
		} else {
			m.periodSpec = "1m"
		}
	}
}

func (m *Model) saveCurrentChatAIConfig(skillName, notes, periodSpec string, enabled bool) {
	ch := m.activeChat()
	if ch == nil {
		return
	}
	if m.chatAIConfigs == nil {
		m.chatAIConfigs = make(map[int64]storage.ChatAIConfig)
	}
	if !enabled {
		delete(m.chatAIConfigs, ch.ChatID)
		_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
		m.syncCurrentChatAISettings()
		return
	}
	if strings.TrimSpace(skillName) == "" {
		skillName = config.DefaultSkillName
	}
	if strings.TrimSpace(periodSpec) == "" {
		periodSpec = "1m"
	}
	prev := m.chatAIConfigs[ch.ChatID]
	sendMode := prev.SendMode
	if sendMode == "" {
		sendMode = "draft"
	}
	cfg := storage.ChatAIConfig{
		ChatID:               ch.ChatID,
		ChatTitle:            ch.Title,
		Enabled:              true,
		Skill:                strings.TrimSpace(skillName),
		Notes:                strings.TrimSpace(notes),
		PeriodSpec:           strings.TrimSpace(periodSpec),
		AutoReply:            prev.AutoReply,
		SendMode:             sendMode,
		LastAutoRepliedMsgID: prev.LastAutoRepliedMsgID,
		UpdatedAt:            time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	}
	m.chatAIConfigs[ch.ChatID] = cfg
	_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
	m.syncCurrentChatAISettings()
}

func (m *Model) toggleChatAutoReply() {
	ch := m.activeChat()
	if ch == nil {
		return
	}
	if m.chatAIConfigs == nil {
		m.chatAIConfigs = make(map[int64]storage.ChatAIConfig)
	}
	cur, ok := m.activeChatAIConfig()
	if !ok {
		cur = storage.ChatAIConfig{
			ChatID:     ch.ChatID,
			ChatTitle:  ch.Title,
			Enabled:    true,
			Skill:      m.activeSkill,
			Notes:      m.runNotes,
			PeriodSpec: m.periodSpec,
			SendMode:   "draft",
		}
	}
	cur.Enabled = true
	cur.AutoReply = !cur.AutoReply
	if cur.SendMode == "" {
		cur.SendMode = "draft"
	}
	if len(m.liveMessages) > 0 {
		cur.LastAutoRepliedMsgID = m.liveMessages[len(m.liveMessages)-1].MessageID
	}
	cur.UpdatedAt = time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	m.chatAIConfigs[ch.ChatID] = cur
	_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
	m.syncCurrentChatAISettings()
	m.addTrace(fmt.Sprintf("Chat '%s': AI Auto-Reply set to %v (SendMode=%s)", ch.Title, cur.AutoReply, cur.SendMode))
}

func (m *Model) toggleChatSendMode() {
	ch := m.activeChat()
	if ch == nil {
		return
	}
	if m.chatAIConfigs == nil {
		m.chatAIConfigs = make(map[int64]storage.ChatAIConfig)
	}
	cur, ok := m.activeChatAIConfig()
	if !ok {
		cur = storage.ChatAIConfig{
			ChatID:     ch.ChatID,
			ChatTitle:  ch.Title,
			Enabled:    true,
			Skill:      m.activeSkill,
			Notes:      m.runNotes,
			PeriodSpec: m.periodSpec,
			SendMode:   "draft",
		}
	}
	cur.Enabled = true
	if cur.SendMode == "auto_send" {
		cur.SendMode = "draft"
	} else {
		cur.SendMode = "auto_send"
	}
	cur.UpdatedAt = time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	m.chatAIConfigs[ch.ChatID] = cur
	_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
	m.syncCurrentChatAISettings()
	m.addTrace(fmt.Sprintf("Chat '%s': AI Send Mode set to '%s'", ch.Title, cur.SendMode))
}

func (m *Model) activeSkillOutputRecord() *storage.SkillOutputRecord {
	ch := m.activeChat()
	if ch == nil || m.skillOutputs == nil {
		return nil
	}
	rec, ok := m.skillOutputs[ch.ChatID]
	if !ok || strings.TrimSpace(rec.Output) == "" {
		return nil
	}
	return &rec
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

func (m *Model) reloadSkills() {
	list, errs := skills.Load(m.cfg.SkillsDir())
	m.skillList = list
	m.skillErrs = errs
	if m.skillCursor >= len(m.skillList) {
		m.skillCursor = maxInt(0, len(m.skillList)-1)
	}
	if m.activeSkill == "" {
		m.activeSkill = m.cfg.DefaultSkill
	}
	if m.activeSkill == "" {
		m.activeSkill = config.DefaultSkillName
	}
}

// syncSkillsFromDisk checks whether any skills were added, removed, or modified externally
// (e.g. via macOS Finder or a GUI editor) and reloads them in real time without losing cursor state.
func (m *Model) syncSkillsFromDisk() bool {
	list, errs := skills.Load(m.cfg.SkillsDir())
	changed := len(list) != len(m.skillList)
	if !changed {
		for i := range list {
			if list[i].Name != m.skillList[i].Name ||
				list[i].Hash != m.skillList[i].Hash ||
				list[i].Path != m.skillList[i].Path {
				changed = true
				break
			}
		}
	}
	if !changed {
		return false
	}

	var prevSelectedName string
	if m.skillCursor >= 0 && m.skillCursor < len(m.skillList) {
		prevSelectedName = m.skillList[m.skillCursor].Name
	}
	prevNames := make(map[string]bool, len(m.skillList))
	for _, sk := range m.skillList {
		prevNames[sk.Name] = true
	}

	m.skillList = list
	m.skillErrs = errs

	// If a new skill appeared on disk (e.g. added via Finder), move cursor to it; otherwise preserve selection
	selectedIdx := -1
	for idx, sk := range m.skillList {
		if !prevNames[sk.Name] {
			selectedIdx = idx
			break
		}
	}
	if selectedIdx < 0 && prevSelectedName != "" {
		for idx, sk := range m.skillList {
			if sk.Name == prevSelectedName {
				selectedIdx = idx
				break
			}
		}
	}
	if selectedIdx >= 0 {
		m.skillCursor = selectedIdx
	} else if m.skillCursor >= len(m.skillList) {
		m.skillCursor = maxInt(0, len(m.skillList)-1)
	}

	m.addTrace(fmt.Sprintf("Live-reloaded skills from %s (%d skills)", m.cfg.SkillsDir(), len(m.skillList)))
	return true
}

func initialAIStatusLine(cfg *config.AppConfig) string {
	if bin := cfg.AgyCLIBin(); bin != "" {
		model := strings.TrimSpace(cfg.AgyModel)
		if model == "" {
			model = "auto"
		}
		if cfg.IsRussian() {
			return fmt.Sprintf("[OK] Antigravity CLI (agy) готов: %s (модель=%s) — нажмите 'c' для теста или 'r' для запуска ИИ", bin, model)
		}
		return fmt.Sprintf("[OK] Antigravity CLI (agy) ready at %s (model=%s) — press 'c' to test or 'r' to run AI", bin, model)
	}
	if cfg.IsRussian() {
		return "[!] Бинарный файл Antigravity CLI (agy) не найден — нажмите 'c' для настройки пути"
	}
	return "[!] Antigravity CLI (agy) binary not found — press 'c' to configure agy path"
}

func (m *Model) addTrace(msg string) {
	ts := time.Now().Format("15:04:05")
	m.traceLogs = append(m.traceLogs, fmt.Sprintf("[%s] %s", ts, msg))
	if len(m.traceLogs) > 300 {
		m.traceLogs = m.traceLogs[len(m.traceLogs)-300:]
	}
}

func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		liveSyncTickCmd(),
		waitForUIEvent(m.uiEventCh),
		m.checkInitialAuthCmd(),
		m.startLiveStreamCmd(),
	)
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizePending = true
		return m, nil

	case tickMsg:
		m.marqueeTick++
		if m.marqueeTick%9 == 0 {
			m.syncSkillsFromDisk()
		}
		return m, tickCmd()

	case liveSyncTickMsg:
		var cmds []tea.Cmd
		cmds = append(cmds, liveSyncTickCmd())
		if !m.statusBusy {
			if ch := m.activeChat(); ch != nil {
				cmds = append(cmds, m.loadChatMessagesCmd(*ch))
			}
		}
		return m, tea.Batch(cmds...)

	case liveMessagesLoadedMsg:
		if msg.err == nil && msg.chatID == m.selectedChatID && len(msg.messages) > 0 {
			wasAtBottom := m.liveMsgCursor >= len(m.liveMessages)-1
			m.liveMessages = msg.messages
			m.lastLiveSync = time.Now()
			if wasAtBottom || m.liveMsgCursor >= len(m.liveMessages) {
				m.liveMsgCursor = maxInt(0, len(m.liveMessages)-1)
			}
			// Check if Auto-Reply should trigger on a new incoming message at the end of the chat
			last := m.liveMessages[len(m.liveMessages)-1]
			if cfg, ok := m.chatAIConfigs[msg.chatID]; ok && cfg.Enabled && cfg.AutoReply {
				if cfg.LastAutoRepliedMsgID == 0 {
					cfg.LastAutoRepliedMsgID = last.MessageID
					m.chatAIConfigs[msg.chatID] = cfg
					_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
				} else if !last.IsOutgoing && last.MessageID > cfg.LastAutoRepliedMsgID {
					cfg.LastAutoRepliedMsgID = last.MessageID
					m.chatAIConfigs[msg.chatID] = cfg
					_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
					if ch := m.activeChat(); ch != nil && ch.ChatID == msg.chatID {
						m.addTrace(fmt.Sprintf("Auto-Reply triggered for incoming message #%d in '%s'", last.MessageID, ch.Title))
						return m, m.generateAIChatReplyCmd(*ch, m.liveMessages, cfg.SendMode)
					}
				}
			}
		}
		return m, nil

	case liveIncomingMsg:
		var autoReplyCmd tea.Cmd
		if msg.chatID == m.selectedChatID {
			exists := false
			for _, existing := range m.liveMessages {
				if existing.MessageID == msg.msg.MessageID && msg.msg.MessageID != 0 {
					exists = true
					break
				}
			}
			if !exists {
				wasAtBottom := m.liveMsgCursor >= len(m.liveMessages)-1
				m.liveMessages = append(m.liveMessages, msg.msg)
				if wasAtBottom {
					m.liveMsgCursor = len(m.liveMessages) - 1
				}
				m.lastLiveSync = time.Now()
				m.addTrace(fmt.Sprintf("Live message #%d received in active chat from %s", msg.msg.MessageID, msg.msg.Sender))

				if cfg, ok := m.chatAIConfigs[msg.chatID]; ok && cfg.Enabled && cfg.AutoReply && !msg.msg.IsOutgoing && msg.msg.MessageID > cfg.LastAutoRepliedMsgID {
					cfg.LastAutoRepliedMsgID = msg.msg.MessageID
					m.chatAIConfigs[msg.chatID] = cfg
					_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
					if ch := m.activeChat(); ch != nil && ch.ChatID == msg.chatID {
						m.addTrace(fmt.Sprintf("Auto-Reply triggered for live incoming message #%d in '%s'", msg.msg.MessageID, ch.Title))
						autoReplyCmd = m.generateAIChatReplyCmd(*ch, m.liveMessages, cfg.SendMode)
					}
				}
			}
		} else {
			for i := range m.chats {
				if m.chats[i].ChatID == msg.chatID {
					m.chats[i].UnreadCount++
					break
				}
			}
		}
		if autoReplyCmd != nil {
			return m, tea.Batch(waitForUIEvent(m.uiEventCh), autoReplyCmd)
		}
		return m, waitForUIEvent(m.uiEventCh)

	case aiChatReplyGeneratedMsg:
		if msg.err != nil {
			m.statusText = "AI reply error"
			m.addTrace(fmt.Sprintf("AI reply generation failed for '%s': %v", msg.chat.Title, msg.err))
			return m, nil
		}
		reply := strings.TrimSpace(msg.reply)
		if reply == "" {
			m.statusText = m.T("status_ready")
			return m, nil
		}
		if msg.sendMode == "auto_send" {
			m.addTrace(fmt.Sprintf("AI Auto-Send reply to '%s': %s", msg.chat.Title, truncateSingleLine(reply, 60)))
			return m, m.sendLiveMessageCmd(msg.chat, reply)
		}
		// Default "draft": put generated message into Tab 1 compose box for Enter confirmation
		m.activeTab = 1
		m.focus = focusMain
		m.chatInputBuffer = reply
		m.chatComposing = true
		m.statusText = m.T("status_ready")
		m.addTrace(fmt.Sprintf("AI drafted reply for '%s' into compose box (press Enter to send).", msg.chat.Title))
		return m, nil

	case liveMessageSentMsg:
		if msg.err != nil {
			m.statusText = "Send failed"
			m.addTrace(fmt.Sprintf("Failed to send message: %v", msg.err))
			return m, waitForUIEvent(m.uiEventCh)
		}
		if msg.chatID == m.selectedChatID {
			m.liveMessages = append(m.liveMessages, msg.msg)
			m.liveMsgCursor = len(m.liveMessages) - 1
			m.lastLiveSync = time.Now()
			m.statusText = m.T("status_ready")
			m.addTrace(fmt.Sprintf("Sent message #%d to chat %d", msg.msg.MessageID, msg.chatID))
		}
		return m, waitForUIEvent(m.uiEventCh)

	case liveMediaDownloadedMsg:
		m.statusText = m.T("status_ready")
		if msg.err != nil {
			m.addTrace(fmt.Sprintf("Media download failed: %v", msg.err))
			return m, nil
		}
		for i := range m.liveMessages {
			if m.liveMessages[i].MessageID == msg.messageID {
				m.liveMessages[i].CachedPath = msg.filePath
			}
		}
		if msg.revealInFinder {
			if isMacOS() {
				_ = exec.Command("open", "-R", msg.filePath).Start()
			}
			m.statusText = "Opened in Finder"
			m.addTrace("Revealed live media in Finder: " + msg.filePath)
		} else if msg.mediaType == "voice" {
			_ = playVoiceAudio(msg.filePath)
			m.addTrace("Playing live voice: " + msg.filePath)
		} else {
			_ = openFileInViewer(msg.filePath)
			m.addTrace("Opened live media in default viewer: " + msg.filePath)
		}
		return m, nil

	case connTestUpdateMsg:
		m.connLines = msg.lines
		if msg.done {
			m.connTesting = false
			m.connPassed = msg.ok
			if msg.ok {
				m.addTrace("MTProto 3-step connection test PASSED.")
			} else {
				m.addTrace("MTProto 3-step connection test FAILED.")
			}
		}
		return m, waitForUIEvent(m.uiEventCh)

	case aiTestDoneMsg:
		m.aiTesting = false
		m.aiPassed = msg.ok
		m.aiStatusLine = msg.line
		m.addTrace("AI Engine test: " + msg.line)
		return m, nil

	case skillEditedMsg:
		m.reloadSkills()
		if msg.err != nil {
			m.addTrace(fmt.Sprintf("Editor exited with error: %v", msg.err))
		} else {
			m.addTrace(fmt.Sprintf("Reloaded skills from %s", m.cfg.SkillsDir()))
		}
		return m, nil

	case skillImportedMsg:
		if msg.canceled {
			m.statusText = m.T("status_ready")
			return m, nil
		}
		if msg.err != nil {
			m.statusText = "Skill import error"
			m.addTrace(fmt.Sprintf("Could not import skill: %v", msg.err))
			return m, nil
		}
		m.reloadSkills()
		if len(msg.imported) > 0 {
			first := msg.imported[0]
			for idx, sk := range m.skillList {
				if sk.Name == first.Name || sk.Path == first.Path {
					m.skillCursor = idx
					break
				}
			}
			m.tab2BoxCursor = 3
			m.tab2BoxEntered = true
			m.scrollTab2ToSkill()
			m.statusText = fmt.Sprintf("Imported: %s", first.Name)
			m.addTrace(fmt.Sprintf("Imported %d skill(s) into %s (selected: %s)", len(msg.imported), m.cfg.SkillsDir(), first.Name))
		}
		return m, nil

	case qrRenderedMsg:
		m.qrArt = msg.qrArt
		m.qrURL = msg.loginURL
		m.qrPNGPath = msg.pngPath
		m.addTrace("Generated fresh Telegram Login QR Code token.")
		return m, waitForUIEvent(m.uiEventCh)

	case qrStatusMsg:
		m.qrStatus = string(msg)
		m.addTrace("QR Login: " + string(msg))
		return m, waitForUIEvent(m.uiEventCh)

	case qrDoneMsg:
		if m.qrCancel != nil {
			m.qrCancel()
			m.qrCancel = nil
		}
		if msg.err != nil {
			if msg.err != context.Canceled {
				m.qrStatus = fmt.Sprintf("[!] Error: %v (Press 'r' to retry or 'c' to edit keys)", msg.err)
				m.addTrace(fmt.Sprintf("QR Login error: %v", msg.err))
			}
			return m, waitForUIEvent(m.uiEventCh)
		}
		m.authUser = msg.user
		m.statusText = "Connected " + msg.user
		m.activeModal = modalNone
		m.addTrace("Authorized Telegram session as " + msg.user)
		return m, tea.Batch(waitForUIEvent(m.uiEventCh), m.loadDialogsCmd(), m.startLiveStreamCmd())

	case dialogsLoadedMsg:
		if msg.err == nil && len(msg.chats) > 0 {
			m.chats = m.reorderChatsWithPinned(msg.chats)
			m.chatCursor = 0
			m.selectedChatID = m.chats[0].ChatID
			m.syncCurrentChatAISettings()
			m.statusText = "Live MTProto"
			m.addTrace(fmt.Sprintf("Loaded %d live dialogs from Telegram.", len(m.chats)))
			return m, m.loadChatMessagesCmd(m.chats[0])
		}
		return m, nil

	case parseLogMsg:
		m.addTrace(string(msg))
		m.statusText = string(msg)
		return m, waitForUIEvent(m.uiEventCh)

	case parseDoneMsg:
		m.statusBusy = false
		if msg.err != nil {
			m.statusText = "Skill error"
			m.addTrace(fmt.Sprintf("Chat AI skill execution failed: %v", msg.err))
		} else if msg.record != nil {
			if m.skillOutputs == nil {
				m.skillOutputs = make(map[int64]storage.SkillOutputRecord)
			}
			m.skillOutputs[msg.record.ChatID] = *msg.record
			_ = storage.SaveChatSkillOutputs(m.cfg.DataDir, m.skillOutputs)
			if saved, ok := m.skillOutputs[msg.record.ChatID]; ok {
				*msg.record = saved
			}
			m.activeTab = 3
			m.mainScroll = 0
			m.statusText = m.T("parsed_chats", msg.record.Skill)
			m.addTrace(fmt.Sprintf("Completed AI skill '%s' for '%s' -> switched to Tab 3 (AI Output).", msg.record.Skill, msg.record.ChatTitle))
		}
		return m, waitForUIEvent(m.uiEventCh)

	case tea.KeyMsg:
		if m.activeModal != modalNone {
			return m.handleModalKey(msg)
		}
		if m.chatSearching {
			return m.handleChatSearchKey(msg)
		}
		if m.activeTab == 1 && m.chatComposing {
			return m.handleLiveComposeKey(msg)
		}
		return m.handleMainKey(msg)
	}

	return m, nil
}

// SetQRModalForCapture sets the model state to show the QR modal for captures and previews.
func (m *Model) SetQRModalForCapture(status, qrArt, qrPNGPath string) {
	m.activeModal = modalLoginQR
	m.qrStatus = status
	m.qrArt = qrArt
	m.qrPNGPath = qrPNGPath
}

// SetGlobalConfigModalForCapture configures the model to display the Global Configuration modal.
func (m *Model) SetGlobalConfigModalForCapture(langIdx int) {
	m.activeModal = modalGlobalConfig
	m.cfgLangIndex = langIdx
}

// SetSkillPreviewModalForCapture configures the model to display the Skill Preview modal.
func (m *Model) SetSkillPreviewModalForCapture(skillIdx int) {
	m.activeTab = 2
	if skillIdx >= 0 && skillIdx < len(m.skillList) {
		m.skillCursor = skillIdx
	}
	m.previewScroll = 0
	m.activeModal = modalSkillPreview
}
