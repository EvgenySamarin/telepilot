package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"telepilot/internal/ai"
	"telepilot/internal/config"
	"telepilot/internal/i18n"
	"telepilot/internal/mtproto"
	"telepilot/internal/skills"
	"telepilot/internal/storage"
)

// Tokyo-Night Color Palette
var (
	colBg        = lipgloss.Color("#1a1b26")
	colPanelBg   = lipgloss.Color("#16161e")
	colBorder    = lipgloss.Color("#3b4261")
	colBorderFoc = lipgloss.Color("#7aa2f7")
	colBlue      = lipgloss.Color("#7aa2f7")
	colCyan      = lipgloss.Color("#7dcfff")
	colGreen     = lipgloss.Color("#9ece6a")
	colYellow    = lipgloss.Color("#e0af68")
	colRed       = lipgloss.Color("#f7768e")
	colPurple    = lipgloss.Color("#bb9af7")
	colText      = lipgloss.Color("#c0caf5")
	colMuted     = lipgloss.Color("#565f89")
	colHighlight = lipgloss.Color("#283457")
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
	modalPeriod
	modalPrompt
	modalHelp
	modalConfirmDelete
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
	chatID    int64
	messageID int
	mediaType string
	filePath  string
	err       error
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
	records []*storage.PeriodSummaryRecord
	err     error
}

type Model struct {
	cfg     *config.AppConfig
	ai      *ai.Engine
	mtproto *mtproto.Service

	width  int
	height int

	focus       focusArea
	activeTab   int // 1:Live Chat, 2:AI & Skills, 3:Period Archive, 4:Trace
	activeModal modalKind

	// Single-chat selection in full-height sidebar
	chats            []storage.ChatInfo
	selectedChatID   int64
	chatCursor       int
	chatScrollOffset int

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

	// Tab 3: Period Archives for the currently selected chat
	periods            []storage.PeriodSummaryRecord
	periodCursor       int
	periodScrollOffset int
	mediaCursor        int

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

	// Period & Prompt modal input buffers
	inputBuffer string

	// Delete confirmation modal button focus (0 = Confirm Delete, 1 = Cancel)
	deleteBtnFocus int

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

	// Global Configuration modal state (Language selection: 0 = En, 1 = Ru)
	cfgLangIndex int
}

func (m *Model) T(key string, args ...any) string {
	return i18n.T(m.cfg.Language, key, args...)
}

func NewModel(cfg *config.AppConfig, aiEngine *ai.Engine, mtSvc *mtproto.Service, forceDemo bool) *Model {
	if forceDemo {
		_ = storage.EnsureDemoPeriodFolders(cfg)
	}
	chats := make([]storage.ChatInfo, len(storage.DemoChats))
	copy(chats, storage.DemoChats)

	var selChatID int64
	var initLiveMsgs []storage.LiveMessage
	var initPeriods []storage.PeriodSummaryRecord
	if len(chats) > 0 {
		selChatID = chats[0].ChatID
		initLiveMsgs = storage.DemoLiveMessages(chats[0])
		initPeriods = storage.DiscoverChatPeriodFolders(cfg.ExportsDir, chats[0])
	}

	chatAICfgs := storage.LoadChatAIConfigs(cfg.DataDir)

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
		cfg:            cfg,
		ai:             aiEngine,
		mtproto:        mtSvc,
		width:          140,
		height:         38,
		focus:          focusChats,
		activeTab:      1,
		activeModal:    initModal,
		chats:          chats,
		selectedChatID:    selChatID,
		chatAIConfigs:     chatAICfgs,
		singleShotRunners: make(map[int64]*chatSingleShotRunner),
		liveMessages:      initLiveMsgs,
		liveMsgCursor:  maxInt(0, len(initLiveMsgs)-1),
		lastLiveSync:   time.Now(),
		periods:        initPeriods,
		statusText:     i18n.T(cfg.Language, "status_ready"),
		periodSpec:     cfg.DefaultPeriod,
		runNotes:       cfg.DefaultNotes,
		activeSkill:    cfg.DefaultSkill,
		cfgLangIndex:   initLangIdx,
		jobDone:        1,
		jobTotal:       1,
		startTime:      time.Now(),
		uiEventCh:      make(chan tea.Msg, 64),
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

func (m *Model) generateAIChatReplyCmd(chat storage.ChatInfo, msgs []storage.LiveMessage, sendMode string) tea.Cmd {
	skillName := m.activeSkill
	notes := m.runNotes
	if cfg, ok := m.chatAIConfigs[chat.ChatID]; ok && cfg.Enabled {
		if cfg.Skill != "" {
			skillName = cfg.Skill
		}
		notes = cfg.Notes
		if sendMode == "" {
			sendMode = cfg.SendMode
		}
	}
	if sendMode == "" {
		sendMode = "draft"
	}
	m.statusText = "AI generating reply..."
	msgsCopy := make([]storage.LiveMessage, len(msgs))
	copy(msgsCopy, msgs)

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		reply, err := m.ai.GenerateChatReply(ctx, chat.Title, msgsCopy, skillName, notes)
		return aiChatReplyGeneratedMsg{
			chat:     chat,
			reply:    reply,
			sendMode: sendMode,
			err:      err,
		}
	}
}

func (m *Model) refreshChatPeriods() {
	ch := m.activeChat()
	if ch == nil {
		m.periods = nil
		m.periodCursor = 0
		return
	}
	m.periods = storage.DiscoverChatPeriodFolders(m.cfg.ExportsDir, *ch)
	if m.periodCursor >= len(m.periods) {
		m.periodCursor = maxInt(0, len(m.periods)-1)
	}
	m.mediaCursor = 0
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
	m.refreshChatPeriods()
	m.liveMessages = storage.DemoLiveMessages(ch)
	m.liveMsgCursor = maxInt(0, len(m.liveMessages)-1)
	m.addTrace(fmt.Sprintf("Selected chat '%s' (id=%d)", ch.Title, ch.ChatID))
	return m.loadChatMessagesCmd(ch)
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

func openEditorCmd(filePath string) tea.Cmd {
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = os.Getenv("VISUAL")
	}
	if editor == "" {
		editor = "nano"
	}
	parts := strings.Fields(editor)
	c := parts[0]
	args := append(parts[1:], filePath)
	cmd := exec.Command(c, args...)
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		return skillEditedMsg{path: filePath, err: err}
	})
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

func tickCmd() tea.Cmd {
	return tea.Tick(220*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

// liveSyncTickCmd triggers the 15-second fallback sync for the currently active chat.
func liveSyncTickCmd() tea.Cmd {
	return tea.Tick(15*time.Second, func(t time.Time) tea.Msg {
		return liveSyncTickMsg(t)
	})
}

func waitForUIEvent(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
}

func (m *Model) startLiveStreamCmd() tea.Cmd {
	if m.liveStreamCancel != nil {
		m.liveStreamCancel()
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.liveStreamCancel = cancel

	go func() {
		m.mtproto.StartLiveStream(ctx, func(chatID int64, msg storage.LiveMessage) {
			select {
			case m.uiEventCh <- liveIncomingMsg{chatID: chatID, msg: msg}:
			default:
			}
		})
	}()
	return nil
}

func (m *Model) checkInitialAuthCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ok, user := m.mtproto.CheckAuthorized(ctx)
		if !ok {
			return nil
		}
		chats, err := m.mtproto.ListDialogs(ctx, 40)
		if err != nil || len(chats) == 0 {
			return qrDoneMsg{user: user, err: nil}
		}
		return dialogsLoadedMsg{chats: chats}
	}
}

func (m *Model) loadChatMessagesCmd(chat storage.ChatInfo) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		msgs, err := m.mtproto.FetchRecentMessages(ctx, chat, 50)
		return liveMessagesLoadedMsg{chatID: chat.ChatID, messages: msgs, err: err}
	}
}

func (m *Model) sendLiveMessageCmd(chat storage.ChatInfo, text string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
		defer cancel()
		sent, err := m.mtproto.SendMessage(ctx, chat, text)
		return liveMessageSentMsg{chatID: chat.ChatID, msg: sent, err: err}
	}
}

func (m *Model) downloadAndOpenLiveMediaCmd(chat storage.ChatInfo, msg storage.LiveMessage) tea.Cmd {
	m.statusText = "Downloading media..."
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		path, err := m.mtproto.DownloadMessageMedia(ctx, chat, msg)
		return liveMediaDownloadedMsg{
			chatID:    chat.ChatID,
			messageID: msg.MessageID,
			mediaType: msg.MediaType,
			filePath:  path,
			err:       err,
		}
	}
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.marqueeTick++
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
		if msg.mediaType == "voice" {
			_ = exec.Command("afplay", msg.filePath).Start()
			m.addTrace("Playing live voice: " + msg.filePath)
		} else {
			_ = exec.Command("qlmanage", "-p", msg.filePath).Start()
			m.addTrace("QuickLook live media: " + msg.filePath)
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
			m.chats = msg.chats
			m.chatCursor = 0
			m.selectedChatID = m.chats[0].ChatID
			m.syncCurrentChatAISettings()
			m.refreshChatPeriods()
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
			m.statusText = "Parse error"
			m.addTrace(fmt.Sprintf("Chat period parse failed: %v", msg.err))
		} else {
			m.refreshChatPeriods()
			if len(msg.records) > 0 && msg.records[0] != nil {
				for idx, p := range m.periods {
					if p.FolderPath == msg.records[0].FolderPath {
						m.periodCursor = idx
						break
					}
				}
			}
			m.activeTab = 3
			m.mainScroll = 0
			m.statusText = m.T("parsed_chats", len(msg.records))
			m.addTrace("Completed period summarization -> switched to Tab 3 (Period Archive).")
		}
		return m, waitForUIEvent(m.uiEventCh)

	case tea.KeyMsg:
		if m.activeModal != modalNone {
			return m.handleModalKey(msg)
		}
		if m.activeTab == 1 && m.chatComposing {
			return m.handleLiveComposeKey(msg)
		}
		return m.handleMainKey(msg)
	}

	return m, nil
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
			m.chatCursor = 0
			m.marqueeTick = 0
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
			if len(m.chats) > 0 {
				m.chatCursor = len(m.chats) - 1
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
			if len(m.chats) > 0 {
				m.chatCursor = minInt(len(m.chats)-1, m.chatCursor+6)
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
			m.chatCursor = maxInt(0, m.chatCursor-6)
			m.marqueeTick = 0
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
			if m.chatCursor > 0 {
				m.chatCursor--
				m.marqueeTick = 0
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
			if m.chatCursor < len(m.chats)-1 {
				m.chatCursor++
				m.marqueeTick = 0
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
		if m.activeTab == 2 {
			m.inputBuffer = ""
			m.activeModal = modalNewSkill
		} else if m.activeTab == 3 {
			if rec := m.activePeriodRecord(); rec != nil && len(rec.MediaItems) > 0 {
				m.mediaCursor = (m.mediaCursor + 1) % len(rec.MediaItems)
			}
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

func (m *Model) saveFormCredentials() {
	apiID, _ := strconv.Atoi(strings.TrimSpace(m.formFields[0]))
	apiHash := strings.TrimSpace(m.formFields[1])
	serverRaw := strings.TrimSpace(m.formFields[2])
	serverIP := serverRaw
	serverPort := 443
	if strings.Contains(serverRaw, ":") {
		parts := strings.SplitN(serverRaw, ":", 2)
		serverIP = strings.TrimSpace(parts[0])
		if p, err := strconv.Atoi(strings.TrimSpace(parts[1])); err == nil && p > 0 {
			serverPort = p
		}
	}
	agyModel := strings.TrimSpace(m.formFields[3])
	m.cfg.SaveCredentials(apiID, apiHash, serverIP, serverPort, agyModel)
}

func (m *Model) saveAIConfigForm() {
	agyBin := strings.TrimSpace(m.aiFormFields[0])
	agyModel := strings.TrimSpace(m.aiFormFields[1])
	if agyModel == "" {
		agyModel = "auto"
		m.aiFormFields[1] = agyModel
	}
	m.formFields[3] = agyModel
	m.cfg.SaveAIConfig(agyBin, agyModel)
}

func (m *Model) startAITestCmd() tea.Cmd {
	m.saveAIConfigForm()
	m.aiTesting = true
	m.aiPassed = false
	m.aiStatusLine = fmt.Sprintf("Testing Antigravity CLI (agy) connection (%s)...", m.cfg.ActiveModelLabel())

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		ok, line := m.ai.TestConnection(ctx)
		return aiTestDoneMsg{ok: ok, line: line}
	}
}

func (m *Model) startConnectionTestCmd() tea.Cmd {
	m.saveFormCredentials()
	m.connTesting = true
	m.connPassed = false
	m.connLines = []string{
		fmt.Sprintf("Testing TCP & MTProto 2.0 handshake -> %s:%d (DC %d)...",
			m.cfg.TelegramServerIP, m.cfg.TelegramServerPort, m.cfg.TelegramDCID),
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 18*time.Second)
		defer cancel()

		ok, finalLines := m.mtproto.TestConnection(ctx, func(lines []string) {
			m.uiEventCh <- connTestUpdateMsg{lines: lines, done: false}
		})
		return connTestUpdateMsg{lines: finalLines, done: true, ok: ok}
	}
}

func (m *Model) startQRLoginCmd() tea.Cmd {
	m.saveFormCredentials()
	if m.qrCancel != nil {
		m.qrCancel()
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	m.qrCancel = cancel
	m.activeModal = modalLoginQR
	m.qrArt = ""
	m.qrStatus = fmt.Sprintf("Connecting to %s:%d (DC %d) to export QR login token...",
		m.cfg.TelegramServerIP, m.cfg.TelegramServerPort, m.cfg.TelegramDCID)

	go func() {
		user, err := m.mtproto.RunQRLogin(
			ctx,
			func(qrArt, loginURL, pngPath string) {
				m.uiEventCh <- qrRenderedMsg{qrArt: qrArt, loginURL: loginURL, pngPath: pngPath}
			},
			func(status string) {
				m.uiEventCh <- qrStatusMsg(status)
			},
		)
		m.uiEventCh <- qrDoneMsg{user: user, err: err}
	}()

	return nil
}

func (m *Model) loadDialogsCmd() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		chats, err := m.mtproto.ListDialogs(ctx, 40)
		return dialogsLoadedMsg{chats: chats, err: err}
	}
}

func (m *Model) getChatSingleShotRunner(chatID int64) *chatSingleShotRunner {
	if m.singleShotRunners == nil {
		m.singleShotRunners = make(map[int64]*chatSingleShotRunner)
	}
	r, ok := m.singleShotRunners[chatID]
	if !ok {
		r = &chatSingleShotRunner{}
		if cfg, hasCfg := m.chatAIConfigs[chatID]; hasCfg && strings.TrimSpace(cfg.Notes) != "" {
			r.history = []string{strings.TrimSpace(cfg.Notes)}
		}
		m.singleShotRunners[chatID] = r
	}
	return r
}

func (m *Model) cancelSingleShotForChat(chatID int64, chatTitle string) bool {
	r := m.getChatSingleShotRunner(chatID)
	r.mu.Lock()
	wasRunning := r.running
	hadHistory := len(r.history) > 0
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
	r.running = false
	r.currentInstr = ""
	r.queue = nil
	r.history = nil
	r.mu.Unlock()

	if cfg, ok := m.chatAIConfigs[chatID]; ok && cfg.Notes != "" {
		cfg.Notes = ""
		m.chatAIConfigs[chatID] = cfg
		_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
		m.syncCurrentChatAISettings()
	}
	if wasRunning || hadHistory {
		m.addTrace(fmt.Sprintf("Cleared/stopped single-shot AI instruction for '%s'", chatTitle))
	}
	return wasRunning || hadHistory
}

// runSingleShotInstructionCmd executes a standalone AI instruction directly in the selected chat.
// When appendMode is false (Enter), any previous running instruction for this chat is cancelled immediately and replaced.
// When appendMode is true (Ctrl+A / Ctrl+R), the new instruction is appended/queued after any active instruction without cancelling it.
func (m *Model) runSingleShotInstructionCmd(instruction string, appendMode bool) tea.Cmd {
	ch := m.activeChat()
	if ch == nil {
		return nil
	}
	target := *ch
	msgsCopy := make([]storage.LiveMessage, len(m.liveMessages))
	copy(msgsCopy, m.liveMessages)

	runner := m.getChatSingleShotRunner(target.ChatID)
	runner.mu.Lock()

	if !appendMode {
		// Pure Single-Shot: cancel any active execution or queued instructions for this chat
		if runner.cancel != nil {
			runner.cancel()
			runner.cancel = nil
		}
		if runner.running {
			m.addTrace(fmt.Sprintf("Cancelled previous single-shot instruction in '%s' to run new single-shot.", target.Title))
		}
		runner.queue = []string{instruction}
		runner.history = []string{instruction}
	} else {
		// Append Mode: keep running instruction and queue/combine this one
		runner.history = append(runner.history, instruction)
		if runner.running {
			runner.queue = append(runner.queue, instruction)
			queuedCount := len(runner.queue)
			combinedNotes := strings.Join(runner.history, " + ")
			runner.mu.Unlock()
			skillToUse := m.activeSkill
			if cfg, ok := m.activeChatAIConfig(); ok && cfg.Skill != "" {
				skillToUse = cfg.Skill
			}
			m.saveCurrentChatAIConfig(skillToUse, combinedNotes, m.periodSpec, true)
			m.statusText = fmt.Sprintf("Appended (#%d in queue)", queuedCount)
			m.addTrace(fmt.Sprintf("Appended instruction to active single-shot queue for '%s': %s", target.Title, instruction))
			return nil
		}
		combined := strings.Join(runner.history, "\nThen also: ")
		runner.queue = []string{combined}
	}

	persistedNotes := strings.Join(runner.history, " + ")
	ctx, cancel := context.WithCancel(context.Background())
	runner.cancel = cancel
	runner.running = true
	runner.mu.Unlock()

	skillToUse := m.activeSkill
	if cfg, ok := m.activeChatAIConfig(); ok && cfg.Skill != "" {
		skillToUse = cfg.Skill
	}
	m.saveCurrentChatAIConfig(skillToUse, persistedNotes, m.periodSpec, true)

	m.statusText = "AI Single-Shot..."
	if appendMode {
		m.addTrace(fmt.Sprintf("Executing appended single-shot AI instruction in '%s': %s", target.Title, instruction))
	} else {
		m.addTrace(fmt.Sprintf("Executing fresh single-shot AI instruction in '%s': %s", target.Title, instruction))
	}

	go func(runCtx context.Context) {
		defer func() {
			runner.mu.Lock()
			if runner.cancel != nil {
				// Only clear running state if this goroutine wasn't superseded by a newer one
				select {
				case <-runCtx.Done():
				default:
					runner.running = false
					runner.currentInstr = ""
				}
			}
			runner.mu.Unlock()
		}()

		for {
			runner.mu.Lock()
			if len(runner.queue) == 0 {
				runner.running = false
				runner.currentInstr = ""
				runner.mu.Unlock()
				break
			}
			nextInstr := runner.queue[0]
			runner.queue = runner.queue[1:]
			runner.currentInstr = nextInstr
			runner.mu.Unlock()

			select {
			case <-runCtx.Done():
				return
			default:
			}

			planCtx, planCancel := context.WithTimeout(runCtx, 60*time.Second)
			actions, err := m.ai.PlanSingleShotInstruction(planCtx, target.Title, msgsCopy, nextInstr)
			planCancel()
			if err != nil {
				if runCtx.Err() != nil {
					return
				}
				m.uiEventCh <- parseLogMsg(fmt.Sprintf("Single-shot AI error: %v", err))
				return
			}
			m.uiEventCh <- parseLogMsg(fmt.Sprintf("AI planned %d message(s) for '%s'", len(actions), target.Title))

			for idx, act := range actions {
				if act.DelaySeconds > 0 {
					for rem := act.DelaySeconds; rem > 0; rem-- {
						select {
						case <-runCtx.Done():
							return
						default:
						}
						m.uiEventCh <- parseLogMsg(fmt.Sprintf("AI Single-Shot [%d/%d]: waiting %ds...", idx+1, len(actions), rem))
						select {
						case <-runCtx.Done():
							return
						case <-time.After(1 * time.Second):
						}
					}
				}
				select {
				case <-runCtx.Done():
					return
				default:
				}
				m.uiEventCh <- parseLogMsg(fmt.Sprintf("AI Single-Shot [%d/%d]: sending message...", idx+1, len(actions)))
				sendCtx, sendCancel := context.WithTimeout(runCtx, 15*time.Second)
				sent, sendErr := m.mtproto.SendMessage(sendCtx, target, act.Text)
				sendCancel()
				if runCtx.Err() != nil {
					return
				}
				m.uiEventCh <- liveMessageSentMsg{chatID: target.ChatID, msg: sent, err: sendErr}
				if sendErr != nil {
					return
				}
				msgsCopy = append(msgsCopy, sent)
			}
			m.uiEventCh <- parseLogMsg(fmt.Sprintf("Ready (AI sent %d msg(s))", len(actions)))
		}
	}(ctx)

	return nil
}

// runChatAICmd parses and summarizes the currently selected chat for its configured period, skill, and notes.
func (m *Model) runChatAICmd() tea.Cmd {
	ch := m.activeChat()
	if ch == nil {
		return nil
	}
	target := *ch

	skillName := config.DefaultSkillName
	periodSpec := m.periodSpec
	if cfg, ok := m.activeChatAIConfig(); ok {
		if cfg.Skill != "" {
			skillName = cfg.Skill
		}
		if cfg.PeriodSpec != "" {
			periodSpec = cfg.PeriodSpec
		}
	} else if m.activeSkill != "" {
		skillName = m.activeSkill
	}
	if periodSpec == "" {
		periodSpec = "1m"
	}

	m.statusBusy = true
	m.jobDone = 0
	m.jobTotal = 1
	m.statusText = fmt.Sprintf("Running AI (%s)...", target.Title)

	go func() {
		ctx := context.Background()
		m.uiEventCh <- parseLogMsg(fmt.Sprintf("Running AI for '%s' [%s] via skill '%s'...", target.Title, periodSpec, skillName))
		rec, err := m.mtproto.ParseChatPeriod(ctx, target, periodSpec, skillName, "", func(logLine string) {
			m.uiEventCh <- parseLogMsg(logLine)
		})
		if err != nil {
			m.uiEventCh <- parseDoneMsg{records: nil, err: err}
			return
		}
		m.uiEventCh <- parseDoneMsg{records: []*storage.PeriodSummaryRecord{rec}, err: nil}
	}()

	return nil
}

func (m *Model) activePeriodRecord() *storage.PeriodSummaryRecord {
	if len(m.periods) == 0 {
		return nil
	}
	if m.periodCursor < 0 || m.periodCursor >= len(m.periods) {
		m.periodCursor = 0
	}
	return &m.periods[m.periodCursor]
}

func (m *Model) executeDeleteActivePeriod() {
	rec := m.activePeriodRecord()
	if rec == nil || rec.FolderPath == "" {
		return
	}
	targetPath := rec.FolderPath
	title := rec.ChatTitle
	if err := storage.DeletePeriodRun(targetPath, m.cfg.ExportsDir); err != nil {
		m.statusText = "Delete failed"
		m.addTrace(fmt.Sprintf("Failed to delete period folder %s: %v", targetPath, err))
		return
	}
	m.addTrace(fmt.Sprintf("Deleted period folder and all media subfolders: %s", targetPath))
	m.refreshChatPeriods()
	m.mediaCursor = 0
	m.mainScroll = 0
	m.statusText = fmt.Sprintf("Deleted %s", sanitizeTerminalText(title))
}

// View renders the entire responsive terminal layout.
func (m *Model) View() string {
	if m.width < 60 || m.height < 18 {
		return "Terminal window too small — please resize to at least 80x24."
	}

	if m.activeModal != modalNone {
		return m.renderModalOverlay()
	}

	topBar := m.renderTopBar()
	bottomBar := m.renderBottomBar()

	topH := lipgloss.Height(topBar)
	bottomH := lipgloss.Height(bottomBar)
	bodyHeight := m.height - topH - bottomH
	if bodyHeight < 10 {
		bodyHeight = 10
	}

	sidebarWidth := 38
	if m.width >= 150 {
		sidebarWidth = 42
	} else if m.width < 100 {
		sidebarWidth = 30
	}
	mainWidth := m.width - sidebarWidth - 1
	if mainWidth < 40 {
		mainWidth = 40
	}

	sidebar := m.renderSidebar(sidebarWidth, bodyHeight)
	mainPane := m.renderMainPanel(mainWidth, bodyHeight)

	body := lipgloss.JoinHorizontal(lipgloss.Top, sidebar, " ", mainPane)
	return lipgloss.JoinVertical(lipgloss.Left, topBar, body, bottomBar)
}

func (m *Model) renderTopBar() string {
	icon := "◉"
	statusColor := colGreen
	if m.statusBusy {
		icon = "⟳"
		statusColor = colYellow
	}
	chatTitle := "None"
	aiBadge := ""
	if ch := m.activeChat(); ch != nil {
		chatTitle = sanitizeTerminalText(ch.Title)
		if cfg, ok := m.activeChatAIConfig(); ok {
			aiBadge = " " + lipgloss.NewStyle().Foreground(colPurple).Bold(true).Render("[AI:"+cfg.Skill+"]")
		}
	}

	left := fmt.Sprintf(" %s │ %s%s │ %s: %s",
		lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(icon+" "+m.statusText),
		m.T("active_chat_hdr", lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(chatTitle)),
		aiBadge,
		m.T("period_label"),
		lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(m.periodSpec),
	)

	elapsedSec := int(time.Since(m.startTime).Seconds())
	right := fmt.Sprintf("%s   %ds ",
		lipgloss.NewStyle().Foreground(colPurple).Render(m.cfg.ActiveModelLabel()),
		elapsedSec,
	)

	style := lipgloss.NewStyle().
		Width(m.width).
		Background(colPanelBg).
		Foreground(colText)

	if lipgloss.Width(left)+lipgloss.Width(right)+1 <= m.width {
		gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
		return style.Render(left + strings.Repeat(" ", gap) + right)
	}

	line1 := padOrTruncate(left, m.width)
	line2 := padOrTruncate(" "+strings.TrimSpace(right), m.width)
	return style.Render(line1 + "\n" + line2)
}

func (m *Model) renderBottomBar() string {
	type keyPair struct {
		k string
		v string
	}

	// Context-aware hotkeys tailored to the active tab + common global shortcuts
	var keys []keyPair
	if m.activeTab == 1 && m.chatComposing {
		keys = []keyPair{
			{"Enter", m.T("key_send")},
			{"Esc", m.T("key_cancel")},
			{"Ctrl+U", "Clear"},
		}
	} else {
		keys = []keyPair{
			{"1-4", m.T("key_views")},
			{"Tab/h/l", m.T("key_focus")},
			{"j/k", m.T("key_vim_nav")},
		}
		if m.focus == focusChats {
			keys = append(keys, keyPair{"Enter/Space", m.T("key_select_chat")})
		}
		switch m.activeTab {
		case 1:
			keys = append(keys,
				keyPair{"i/Enter", m.T("key_compose")},
				keyPair{"m", m.T("key_notes")},
				keyPair{"a", m.T("key_ai_reply")},
				keyPair{"v", m.T("key_quicklook")},
				keyPair{"r", m.T("key_refresh")},
			)
		case 2:
			keys = append(keys,
				keyPair{"Enter", m.T("key_activate_skill")},
				keyPair{"a", m.T("key_auto_reply")},
				keyPair{"s", m.T("key_send_mode")},
				keyPair{"A", m.T("key_ai_reply")},
				keyPair{"x", m.T("key_disable_ai")},
				keyPair{"m", m.T("key_notes")},
				keyPair{"v", m.T("key_preview")},
				keyPair{"e", m.T("key_edit_skill")},
				keyPair{"n", m.T("key_new_skill")},
				keyPair{"r", m.T("key_run_ai")},
			)
		case 3:
			keys = append(keys,
				keyPair{"r", m.T("key_run_ai")},
				keyPair{"p", fmt.Sprintf("%s(%s)", m.T("key_period"), m.periodSpec)},
				keyPair{"[/]", m.T("key_prev_next_period")},
				keyPair{"v", m.T("key_quicklook")},
				keyPair{"o", m.T("key_finder")},
				keyPair{"d", m.T("key_delete_period")},
			)
		}
		keys = append(keys,
			keyPair{"c", m.T("key_ai_config")},
			keyPair{"C", m.T("key_global_config")},
			keyPair{"L", m.T("key_login")},
			keyPair{"?", m.T("key_help")},
			keyPair{"q", m.T("key_quit")},
		)
	}

	innerW := m.width - 2
	if innerW < 20 {
		innerW = 20
	}

	var items []string
	for _, kv := range keys {
		kStr := lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(kv.k)
		vStr := lipgloss.NewStyle().Foreground(colMuted).Render(":" + kv.v)
		items = append(items, kStr+vStr)
	}

	var lines []string
	curLine := " "
	for _, item := range items {
		sep := "  "
		if curLine == " " {
			sep = ""
		}
		candidate := curLine + sep + item
		if lipgloss.Width(candidate) <= innerW || curLine == " " {
			curLine = candidate
		} else {
			lines = append(lines, padOrTruncate(curLine, innerW))
			curLine = " " + item
		}
	}
	if strings.TrimSpace(curLine) != "" {
		lines = append(lines, padOrTruncate(curLine, innerW))
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		Width(innerW).
		Render(strings.Join(lines, "\n"))
}

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
	maxChatRows := chatsH - 3
	if maxChatRows < 1 {
		maxChatRows = 1
	}

	if m.chatCursor < m.chatScrollOffset {
		m.chatScrollOffset = m.chatCursor
	}
	if m.chatCursor >= m.chatScrollOffset+maxChatRows {
		m.chatScrollOffset = m.chatCursor - maxChatRows + 1
	}
	if m.chatScrollOffset < 0 {
		m.chatScrollOffset = 0
	}

	chatHdr := m.T("sidebar_chats_hdr", minInt(m.chatCursor+1, len(m.chats)), len(m.chats))
	var chatLines []string
	chatLines = append(chatLines, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(chatHdr, innerW)))

	for i := m.chatScrollOffset; i < len(m.chats) && len(chatLines)-1 < maxChatRows; i++ {
		c := m.chats[i]
		isSelected := c.ChatID == m.selectedChatID
		isFocusedRow := m.focus == focusChats && m.chatCursor == i
		hasAI := m.isChatAIEnabled(c.ChatID)

		selMark := "( )"
		if isSelected {
			selMark = "(●)"
		}
		cursor := "  "
		if isFocusedRow {
			cursor = "▸ "
		}
		aiTag := ""
		if hasAI {
			aiTag = "[AI] "
		}
		cleanTitle := sanitizeTerminalText(c.Title)
		rawRow := fmt.Sprintf("%s%s %s%s (%d)", cursor, selMark, aiTag, cleanTitle, c.UnreadCount)

		var formatted string
		if isFocusedRow {
			formatted = marqueeSingleLine(rawRow, innerW, m.marqueeTick)
			fg := colCyan
			if hasAI {
				fg = colPurple
			}
			formatted = lipgloss.NewStyle().Foreground(fg).Bold(true).Background(colHighlight).Width(innerW).Render(formatted)
		} else {
			formatted = padOrTruncate(rawRow, innerW)
			switch {
			case hasAI && isSelected:
				formatted = lipgloss.NewStyle().Foreground(colPurple).Bold(true).Render(formatted)
			case hasAI:
				formatted = lipgloss.NewStyle().Foreground(colPurple).Render(formatted)
			case isSelected:
				formatted = lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(formatted)
			default:
				formatted = lipgloss.NewStyle().Foreground(colMuted).Render(formatted)
			}
		}
		chatLines = append(chatLines, formatted)
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

func (m *Model) renderMainPanel(width, height int) string {
	innerW := width - 2
	if innerW < 20 {
		innerW = 20
	}
	innerH := height - 2
	if innerH < 8 {
		innerH = 8
	}

	borderCol := colBorder
	if m.focus == focusMain {
		borderCol = colBorderFoc
	}

	tabs := []struct {
		id    int
		label string
	}{
		{1, m.T("tab_1")},
		{2, m.T("tab_2")},
		{3, m.T("tab_3")},
		{4, m.T("tab_4")},
	}
	var tabTokens []string
	tabTokens = append(tabTokens, lipgloss.NewStyle().Foreground(colText).Bold(true).Render(m.T("tab_details")))
	for _, t := range tabs {
		if m.activeTab == t.id {
			tabTokens = append(tabTokens, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render("["+t.label+"]"))
		} else {
			tabTokens = append(tabTokens, lipgloss.NewStyle().Foreground(colMuted).Render("["+t.label+"]"))
		}
	}

	var headerLines []string
	curHdr := ""
	for _, tok := range tabTokens {
		sep := " "
		if curHdr == "" {
			sep = ""
		}
		cand := curHdr + sep + tok
		if lipgloss.Width(cand) <= innerW || curHdr == "" {
			curHdr = cand
		} else {
			headerLines = append(headerLines, padOrTruncate(curHdr, innerW))
			curHdr = "  " + tok
		}
	}
	if strings.TrimSpace(curHdr) != "" {
		headerLines = append(headerLines, padOrTruncate(curHdr, innerW))
	}

	visibleRows := innerH - len(headerLines) - 1
	if visibleRows < 1 {
		visibleRows = 1
	}

	if m.activeTab == 1 {
		liveLines := clampLinesToWidth(m.renderLiveChatTab(innerW, visibleRows), innerW)
		if len(liveLines) > visibleRows {
			liveLines = liveLines[:visibleRows]
		}
		for len(liveLines) < visibleRows {
			liveLines = append(liveLines, strings.Repeat(" ", innerW))
		}
		allLines := append([]string{}, headerLines...)
		allLines = append(allLines, strings.Repeat(" ", innerW))
		allLines = append(allLines, liveLines...)
		return lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(borderCol).
			Width(innerW).
			Height(innerH).
			Render(strings.Join(allLines, "\n"))
	}

	var contentLines []string
	rec := m.activePeriodRecord()

	switch m.activeTab {
	case 2:
		contentLines = m.renderRePromptTab(rec, innerW)
	case 3:
		contentLines = m.renderUnifiedPeriodTab(rec, innerW, visibleRows)
	case 4:
		contentLines = m.renderTraceTab(innerW)
	}
	contentLines = clampLinesToWidth(contentLines, innerW)

	if m.mainScroll >= len(contentLines) {
		m.mainScroll = maxInt(0, len(contentLines)-1)
	}

	sliced := contentLines
	if m.mainScroll < len(sliced) {
		sliced = sliced[m.mainScroll:]
	}
	if len(sliced) > visibleRows {
		sliced = sliced[:visibleRows]
	}
	for len(sliced) < visibleRows {
		sliced = append(sliced, strings.Repeat(" ", innerW))
	}

	allLines := append([]string{}, headerLines...)
	allLines = append(allLines, strings.Repeat(" ", innerW))
	allLines = append(allLines, sliced...)

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderCol).
		Width(innerW).
		Height(innerH).
		Render(strings.Join(allLines, "\n"))
}

// clampLinesToWidth ensures every string element is a single line whose visual width never exceeds maxW,
// preventing Lipgloss from wrapping any line and overflowing the terminal height.
func clampLinesToWidth(lines []string, maxW int) []string {
	if maxW < 1 {
		maxW = 1
	}
	out := make([]string, 0, len(lines))
	for _, raw := range lines {
		for _, sub := range strings.Split(raw, "\n") {
			if lipgloss.Width(sub) > maxW {
				sub = ansi.Truncate(sub, maxW, "")
			}
			out = append(out, sub)
		}
	}
	return out
}

// splitDateAndTime splits "2026-11-03 15:18" into ("2026-11-03", "15:18").
func splitDateAndTime(ts string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(ts))
	if len(parts) >= 2 {
		tPart := parts[1]
		if len(tPart) > 5 {
			tPart = tPart[:5]
		}
		return parts[0], tPart
	}
	if len(parts) == 1 {
		return "", parts[0]
	}
	return "", "--:--"
}

// renderDateSeparator renders a centered muted date divider like " ─────────── 2026-11-03 ─────────── ".
func renderDateSeparator(dateStr string, innerW int) string {
	label := " " + dateStr + " "
	avail := innerW - 4 - lipgloss.Width(label)
	if avail < 6 {
		return lipgloss.NewStyle().Foreground(colMuted).Render("  " + dateStr)
	}
	left := avail / 2
	right := avail - left
	line := "  " + strings.Repeat("─", left) + label + strings.Repeat("─", right)
	return lipgloss.NewStyle().Foreground(colMuted).Render(line)
}

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
	hdrLine := lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(hdrText, innerW))

	inputBoxLines := m.renderLiveInputBox(chatTitle, innerW)
	msgViewportH := visibleRows - 2 - len(inputBoxLines)
	if msgViewportH < 3 {
		msgViewportH = 3
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
					prefix = lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(cursorGlyph+timeStr+" ┃ ")
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
	out = append(out, hdrLine, strings.Repeat(" ", innerW))
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

func renderInteriorCardWithBorder(title string, body string, cardWidth int, titleCol, borderCol lipgloss.Color) []string {
	innerTextW := cardWidth - 4
	if innerTextW < 10 {
		innerTextW = 10
	}

	wrappedLines := wrapTextBlock(body, innerTextW)
	var paddedLines []string
	for _, l := range wrappedLines {
		padded := padOrTruncate(l, innerTextW)
		paddedLines = append(paddedLines, renderColorizedCardLine(padded))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderCol).
		Padding(0, 1).
		Width(cardWidth - 2).
		Render(strings.Join(paddedLines, "\n"))

	var out []string
	if strings.TrimSpace(title) != "" {
		tLine := " " + lipgloss.NewStyle().Foreground(titleCol).Bold(true).Render(truncateSingleLine(title, cardWidth))
		out = append(out, tLine)
	}
	for _, bl := range strings.Split(box, "\n") {
		out = append(out, " "+bl)
	}
	return out
}

// renderColorizedCardLine highlights ON/ACTIVE status tokens in green and OFF/NOT CONFIGURED status tokens in red
// while preserving the exact visual width of the padded card line.
func renderColorizedCardLine(padded string) string {
	type statusTok struct {
		token string
		col   lipgloss.Color
	}
	tokens := []statusTok{
		{"[NOT CONFIGURED]", colRed},
		{"[НЕ НАСТРОЕН]", colRed},
		{"[NOT CONNECTED]", colRed},
		{"[НЕ ПОДКЛЮЧЕН]", colRed},
		{"[CONNECTED]", colGreen},
		{"[ПОДКЛЮЧЕН]", colGreen},
		{"[ACTIVE]", colGreen},
		{"[АКТИВЕН]", colGreen},
		{"[RUNNING]", colGreen},
		{"[ВЫПОЛНЯЕТСЯ]", colGreen},
		{"[OK]", colGreen},
		{"[FAIL]", colRed},
		{"[!]", colRed},
		{"[OFF]", colRed},
		{"[ВЫКЛ]", colRed},
		{"[ON]", colGreen},
		{"[ВКЛ]", colGreen},
		{" OFF ", colRed},
		{" ВЫКЛ ", colRed},
		{" ON ", colGreen},
		{" ВКЛ ", colGreen},
	}

	baseStyle := lipgloss.NewStyle().Foreground(colText)
	var out strings.Builder
	rem := padded
	for len(rem) > 0 {
		bestIdx := -1
		var bestTok statusTok
		for _, st := range tokens {
			if idx := strings.Index(rem, st.token); idx >= 0 {
				if bestIdx == -1 || idx < bestIdx {
					bestIdx = idx
					bestTok = st
				}
			}
		}
		if bestIdx < 0 {
			out.WriteString(baseStyle.Render(rem))
			break
		}
		if bestIdx > 0 {
			out.WriteString(baseStyle.Render(rem[:bestIdx]))
		}
		out.WriteString(lipgloss.NewStyle().Foreground(bestTok.col).Bold(true).Render(bestTok.token))
		rem = rem[bestIdx+len(bestTok.token):]
	}
	return out.String()
}

// renderUnifiedPeriodTab renders Tab 3: Period Archive Selector + Summary + Media Vault + Transcript collapsed in one view.
func (m *Model) renderUnifiedPeriodTab(rec *storage.PeriodSummaryRecord, innerW int, visibleRows int) []string {
	ch := m.activeChat()
	chatTitle := "Selected Chat"
	if ch != nil {
		chatTitle = sanitizeTerminalText(ch.Title)
	}

	if rec == nil {
		return []string{
			lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(m.T("period_tab_hdr", chatTitle, 0, 0, m.periodSpec), innerW)),
			"",
			truncateSingleLine(m.T("sum_empty"), innerW),
		}
	}

	cardW := innerW - 4
	if cardW < 20 {
		cardW = 20
	}

	var out []string
	out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(
		padOrTruncate(m.T("period_tab_hdr", chatTitle, minInt(m.periodCursor+1, len(m.periods)), len(m.periods), m.periodSpec), innerW),
	), "")

	// Period Archive Selector Card for the active chat
	if len(m.periods) > 0 {
		var pRows []string
		for idx, p := range m.periods {
			prefix := "  "
			if idx == m.periodCursor {
				prefix = "▸ "
			}
			line := fmt.Sprintf("%s[%d/%d] %s -> %s (%s)  │  %d msgs (%dv %di %dd)  │  Skill: %s",
				prefix, idx+1, len(m.periods), p.PeriodStart, p.PeriodEnd, p.PeriodSpec,
				p.TotalMessages, p.VoiceCount, p.ImageCount, p.DocumentCount, p.Skill)
			pRows = append(pRows, line)
		}
		out = append(out, renderInteriorCard(m.T("period_selector_card", chatTitle), strings.Join(pRows, "\n"), cardW, colPurple)...)
		out = append(out, "")
	}

	// Section 1: Period Summary Cards
	out = append(out, m.renderSummaryTab(rec, innerW)...)
	out = append(out, "")

	// Section 2: Media Vault
	out = append(out, m.renderMediaVaultSection(rec, innerW)...)
	out = append(out, "")

	// Section 3: Chronological Message & Voice Transcript
	out = append(out, m.renderTranscriptTab(rec, innerW)...)

	return out
}

func (m *Model) renderSummaryTab(rec *storage.PeriodSummaryRecord, innerW int) []string {
	if rec == nil {
		return []string{truncateSingleLine(m.T("sum_empty"), innerW)}
	}

	cardW := innerW - 4
	if cardW < 20 {
		cardW = 20
	}

	var out []string
	title := lipgloss.NewStyle().Foreground(colText).Bold(true).Render(truncateSingleLine(m.T("sum_hdr", rec.ChatTitle), innerW))
	meta := lipgloss.NewStyle().Foreground(colMuted).Render(truncateSingleLine(fmt.Sprintf("   ID: %d  │  Created: %s  │  Model: %s", rec.ChatID, rec.CreatedAt, rec.ModelUsed), innerW))
	out = append(out, title, meta, "")

	statusBody := strings.Join([]string{
		m.T("sum_status_label", rec.Status),
		m.T("sum_period_range", rec.PeriodStart, rec.PeriodEnd, rec.PeriodSpec),
		m.T("sum_accounting", rec.TotalMessages, rec.VoiceCount, rec.ImageCount, rec.DocumentCount),
		m.T("sum_archive", rec.FolderPath),
	}, "\n")
	out = append(out, renderInteriorCard("", statusBody, cardW, colGreen)...)
	out = append(out, "")

	skillDesc := rec.Skill
	if skillDesc == "" {
		skillDesc = m.T("sum_legacy_prompt")
	}
	if rec.SkillHash != "" {
		skillDesc += " (" + rec.SkillHash + ")"
	}
	if strings.TrimSpace(rec.Prompt) != "" {
		skillDesc += "\n" + m.T("sum_extra_notes") + rec.Prompt
	}
	out = append(out, renderInteriorCard(m.T("sum_card_skill"), skillDesc, cardW, colBlue)...)
	out = append(out, "")

	out = append(out, renderInteriorCard(m.T("sum_card_executive"), rec.ExecutiveSummary, cardW, colCyan)...)
	out = append(out, "")

	out = append(out, renderInteriorCard(m.T("sum_card_decisions"), rec.KeyDecisions, cardW, colYellow)...)
	out = append(out, "")

	out = append(out, renderInteriorCard(m.T("sum_card_media"), rec.VoiceMediaInsights, cardW, colPurple)...)
	out = append(out, "")

	out = append(out, renderInteriorCard(m.T("sum_card_actions"), rec.ActionItems, cardW, colGreen)...)
	return out
}

func (m *Model) renderMediaVaultSection(rec *storage.PeriodSummaryRecord, innerW int) []string {
	if rec == nil || len(rec.MediaItems) == 0 {
		return []string{truncateSingleLine(m.T("media_empty"), innerW)}
	}
	if m.mediaCursor < 0 {
		m.mediaCursor = 0
	}
	if m.mediaCursor >= len(rec.MediaItems) {
		m.mediaCursor = len(rec.MediaItems) - 1
	}

	cardW := innerW - 4
	var out []string
	out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(
		padOrTruncate(m.T("media_hdr", m.mediaCursor+1, len(rec.MediaItems), rec.FolderPath), innerW),
	), "")

	for idx, it := range rec.MediaItems {
		tag := "[" + strings.ToUpper(it.MediaType) + "]"
		if it.MediaType == "document" {
			tag = "[DOC]"
		}
		selPrefix := "  "
		bCol := colBorder
		if idx == m.mediaCursor {
			selPrefix = "▸ "
			bCol = colCyan
		}
		header := fmt.Sprintf("%s%s %s  •  %s  •  %s  (%d KB)",
			selPrefix, tag, it.FileName, it.Sender, it.Timestamp, it.FileSizeBytes/1024)
		aiCapture := formatAICaptureText(it.AIText, m.cfg.Language)
		body := m.T("media_file_lbl", it.FilePath, aiCapture)
		cardLines := renderInteriorCard(header, body, cardW, bCol)
		out = append(out, cardLines...)
		out = append(out, "")
	}
	return out
}

func (m *Model) renderMediaVaultTab(rec *storage.PeriodSummaryRecord, innerW int, visibleRows int) []string {
	return m.renderMediaVaultSection(rec, innerW)
}

func (m *Model) renderTranscriptTab(rec *storage.PeriodSummaryRecord, innerW int) []string {
	if rec == nil || len(rec.Messages) == 0 {
		return []string{truncateSingleLine(m.T("transcript_empty"), innerW)}
	}
	textW := innerW - 12
	if textW < 16 {
		textW = 16
	}
	var out []string
	out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(
		padOrTruncate(m.T("transcript_hdr", len(rec.Messages)), innerW),
	), "")

	prevDate := ""
	prevSender := ""
	for _, msg := range rec.Messages {
		msgDate, msgTime := splitDateAndTime(msg.Timestamp)
		sender := sanitizeTerminalText(msg.Sender)
		if sender == "" {
			sender = "Unknown"
		}
		accentCol := colBlue
		if sender == "You" {
			accentCol = colGreen
		}

		dateChanged := msgDate != "" && msgDate != prevDate
		if dateChanged {
			if len(out) > 2 {
				out = append(out, "")
			}
			out = append(out, renderDateSeparator(msgDate, innerW))
			prevDate = msgDate
			prevSender = ""
		}

		if sender != prevSender {
			if len(out) > 2 && !dateChanged {
				out = append(out, "")
			}
			senderLine := "   " + lipgloss.NewStyle().Foreground(accentCol).Bold(true).Render(truncateSingleLine(sender, innerW-4))
			out = append(out, senderLine)
			prevSender = sender
		}

		var bodyLines []string
		if strings.TrimSpace(msg.Text) != "" {
			bodyLines = append(bodyLines, wrapTextBlock(msg.Text, textW)...)
		}
		if msg.MediaType == "voice" || msg.VoiceTranscript != "" {
			aiCap := formatAICaptureText(msg.VoiceTranscript, m.cfg.Language)
			bodyLines = append(bodyLines, wrapTextBlock(fmt.Sprintf("📎 [VOICE - %s]: %s", msg.MediaFile, aiCap), textW)...)
		} else if msg.MediaType != "" || msg.MediaAnalysis != "" {
			mTag := strings.ToUpper(msg.MediaType)
			if mTag == "DOCUMENT" {
				mTag = "DOC"
			}
			aiCap := formatAICaptureText(msg.MediaAnalysis, m.cfg.Language)
			bodyLines = append(bodyLines, wrapTextBlock(fmt.Sprintf("📎 [%s - %s]: %s", mTag, msg.MediaFile, aiCap), textW)...)
		}
		if len(bodyLines) == 0 {
			bodyLines = append(bodyLines, "(empty message)")
		}

		for i, bl := range bodyLines {
			timeStr := padOrTruncate(msgTime, 5)
			if i > 0 {
				timeStr = "     "
			}
			tStyled := lipgloss.NewStyle().Foreground(colMuted).Render("  " + timeStr + " ")
			barStyled := lipgloss.NewStyle().Foreground(accentCol).Render("│ ")
			textCol := colText
			if strings.HasPrefix(bl, "📎 [") {
				textCol = colYellow
			}
			out = append(out, tStyled+barStyled+lipgloss.NewStyle().Foreground(textCol).Render(bl))
		}
	}
	return out
}

func formatAICaptureText(raw string, lang string) string {
	s := strings.TrimSpace(raw)
	if s == "" ||
		strings.HasPrefix(s, "Image ") ||
		strings.HasPrefix(s, "Downloaded image ") ||
		strings.HasPrefix(s, "Document ") ||
		strings.HasPrefix(s, "[Voice note ") ||
		s == "Requires AI transcription" ||
		s == "Требуется ИИ-транскрипция" {
		if strings.ToLower(lang) == "ru" {
			return "Требуется ИИ-транскрипция"
		}
		return "Requires AI transcription"
	}
	return s
}

func (m *Model) scrollTab2ToBox(boxIdx int) {
	if boxIdx < 0 || boxIdx >= len(m.tab2BoxBounds) {
		return
	}
	if boxIdx == 0 {
		m.mainScroll = 0
		return
	}
	bStart := m.tab2BoxBounds[boxIdx][0]
	bEnd := m.tab2BoxBounds[boxIdx][1]
	if bEnd <= bStart {
		return
	}
	visibleRows := maxInt(6, m.height-8)
	if bStart < m.mainScroll {
		m.mainScroll = maxInt(0, bStart-1)
	} else if bEnd > m.mainScroll+visibleRows {
		m.mainScroll = maxInt(0, bEnd-visibleRows+1)
	}
}

func (m *Model) scrollTab2ToSkill() {
	bStart := m.tab2BoxBounds[3][0]
	if bStart <= 0 {
		return
	}
	visibleRows := maxInt(6, m.height-8)
	skillLine := bStart + 2 + m.skillCursor
	if skillLine < m.mainScroll+1 {
		m.mainScroll = maxInt(0, skillLine-2)
	} else if skillLine >= m.mainScroll+visibleRows-2 {
		m.mainScroll = maxInt(0, skillLine-visibleRows+3)
	}
}

func (m *Model) renderTab2Box(boxIdx int, title string, body string, cardW int, defaultTitleCol lipgloss.Color) []string {
	isSelectedBox := m.focus == focusMain && m.activeTab == 2 && m.tab2BoxCursor == boxIdx
	isEnteredBox := isSelectedBox && m.tab2BoxEntered

	borderCol := colBorder
	titleCol := defaultTitleCol
	displayTitle := "  " + title

	if isEnteredBox {
		borderCol = colGreen
		titleCol = colGreen
		if m.cfg.IsRussian() {
			displayTitle = "▼ " + title + " [АКТИВНО — j/k:Выбор, Enter:Применить, Esc:Назад]"
		} else {
			displayTitle = "▼ " + title + " [ACTIVE — j/k:Select, Enter:Apply, Esc:Back]"
		}
	} else if isSelectedBox {
		borderCol = colBorderFoc
		titleCol = colCyan
		if m.cfg.IsRussian() {
			displayTitle = "▸ " + title + " [Enter: Открыть]"
		} else {
			displayTitle = "▸ " + title + " [Enter: Open]"
		}
	}

	return renderInteriorCardWithBorder(displayTitle, body, cardW, titleCol, borderCol)
}

// renderRePromptTab renders Tab 2: Per-Chat AI Configuration & Skills Library with 2-level box navigation.
func (m *Model) renderRePromptTab(rec *storage.PeriodSummaryRecord, innerW int) []string {
	cardW := innerW - 4
	if cardW < 20 {
		cardW = 20
	}
	var out []string
	out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(m.T("skills_tab_hdr"), innerW)), "")

	ch := m.activeChat()
	chatTitle := "Selected Chat"
	if ch != nil {
		chatTitle = sanitizeTerminalText(ch.Title)
	}

	// Box 0 (Top): AI Actions & Shortcuts Card
	var actionsBody string
	if m.cfg.IsRussian() {
		actionsBody = strings.Join([]string{
			"Навыки и архив чата:",
			"    [Enter] Войти в выбранный блок / применить действие (Esc — выйти из блока)",
			"    [x]     Отключить ИИ для текущего чата и сбросить настройки",
			"    [v / e] Просмотр навыка (v) или редактирование SKILL.md в $EDITOR (e)",
			"    [n / r] Создать новый навык (n) или запустить ИИ-анализ периода (r)",
			"Управление живым чатом ИИ:",
			"    [a]     Автоответ ИИ ([ВКЛ] / [ВЫКЛ]) на входящие сообщения в чате",
			"    [s]     Режим отправки ИИ: В черновик поля ввода (Enter) или Сразу в чат",
			"    [A]     Сгенерировать ответ ИИ сейчас (во вкладке 1 — клавиша 'a')",
			"    [m]     Разовая команда ИИ (запуск / дополнение / стоп)",
		}, "\n")
	} else {
		actionsBody = strings.Join([]string{
			"Skills & Period Archive:",
			"    [Enter] Enter highlighted box / apply option (Esc to exit back to boxes)",
			"    [x]     Disable AI for selected chat & clear settings",
			"    [v / e] Preview skill prompt (v) or edit SKILL.md in $EDITOR (e)",
			"    [n / r] Create new skill (n) or run period archive analysis (r)",
			"Live Chat AI Control:",
			"    [a]     Toggle AI Auto-Reply ([ON] / [OFF]) for incoming messages",
			"    [s]     Toggle AI Send Mode (Draft to Compose Box vs Auto-Send)",
			"    [A]     Generate AI reply right now (or press 'a' in Tab 1 Live Chat)",
			"    [m]     Single-Shot AI instruction (run / append / stop)",
		}, "\n")
	}
	b0Start := len(out)
	out = append(out, m.renderTab2Box(0, m.T("skills_card_actions"), actionsBody, cardW, colPurple)...)
	m.tab2BoxBounds[0] = [2]int{b0Start, len(out)}
	out = append(out, "")

	// Box 1: Per-Chat AI Instructions Card
	chatCfg, chatAIEnabled := m.activeChatAIConfig()
	runNotesText := strings.TrimSpace(chatCfg.Notes)
	if ch != nil {
		r := m.getChatSingleShotRunner(ch.ChatID)
		r.mu.Lock()
		if r.running {
			if m.cfg.IsRussian() {
				runNotesText = fmt.Sprintf("[ВКЛ] [ВЫПОЛНЯЕТСЯ] %s", r.currentInstr)
			} else {
				runNotesText = fmt.Sprintf("[ON] [RUNNING] %s", r.currentInstr)
			}
			if len(r.queue) > 0 {
				runNotesText += fmt.Sprintf(" (+%d queued)", len(r.queue))
			}
		} else if len(r.history) > 0 {
			if m.cfg.IsRussian() {
				runNotesText = fmt.Sprintf("[ВКЛ] %s", strings.Join(r.history, " + "))
			} else {
				runNotesText = fmt.Sprintf("[ON] %s", strings.Join(r.history, " + "))
			}
		}
		r.mu.Unlock()
	}
	if runNotesText == "" {
		runNotesText = m.T("skills_notes_none")
	}
	autoReplyLabel := m.T("skills_autoreply_off")
	if chatCfg.AutoReply {
		autoReplyLabel = m.T("skills_autoreply_on")
	}
	sendModeLabel := m.T("skills_sendmode_draft")
	if chatCfg.SendMode == "auto_send" {
		sendModeLabel = m.T("skills_sendmode_auto")
	}
	var chatAIBody string
	var chatAICol lipgloss.Color
	if chatAIEnabled {
		chatAICol = colPurple
		chatAIBody = m.T("skills_chat_enabled", chatCfg.Skill, chatCfg.PeriodSpec, autoReplyLabel, sendModeLabel, runNotesText)
	} else {
		chatAICol = colYellow
		chatAIBody = m.T("skills_chat_disabled", config.DefaultSkillName, m.periodSpec)
	}
	if m.tab2BoxEntered && m.tab2BoxCursor == 1 {
		chatAIBody = highlightBox1EnteredOption(chatAIBody, m.tab2ChatOptCursor)
	}
	b1Start := len(out)
	out = append(out, m.renderTab2Box(1, m.T("skills_card_chat_ai", chatTitle), chatAIBody, cardW, chatAICol)...)
	m.tab2BoxBounds[1] = [2]int{b1Start, len(out)}
	out = append(out, "")

	// Box 2: Common AI Engine Configuration Card
	cliPath := m.cfg.AgyCLIBin()
	apiStatus := "[OFF] [NOT CONNECTED] agy CLI not found — press 'c'"
	if m.cfg.IsRussian() {
		apiStatus = "[ВЫКЛ] [НЕ ПОДКЛЮЧЕН] agy CLI не найден — нажмите 'c'"
	}
	if cliPath != "" {
		if m.cfg.IsRussian() {
			apiStatus = fmt.Sprintf("[ВКЛ] [ПОДКЛЮЧЕН] %s", cliPath)
		} else {
			apiStatus = fmt.Sprintf("[ON] [CONNECTED] %s", cliPath)
		}
	}
	modelName := strings.TrimSpace(m.cfg.AgyModel)
	if modelName == "" {
		modelName = "auto"
	}
	activeSkillLabel := chatCfg.Skill
	if !chatAIEnabled {
		activeSkillLabel = config.DefaultSkillName + " (default)"
	}
	aiConfigBody := strings.Join([]string{
		m.T("skills_lbl_engine", apiStatus, m.cfg.ActiveModelLabel(), modelName),
		m.T("skills_lbl_skill", activeSkillLabel, m.cfg.SkillsDir()),
		m.T("skills_lbl_diag", m.aiStatusLine),
	}, "\n")
	b2Start := len(out)
	out = append(out, m.renderTab2Box(2, m.T("skills_card_ai_config"), aiConfigBody, cardW, colGreen)...)
	m.tab2BoxBounds[2] = [2]int{b2Start, len(out)}
	out = append(out, "")

	// Box 3: Skills Library Card (cursor highlight inside only shown when user pressed Enter to enter Box 3)
	inSkillsBox := m.focus == focusMain && m.activeTab == 2 && m.tab2BoxEntered && m.tab2BoxCursor == 3
	var skillRows []string
	if len(m.skillList) == 0 {
		skillRows = append(skillRows, m.T("skills_empty_lib"))
	} else {
		for idx, sk := range m.skillList {
			prefix := "  "
			isHovered := inSkillsBox && idx == m.skillCursor
			if isHovered {
				prefix = "▸ "
			}
			activeBadge := "         "
			isChatActiveSkill := chatAIEnabled && sk.Name == chatCfg.Skill && sk.Kind == skills.KindSummary
			if isChatActiveSkill {
				activeBadge = m.T("skills_badge_active")
			}
			activeBadge = padRightVisual(activeBadge, 10)

			nameStr := padRightVisual(truncateSingleLine(sk.Name, 18), 18)

			kindBadge := fmt.Sprintf("[%s]", strings.Join(sk.Media, ","))
			if sk.Kind == skills.KindSummary {
				kindBadge = m.T("skills_badge_summary")
			}
			kindBadge = padRightVisual(truncateSingleLine(kindBadge, 14), 14)

			descW := cardW - 52
			if descW < 10 {
				descW = 10
			}
			desc := truncateSingleLine(sk.Description, descW)

			row := fmt.Sprintf("%s%s %s %s %s (%s)", prefix, activeBadge, nameStr, kindBadge, desc, sk.Hash)
			if isHovered {
				skillRows = append(skillRows, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(row))
			} else if isChatActiveSkill {
				skillRows = append(skillRows, lipgloss.NewStyle().Foreground(colPurple).Bold(true).Render(row))
			} else {
				skillRows = append(skillRows, lipgloss.NewStyle().Foreground(colText).Render(row))
			}
		}
	}

	skillsTitle := m.T("skills_card_catalog", minInt(m.skillCursor+1, len(m.skillList)), len(m.skillList))
	b3Start := len(out)
	out = append(out, m.renderTab2Box(3, skillsTitle, strings.Join(skillRows, "\n"), cardW, colCyan)...)
	m.tab2BoxBounds[3] = [2]int{b3Start, len(out)}
	out = append(out, "")

	// Box 4: Selected Chat Latest Archive & Run Settings Card
	b4Start := len(out)
	if rec != nil {
		pendingAI := 0
		transcribedAI := 0
		for _, it := range rec.MediaItems {
			if formatAICaptureText(it.AIText, m.cfg.Language) == "Requires AI transcription" ||
				formatAICaptureText(it.AIText, m.cfg.Language) == "Требуется ИИ-транскрипция" {
				pendingAI++
			} else {
				transcribedAI++
			}
		}

		periodLines := []string{
			m.T("skills_period_sel", sanitizeTerminalText(rec.ChatTitle), rec.PeriodStart, rec.PeriodEnd, rec.PeriodSpec),
			m.T("skills_period_sum", rec.CreatedAt, rec.ModelUsed, rec.Skill),
			m.T("skills_period_media", len(rec.MediaItems), transcribedAI, pendingAI),
			m.T("skills_period_notes", runNotesText),
		}
		out = append(out, m.renderTab2Box(4, m.T("skills_card_run"), strings.Join(periodLines, "\n"), cardW, colBlue)...)
	} else {
		noPeriodBody := m.T("skills_period_none", runNotesText)
		out = append(out, m.renderTab2Box(4, m.T("skills_card_run"), noPeriodBody, cardW, colBlue)...)
	}
	m.tab2BoxBounds[4] = [2]int{b4Start, len(out)}
	return out
}

func highlightBox1EnteredOption(body string, optIdx int) string {
	// Indented option lines in Box 1:
	// line 3: Active Skill value (optIdx 0)
	// line 5: Auto-Reply (optIdx 1)
	// line 6: Send Mode (optIdx 2)
	// line 8: Single-Shot value (optIdx 3)
	targetLineMap := map[int]int{
		0: 3,
		1: 5,
		2: 6,
		3: 8,
	}
	targetLine, ok := targetLineMap[optIdx]
	if !ok {
		return body
	}
	lines := strings.Split(body, "\n")
	if targetLine >= 0 && targetLine < len(lines) {
		trimmed := strings.TrimPrefix(lines[targetLine], "    ")
		lines[targetLine] = "  ▸ " + trimmed
	}
	return strings.Join(lines, "\n")
}

func (m *Model) renderTraceTab(innerW int) []string {
	var out []string
	out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(m.T("trace_hdr"), innerW)), "")
	for _, line := range m.traceLogs {
		out = append(out, "  "+truncateSingleLine(line, innerW-4))
	}
	return out
}

func renderInteriorCard(title string, body string, cardWidth int, titleCol lipgloss.Color) []string {
	return renderInteriorCardWithBorder(title, body, cardWidth, titleCol, colBorder)
}

func (m *Model) renderModalOverlay() string {
	var modalBox string

	switch m.activeModal {
	case modalLoginForm:
		modalBox = m.renderLoginFormModal()
	case modalLoginQR:
		modalBox = m.renderLoginQRModal()
	case modalPeriod:
		modalBox = m.renderInputModal(
			m.T("input_period_title"),
			m.T("input_period_sub"),
			m.inputBuffer,
			m.T("input_period_foot"),
		)
	case modalPrompt:
		modalBox = m.renderSingleShotModal()
	case modalNewSkill:
		modalBox = m.renderInputModal(
			m.T("input_new_skill_title"),
			m.T("input_new_skill_sub"),
			m.inputBuffer,
			m.T("input_new_skill_foot"),
		)
	case modalAIConfig:
		modalBox = m.renderAIConfigModal()
	case modalGlobalConfig:
		modalBox = m.renderGlobalConfigModal()
	case modalHelp:
		modalBox = m.renderHelpModal()
	case modalConfirmDelete:
		modalBox = m.renderConfirmDeleteModal()
	case modalSkillPreview:
		modalBox = m.renderSkillPreviewModal()
	}

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modalBox)
}

func (m *Model) renderSkillPreviewModal() string {
	if len(m.skillList) == 0 || m.skillCursor < 0 || m.skillCursor >= len(m.skillList) {
		return m.renderInputModal("─Skill Preview─", "No skill selected.", "", "Esc: Close")
	}
	sk := m.skillList[m.skillCursor]

	modalW := 96
	if m.width-4 < modalW {
		modalW = m.width - 4
	}
	if modalW < 40 {
		modalW = 40
	}
	innerW := modalW - 4

	modalH := m.height - 4
	if modalH < 14 {
		modalH = 14
	}
	innerH := modalH - 4

	var lines []string
	hdr := fmt.Sprintf("─Skill Preview: %s [%s]─", sk.Name, sk.Kind)
	lines = append(lines, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(hdr, innerW)))
	lines = append(lines, "")

	metaCol := lipgloss.NewStyle().Foreground(colYellow)
	lines = append(lines, metaCol.Render(fmt.Sprintf("Name        : %s  (%s)", sk.Name, sk.Hash)))
	lines = append(lines, metaCol.Render(fmt.Sprintf("Kind        : %s", sk.Kind)))
	if sk.Kind == skills.KindMedia {
		lines = append(lines, metaCol.Render(fmt.Sprintf("Target Media: %s", strings.Join(sk.Media, ", "))))
	}
	lines = append(lines, metaCol.Render(fmt.Sprintf("File Path   : %s", sk.Path)))
	lines = append(lines, metaCol.Render(fmt.Sprintf("Description : %s", sk.Description)))
	lines = append(lines, "")
	lines = append(lines, lipgloss.NewStyle().Foreground(colBlue).Bold(true).Render("Prompt Instructions (<skill>):"))
	lines = append(lines, lipgloss.NewStyle().Foreground(colBorder).Render(strings.Repeat("─", innerW)))

	bodyParagraphs := strings.Split(sk.Body, "\n")
	for _, p := range bodyParagraphs {
		if strings.TrimSpace(p) == "" {
			lines = append(lines, "")
			continue
		}
		for _, wl := range wrapTextBlock(p, innerW) {
			lines = append(lines, lipgloss.NewStyle().Foreground(colText).Render(wl))
		}
	}

	maxVisible := innerH - 1
	if maxVisible < 5 {
		maxVisible = 5
	}
	if m.previewScroll >= len(lines) {
		m.previewScroll = maxInt(0, len(lines)-1)
	}

	bodyLines := lines
	if m.previewScroll < len(bodyLines) {
		bodyLines = bodyLines[m.previewScroll:]
	}
	if len(bodyLines) > maxVisible {
		bodyLines = bodyLines[:maxVisible]
	}
	for len(bodyLines) < maxVisible {
		bodyLines = append(bodyLines, strings.Repeat(" ", innerW))
	}

	footer := lipgloss.NewStyle().Foreground(colMuted).Render(
		fmt.Sprintf("Esc / q / v: Close  │  e: Open in $EDITOR  │  j / k: Scroll [%d/%d]", m.previewScroll+1, maxInt(1, len(lines))),
	)
	bodyLines = append(bodyLines, "", footer)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(bodyLines, "\n"))
}

func (m *Model) renderLoginFormModal() string {
	modalW := 86
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	labels := [4]string{
		"API ID:        ",
		"API Hash:      ",
		"MTProto Server:",
		"AGY Model:     ",
	}

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render("─Telegram MTProto Connection Test & Phone QR Login─"))
	rows = append(rows, "")

	statusBanner := "Verify MTProto connection below, then press 'g' to generate Phone QR code."
	bannerCol := colYellow
	if m.connTesting {
		statusBanner = "⟳ Running 3-step MTProto connection check..."
		bannerCol = colCyan
	} else if m.connPassed {
		statusBanner = "[OK] Connection verified! Press 'g' to generate your Phone QR Code."
		bannerCol = colGreen
	}
	rows = append(rows, lipgloss.NewStyle().Foreground(bannerCol).Bold(true).Render(truncateSingleLine(statusBanner, innerW)))
	rows = append(rows, "")

	fieldW := innerW - 18
	if fieldW < 20 {
		fieldW = 20
	}

	for i := 0; i < 4; i++ {
		val := m.formFields[i]
		if i == 3 && strings.TrimSpace(val) == "" && m.formFocus != 3 {
			val = "auto (Antigravity CLI model)"
		}

		displayVal := val
		if i == 1 && len(val) > 0 {
			if m.formFocus == 1 {
				displayVal = strings.Repeat("•", len(val))
			} else {
				if len(val) > 8 {
					displayVal = val[:4] + strings.Repeat("•", len(val)-8) + val[len(val)-4:]
				} else {
					displayVal = strings.Repeat("•", len(val))
				}
			}
		}

		if m.formFocus == i {
			displayVal += "█"
		}
		fBorder := colBorder
		if m.formFocus == i {
			fBorder = colCyan
		}
		fieldBox := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(fBorder).
			Padding(0, 1).
			Width(fieldW).
			Render(padOrTruncate(displayVal, fieldW-2))

		lbl := lipgloss.NewStyle().Foreground(colBlue).Bold(true).Render("\n" + labels[i])
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, lbl, " ", fieldBox))
	}

	var diagLines []string
	for _, cl := range m.connLines {
		col := colText
		if strings.Contains(cl, "[OK]") {
			col = colGreen
		} else if strings.Contains(cl, "[FAIL]") || strings.Contains(cl, "[!]") {
			col = colRed
		}
		for _, wl := range wrapTextBlock(cl, innerW-6) {
			diagLines = append(diagLines, lipgloss.NewStyle().Foreground(col).Render(padOrTruncate(wl, innerW-6)))
		}
	}
	whileLen := 4
	for len(diagLines) < whileLen {
		diagLines = append(diagLines, strings.Repeat(" ", innerW-6))
	}

	diagBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(diagLines, "\n"))

	rows = append(rows, "", diagBox, "")

	btnW := (innerW - 5) / 2
	if btnW < 24 {
		btnW = 24
	}
	btnTest := lipgloss.NewStyle().Background(colBlue).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render("Test Conn (t / Enter)")
	btnGenQR := lipgloss.NewStyle().Background(colGreen).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render("Generate QR (g)")
	btnSwapIP := lipgloss.NewStyle().Background(colHighlight).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render("Prod/Test IP (F2)")
	btnEsc := lipgloss.NewStyle().Background(colBorder).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render("Cancel / Close (Esc)")

	btnRow1 := lipgloss.JoinHorizontal(lipgloss.Center, btnTest, "   ", btnGenQR)
	btnRow2 := lipgloss.JoinHorizontal(lipgloss.Center, btnSwapIP, "   ", btnEsc)
	rows = append(rows, btnRow1, "", btnRow2)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(rows, "\n"))
}

func (m *Model) renderLoginQRModal() string {
	modalW := 82
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render("─Telegram Phone QR Code Authorization─"))
	rows = append(rows, "")

	stCol := colYellow
	if strings.HasPrefix(m.qrStatus, "[!]") {
		stCol = colRed
	} else if strings.Contains(m.qrStatus, "Scan QR") {
		stCol = colGreen
	}
	rows = append(rows, lipgloss.NewStyle().Foreground(stCol).Bold(true).Render(truncateSingleLine(m.qrStatus, innerW)))
	rows = append(rows, "")

	var qrContent string
	if strings.TrimSpace(m.qrArt) != "" {
		qrContent = lipgloss.PlaceHorizontal(innerW-4, lipgloss.Center, m.qrArt)
	} else {
		qrContent = lipgloss.Place(innerW-4, 14, lipgloss.Center, lipgloss.Center,
			"Connecting to Telegram MTProto DC 2 server & exporting QR login token...")
	}

	qrBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Width(innerW - 2).
		Render(qrContent)

	rows = append(rows, qrBox, "")

	btnW := (innerW - 5) / 2
	if btnW < 24 {
		btnW = 24
	}
	btnRefresh := lipgloss.NewStyle().Background(colBlue).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render("Refresh QR (r)")
	btnQL := lipgloss.NewStyle().Background(colHighlight).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render("QuickLook PNG (v)")
	btnEdit := lipgloss.NewStyle().Background(colYellow).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render("Edit Keys / Conn Test (c)")
	btnEsc := lipgloss.NewStyle().Background(colBorder).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render("Cancel / Close (Esc)")

	btnRow1 := lipgloss.JoinHorizontal(lipgloss.Center, btnRefresh, "   ", btnQL)
	btnRow2 := lipgloss.JoinHorizontal(lipgloss.Center, btnEdit, "   ", btnEsc)
	rows = append(rows, btnRow1, "", btnRow2)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(rows, "\n"))
}

func (m *Model) renderSingleShotModal() string {
	modalW := 82
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	ch := m.activeChat()
	chatTitle := "Selected Chat"
	var isRunning bool
	var curInstr string
	var queued []string
	var history []string
	if ch != nil {
		chatTitle = sanitizeTerminalText(ch.Title)
		r := m.getChatSingleShotRunner(ch.ChatID)
		r.mu.Lock()
		isRunning = r.running
		curInstr = r.currentInstr
		queued = append([]string{}, r.queue...)
		history = append([]string{}, r.history...)
		r.mu.Unlock()
	}

	var statusLines []string
	statusBorder := colBorder
	if isRunning {
		statusBorder = colYellow
		if m.cfg.IsRussian() {
			statusLines = append(statusLines, lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(
				fmt.Sprintf("⟳ Статус: [ВЫПОЛНЯЕТСЯ в чате '%s']", chatTitle),
			))
			statusLines = append(statusLines, fmt.Sprintf("Активная команда : %s", curInstr))
			if len(queued) > 0 {
				statusLines = append(statusLines, fmt.Sprintf("В очереди (%d)    : %s", len(queued), strings.Join(queued, " -> ")))
			}
		} else {
			statusLines = append(statusLines, lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(
				fmt.Sprintf("⟳ Status: [RUNNING in '%s']", chatTitle),
			))
			statusLines = append(statusLines, fmt.Sprintf("Active Instruction : %s", curInstr))
			if len(queued) > 0 {
				statusLines = append(statusLines, fmt.Sprintf("Queued Next (%d)    : %s", len(queued), strings.Join(queued, " -> ")))
			}
		}
	} else {
		if m.cfg.IsRussian() {
			statusLines = append(statusLines, lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(
				fmt.Sprintf("◉ Статус: [ОЖИДАНИЕ — чат '%s']", chatTitle),
			))
			if len(history) > 0 {
				statusLines = append(statusLines, fmt.Sprintf("Предыдущая команда: %s", strings.Join(history, " + ")))
			} else {
				statusLines = append(statusLines, "Нет активных разовых команд для этого чата.")
			}
		} else {
			statusLines = append(statusLines, lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(
				fmt.Sprintf("◉ Status: [IDLE — chat '%s']", chatTitle),
			))
			if len(history) > 0 {
				statusLines = append(statusLines, fmt.Sprintf("Previous Instruction: %s", strings.Join(history, " + ")))
			} else {
				statusLines = append(statusLines, "No active single-shot instruction running for this chat.")
			}
		}
	}

	var wrappedStatus []string
	for _, sl := range statusLines {
		wrappedStatus = append(wrappedStatus, wrapTextBlock(sl, innerW-4)...)
	}
	statusBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(statusBorder).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(wrappedStatus, "\n"))

	wrappedInput := wrapTextBlock(m.inputBuffer+"█", innerW-4)
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colCyan).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(wrappedInput, "\n"))

	lines := []string{
		lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("input_notes_title")),
		"",
		statusBox,
		"",
		lipgloss.NewStyle().Foreground(colText).Render(m.T("input_notes_sub")),
		"",
		inputBox,
		"",
		lipgloss.NewStyle().Foreground(colMuted).Render(m.T("input_notes_foot")),
	}

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(lines, "\n"))
}

func (m *Model) renderInputModal(title, subtitle, value, footer string) string {
	modalW := 76
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	wrappedInput := wrapTextBlock(value+"█", innerW-4)
	inputBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colCyan).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(wrappedInput, "\n"))

	lines := []string{
		lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(title),
		"",
		lipgloss.NewStyle().Foreground(colText).Render(subtitle),
		"",
		inputBox,
		"",
		lipgloss.NewStyle().Foreground(colMuted).Render(footer),
	}

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(lines, "\n"))
}

func (m *Model) renderConfirmDeleteModal() string {
	modalW := 78
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	rec := m.activePeriodRecord()
	chatTitle := "Unknown"
	periodRange := ""
	folderPath := ""
	accounting := ""
	if rec != nil {
		chatTitle = sanitizeTerminalText(rec.ChatTitle)
		periodRange = fmt.Sprintf("%s -> %s (%s)", rec.PeriodStart, rec.PeriodEnd, rec.PeriodSpec)
		folderPath = rec.FolderPath
		if m.cfg.IsRussian() {
			accounting = fmt.Sprintf("%d сообщ.  |  %d голос  |  %d фото  |  %d док.",
				rec.TotalMessages, rec.VoiceCount, rec.ImageCount, rec.DocumentCount)
		} else {
			accounting = fmt.Sprintf("%d messages  |  %d voice  |  %d images  |  %d documents",
				rec.TotalMessages, rec.VoiceCount, rec.ImageCount, rec.DocumentCount)
		}
	}

	warningText := m.T("del_modal_warning")
	var warnLines []string
	for _, wl := range wrapTextBlock(warningText, innerW-2) {
		warnLines = append(warnLines, lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(wl))
	}

	detailsBody := strings.Join([]string{
		m.T("del_modal_chat", chatTitle),
		m.T("del_modal_period", periodRange),
		m.T("del_modal_contents", accounting),
		m.T("del_modal_folder", folderPath),
	}, "\n")

	var detailLines []string
	for _, dl := range wrapTextBlock(detailsBody, innerW-6) {
		detailLines = append(detailLines, lipgloss.NewStyle().Foreground(colText).Render(padOrTruncate(dl, innerW-6)))
	}
	detailsBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colRed).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(detailLines, "\n"))

	btnW := (innerW - 5) / 2
	if btnW < 22 {
		btnW = 22
	}

	confirmStyle := lipgloss.NewStyle().Width(btnW).Align(lipgloss.Center)
	cancelStyle := lipgloss.NewStyle().Width(btnW).Align(lipgloss.Center)
	if m.deleteBtnFocus == 0 {
		confirmStyle = confirmStyle.Background(colRed).Foreground(colBg).Bold(true)
		cancelStyle = cancelStyle.Background(colPanelBg).Foreground(colText)
	} else {
		confirmStyle = confirmStyle.Background(colPanelBg).Foreground(colRed)
		cancelStyle = cancelStyle.Background(colBlue).Foreground(colBg).Bold(true)
	}

	btnConfirm := confirmStyle.Render(m.T("del_modal_confirm"))
	btnCancel := cancelStyle.Render(m.T("del_modal_cancel"))
	btnRow := lipgloss.JoinHorizontal(lipgloss.Center, btnConfirm, "   ", btnCancel)

	hint := lipgloss.NewStyle().Foreground(colMuted).Align(lipgloss.Center).Width(innerW).
		Render(m.T("del_modal_hint"))

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(colRed).Bold(true).Render(m.T("del_modal_title")))
	rows = append(rows, "")
	rows = append(rows, warnLines...)
	rows = append(rows, "", detailsBox, "", btnRow, "", hint)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colRed).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(rows, "\n"))
}

func (m *Model) renderAIConfigModal() string {
	modalW := 84
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	labels := [2]string{
		m.T("ai_modal_path"),
		m.T("ai_modal_model"),
	}

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("ai_modal_title")))
	rows = append(rows, "")

	var banner string
	bannerCol := colYellow
	if m.aiTesting {
		if m.cfg.IsRussian() {
			banner = "⟳ Проверка подключения к Antigravity CLI (agy -p)..."
		} else {
			banner = "⟳ Testing connection to Antigravity CLI (agy -p)..."
		}
		bannerCol = colCyan
	} else if m.aiPassed {
		if m.cfg.IsRussian() {
			banner = "[OK] Antigravity CLI (agy) проверен и сохранен в .env! Нажмите Esc и 'r' для запуска ИИ."
		} else {
			banner = "[OK] Antigravity CLI (agy) verified and saved to .env! Press Esc and 'r' to re-run AI."
		}
		bannerCol = colGreen
	} else {
		if m.cfg.IsRussian() {
			banner = "Настройка пути к бинарному файлу и модели Google Antigravity CLI (agy)."
		} else {
			banner = "Configure Google Antigravity CLI (agy) binary path & model for voice, image & chat summaries."
		}
	}
	rows = append(rows, lipgloss.NewStyle().Foreground(bannerCol).Bold(true).Render(truncateSingleLine(banner, innerW)))
	rows = append(rows, "")

	fieldW := innerW - 18
	if fieldW < 20 {
		fieldW = 20
	}

	for i := 0; i < 2; i++ {
		val := m.aiFormFields[i]
		if i == 0 && strings.TrimSpace(val) == "" && m.aiFormFocus != 0 {
			if m.cfg.IsRussian() {
				val = "Авто-поиск agy в ~/.local/bin/agy или PATH"
			} else {
				val = "Auto-detect agy in ~/.local/bin/agy or PATH"
			}
		}
		if m.aiFormFocus == i {
			val += "█"
		}
		fBorder := colBorder
		if m.aiFormFocus == i {
			fBorder = colCyan
		}
		fieldBox := lipgloss.NewStyle().
			Border(lipgloss.NormalBorder()).
			BorderForeground(fBorder).
			Padding(0, 1).
			Width(fieldW).
			Render(padOrTruncate(val, fieldW-2))

		lbl := lipgloss.NewStyle().Foreground(colBlue).Bold(true).Render("\n" + labels[i])
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, lbl, " ", fieldBox))
	}

	diagCol := colText
	if strings.Contains(m.aiStatusLine, "[OK]") {
		diagCol = colGreen
	} else if strings.Contains(m.aiStatusLine, "[FAIL]") || strings.Contains(m.aiStatusLine, "[!]") {
		diagCol = colRed
	}
	var diagLines []string
	for _, wl := range wrapTextBlock(m.aiStatusLine, innerW-6) {
		diagLines = append(diagLines, lipgloss.NewStyle().Foreground(diagCol).Render(padOrTruncate(wl, innerW-6)))
	}
	for len(diagLines) < 2 {
		diagLines = append(diagLines, strings.Repeat(" ", innerW-6))
	}

	diagBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
		Padding(0, 1).
		Width(innerW - 2).
		Render(strings.Join(diagLines, "\n"))

	rows = append(rows, "", diagBox, "")

	btnW := (innerW - 5) / 2
	if btnW < 24 {
		btnW = 24
	}
	btnTest := lipgloss.NewStyle().Background(colGreen).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("ai_modal_btn_test"))
	btnCycle := lipgloss.NewStyle().Background(colBlue).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("ai_modal_btn_cycle"))
	btnSaveClose := lipgloss.NewStyle().Background(colHighlight).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render(m.T("ai_modal_btn_save"))
	btnEsc := lipgloss.NewStyle().Background(colBorder).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render(m.T("ai_modal_btn_cancel"))

	btnRow1 := lipgloss.JoinHorizontal(lipgloss.Center, btnTest, "   ", btnCycle)
	btnRow2 := lipgloss.JoinHorizontal(lipgloss.Center, btnSaveClose, "   ", btnEsc)
	rows = append(rows, btnRow1, "", btnRow2)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(rows, "\n"))
}

func (m *Model) renderGlobalConfigModal() string {
	modalW := 84
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	innerW := modalW - 4

	var rows []string
	rows = append(rows, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("cfg_modal_title")))
	rows = append(rows, "")

	banner := m.T("cfg_modal_banner")
	rows = append(rows, lipgloss.NewStyle().Foreground(colYellow).Bold(true).Render(truncateSingleLine(banner, innerW)))
	rows = append(rows, "")

	langLabel := lipgloss.NewStyle().Foreground(colBlue).Bold(true).Render(m.T("cfg_modal_lang"))
	rows = append(rows, langLabel)

	optW := innerW - 4
	if optW < 30 {
		optW = 30
	}

	enPrefix := "  "
	ruPrefix := "  "
	enStyle := lipgloss.NewStyle().Width(optW).Padding(0, 1)
	ruStyle := lipgloss.NewStyle().Width(optW).Padding(0, 1)

	if m.cfgLangIndex == 0 {
		enPrefix = "▸ "
		enStyle = enStyle.Background(colHighlight).Foreground(colGreen).Bold(true)
		ruStyle = ruStyle.Background(colPanelBg).Foreground(colMuted)
	} else {
		ruPrefix = "▸ "
		enStyle = enStyle.Background(colPanelBg).Foreground(colMuted)
		ruStyle = ruStyle.Background(colHighlight).Foreground(colGreen).Bold(true)
	}

	btnEn := enStyle.Render(enPrefix + "[1] En — English (Default)")
	btnRu := ruStyle.Render(ruPrefix + "[2] Ru — Русский (Russian)")
	rows = append(rows, btnEn, btnRu, "")

	infoLines := []string{
		m.T("cfg_modal_info_model", m.cfg.ActiveModelLabel()),
		m.T("cfg_modal_info_period", m.periodSpec),
		m.T("cfg_modal_info_skill", m.activeSkill),
		m.T("cfg_modal_info_skills", m.cfg.SkillsDir()),
		m.T("cfg_modal_info_export", m.cfg.ExportsDir),
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
		Width(innerW - 2).
		Render(strings.Join(wrappedInfo, "\n"))

	rows = append(rows, infoBox, "")

	btnW := (innerW - 5) / 2
	if btnW < 24 {
		btnW = 24
	}
	btnToggle := lipgloss.NewStyle().Background(colBlue).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("cfg_modal_btn_toggle"))
	btnSave := lipgloss.NewStyle().Background(colGreen).Foreground(colBg).Bold(true).Width(btnW).Align(lipgloss.Center).Render(m.T("cfg_modal_btn_save"))
	btnEsc := lipgloss.NewStyle().Background(colBorder).Foreground(colText).Width(btnW).Align(lipgloss.Center).Render(m.T("cfg_modal_btn_cancel"))

	btnRow1 := lipgloss.JoinHorizontal(lipgloss.Center, btnToggle, "   ", btnSave)
	btnRow2 := lipgloss.JoinHorizontal(lipgloss.Center, btnEsc)
	rows = append(rows, btnRow1, "", btnRow2, "")

	hint := lipgloss.NewStyle().Foreground(colMuted).Align(lipgloss.Center).Width(innerW).Render(m.T("cfg_modal_hint"))
	rows = append(rows, hint)

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(rows, "\n"))
}

func (m *Model) renderHelpModal() string {
	modalW := 78
	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("help_modal_title")), "")

	if m.cfg.IsRussian() {
		lines = append(lines,
			"  1 - 4               : Вкладки (1:Чат, 2:ИИ и навыки, 3:Архив периода, 4:Логи)",
			"  Tab / h / l         : Переключение фокуса между списком чатов и главным окном",
			"  j / k или Вверх/Вниз: Перемещение курсора / прокрутка активного окна",
			"  g / G, Ctrl+d/Ctrl+u: Быстрая навигация по списку сообщений или архиву",
			"  Enter / Space (Чаты): Открыть выбранный чат (только 1 активный чат)",
			"  i / Enter (Вкл. 1)  : Написать сообщение в чат (Enter отправить, Esc отмена)",
			"  v (Вкл. 1 / 2 / 3)  : Прослушать голос / открыть медиа (QuickLook) / навык",
			"  Enter (Вкл. 2)      : Включить выбранный ИИ-навык для текущего чата ([AI])",
			"  x (Вкл. 2)          : Отключить ИИ-инструкции для текущего чата",
			"  m (Вкл. 2 / 3)      : Изменить персональные ИИ-заметки для текущего чата",
			"  e / n (Вкл. 2)      : Редактировать навык в $EDITOR (e) или создать новый (n)",
			"  r (Вкл. 1 / 2 / 3)  : Обновить чат (Вкл. 1) или запустить ИИ-анализ (Вкл. 2/3)",
			"  p / [ / ] (Вкл. 3)  : Период анализа (p) и переключение архивов чата ([ / ])",
			"  o / d (Вкл. 3)      : Открыть архив в Finder (o) или удалить архив (d)",
			"  c / C / L           : ИИ-движок (c), Настройки языка (C), Вход по QR (L)",
			"  q / Esc             : Выход или закрытие модального окна",
		)
	} else {
		lines = append(lines,
			"  1 - 4               : Switch tabs (1:Live Chat, 2:AI & Skills, 3:Period Archive, 4:Trace)",
			"  Tab / h / l         : Switch focus between Chats sidebar and Main window",
			"  j / k or Up/Dn      : Move cursor down / up or scroll active view",
			"  g / G, Ctrl+d/Ctrl+u: Jump to top/bottom or half-page scroll",
			"  Enter / Space (Chat): Select & open chat in Tab 1 (single-chat selection)",
			"  i / Enter (Tab 1)   : Compose message in live chat (Enter to send, Esc to cancel)",
			"  v (Tab 1 / 2 / 3)   : Play voice / QuickLook media on-demand / Preview skill",
			"  Enter (Tab 2)       : Enable highlighted summary skill for active chat ([AI])",
			"  x (Tab 2)           : Disable/clear AI instructions for active chat",
			"  m (Tab 2 / 3)       : Edit per-chat AI notes / instructions",
			"  e / n (Tab 2)       : Edit skill in $EDITOR (e) or create new skill (n)",
			"  r (Tab 1 / 2 / 3)   : Refresh live chat (Tab 1) or Run AI period parse (Tab 2/3)",
			"  p / [ / ] (Tab 3)   : Set period window (p) & cycle chat's saved archives ([ / ])",
			"  o / d (Tab 3)       : Open archive in Finder (o) or delete archive (d)",
			"  c / C / L           : AI Engine config (c), Global settings (C), QR Login (L)",
			"  q / Esc             : Quit or close modal",
		)
	}
	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(lines, "\n"))
}

func padRightVisual(s string, width int) string {
	w := lipgloss.Width(s)
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

func sanitizeTerminalText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\uFE0F' || r == '\uFE0E' || r == '\u200D' || (r >= 0x1F1E6 && r <= 0x1F1FF) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func marqueeSingleLine(text string, width int, tick int) string {
	clean := strings.ReplaceAll(text, "\n", " ")
	if lipgloss.Width(clean) <= width {
		return clean + strings.Repeat(" ", width-lipgloss.Width(clean))
	}
	runes := []rune(clean)
	sep := []rune("   •   ")
	loop := append(append([]rune{}, runes...), sep...)
	offset := (tick / 2) % len(loop)
	doubled := append(loop, loop...)

	var out []rune
	curW := 0
	for _, r := range doubled[offset:] {
		rw := lipgloss.Width(string(r))
		if curW+rw > width {
			break
		}
		out = append(out, r)
		curW += rw
	}
	if curW < width {
		return string(out) + strings.Repeat(" ", width-curW)
	}
	return string(out)
}

func truncateSingleLine(text string, maxW int) string {
	clean := strings.ReplaceAll(text, "\n", " ")
	if lipgloss.Width(clean) <= maxW {
		return clean
	}
	runes := []rune(clean)
	if maxW <= 2 {
		return string(runes[:maxInt(0, maxW)])
	}
	for len(runes) > 0 && lipgloss.Width(string(runes))+2 > maxW {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ".."
}

func padOrTruncate(text string, width int) string {
	t := truncateSingleLine(text, width)
	w := lipgloss.Width(t)
	if w < width {
		return t + strings.Repeat(" ", width-w)
	}
	return t
}

func wrapTextBlock(text string, width int) []string {
	if width < 8 {
		width = 8
	}
	var result []string
	paragraphs := strings.Split(text, "\n")
	for _, para := range paragraphs {
		para = strings.TrimRight(para, "\r")
		if strings.TrimSpace(para) == "" {
			result = append(result, "")
			continue
		}
		// Preserve leading indentation (e.g. "    " 4-space indented list items)
		leadSpaces := 0
		for _, ch := range para {
			if ch == ' ' {
				leadSpaces++
			} else {
				break
			}
		}
		if leadSpaces > width/2 {
			leadSpaces = 0
		}
		indentPrefix := strings.Repeat(" ", leadSpaces)
		contIndent := indentPrefix

		words := strings.Fields(para)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}
		cur := indentPrefix + words[0]
		for _, w := range words[1:] {
			if lipgloss.Width(cur)+1+lipgloss.Width(w) <= width {
				cur += " " + w
			} else {
				for lipgloss.Width(cur) > width {
					r := []rune(cur)
					cut := minInt(len(r), width)
					result = append(result, string(r[:cut]))
					cur = contIndent + string(r[cut:])
				}
				result = append(result, cur)
				cur = contIndent + w
			}
		}
		for lipgloss.Width(cur) > width {
			r := []rune(cur)
			cut := minInt(len(r), width)
			result = append(result, string(r[:cut]))
			cur = contIndent + string(r[cut:])
		}
		result = append(result, cur)
	}
	if len(result) == 0 {
		return []string{""}
	}
	return result
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
