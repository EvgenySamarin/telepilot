/*
File: internal/tui/app.go
Description: Core Bubble Tea Model struct, NewModel constructor, Init, main Update
event loop, and screenshot/capture state helpers.
*/

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

	"telepilot/internal/ai"
	"telepilot/internal/config"
	"telepilot/internal/i18n"
	"telepilot/internal/skills"
	"telepilot/internal/storage"
)

/*
AIEngine defines the AI operations required by the TUI, decoupling it from the concrete CLI engine.
*/
type AIEngine interface {
	GenerateChatReply(ctx context.Context, chatTitle string, recent []storage.LiveMessage, skillName string, customNotes string) (string, error)
	TestConnection(ctx context.Context) (bool, string)
	PlanSingleShotInstruction(ctx context.Context, chatTitle string, recent []storage.LiveMessage, instruction string) ([]ai.SingleShotAction, error)
	GenerateScheduledTaskMessage(ctx context.Context, chatTitle string, recent []storage.LiveMessage, taskPrompt string) (string, error)
	ExecuteChatSkill(ctx context.Context, chat storage.ChatInfo, skillName string, customNotes string, recent []storage.LiveMessage) (storage.SkillOutputRecord, error)
}

/*
TelegramClient defines the Telegram operations required by the TUI, decoupling it from MTProto details.
*/
type TelegramClient interface {
	StartLiveStream(ctx context.Context, onNewMessage func(chatID int64, msg storage.LiveMessage))
	CheckAuthorized(ctx context.Context) (bool, string)
	ListDialogs(ctx context.Context, limit int) ([]storage.ChatInfo, error)
	FetchRecentMessages(ctx context.Context, chat storage.ChatInfo, limit int) ([]storage.LiveMessage, error)
	SendMessage(ctx context.Context, chat storage.ChatInfo, text string) (storage.LiveMessage, error)
	DownloadMessageMedia(ctx context.Context, chat storage.ChatInfo, msg storage.LiveMessage) (string, error)
	TestConnection(ctx context.Context, onUpdate func(lines []string)) (bool, []string)
	RunQRLogin(ctx context.Context, onQR func(qrArt, loginURL, pngPath string), onStatus func(status string)) (string, error)
}

type Model struct {
	cfg     *config.AppConfig
	ai      AIEngine
	mtproto TelegramClient

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

	// Background scheduled AI tasks (persisted in data/scheduled_tasks.json)
	scheduledTasks []storage.ScheduledTask

	// Track sent message IDs to eliminate auto-reply self-echoes
	sentMsgIDs map[int]bool

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
	aiTesting     bool
	aiPassed      bool
	aiStatusLine  string
	aiQuotaStatus string

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
	cfgBoxCursor    int  // 0: Language, 1: Favourite Chats, 2: Hidden Chats
	cfgBoxEntered   bool // true when user pressed Enter to drill into a box
	cfgLangIndex    int  // 0: En, 1: Ru
	cfgFavCursor    int  // selected index in favoriteChats list
	cfgHiddenCursor int  // selected index in hiddenChats list

	// Raw chats loaded from Telegram/Demo before filtering
	rawChats []storage.ChatInfo

	// Favourite / pinned chats (persisted in data/favorite_chats.json)
	favoriteChats   []storage.FavoriteChatRecord
	favoriteChatIDs map[int64]bool

	// Hidden chats (persisted in data/hidden_chats.json)
	hiddenChats   []storage.HiddenChatRecord
	hiddenChatIDs map[int64]bool
	hiddenMu      sync.RWMutex
}

/*
T translates a localization key into the active UI language with optional formatting arguments.
*/
func (m *Model) T(key string, args ...any) string {
	return i18n.T(m.cfg.Language, key, args...)
}

/*
NewModel initializes the TUI Model with loaded configs, skills, chats, and runtime state.
*/
func NewModel(cfg *config.AppConfig, aiEngine AIEngine, mtSvc TelegramClient, forceDemo bool) *Model {
	rawChats := make([]storage.ChatInfo, len(storage.DemoChats))
	copy(rawChats, storage.DemoChats)

	var initWarnings []string
	favChats, err := storage.LoadFavoriteChats(cfg.DataDir)
	if err != nil {
		initWarnings = append(initWarnings, fmt.Sprintf("Failed to load favorite chats: %v", err))
	}
	favIDs := make(map[int64]bool, len(favChats))
	for _, f := range favChats {
		favIDs[f.ChatID] = true
	}

	hiddenChats, err := storage.LoadHiddenChats(cfg.DataDir)
	if err != nil {
		initWarnings = append(initWarnings, fmt.Sprintf("Failed to load hidden chats: %v", err))
	}
	hiddenIDs := make(map[int64]bool, len(hiddenChats))
	for _, h := range hiddenChats {
		hiddenIDs[h.ChatID] = true
	}

	var visibleChats []storage.ChatInfo
	for _, c := range rawChats {
		if !hiddenIDs[c.ChatID] {
			visibleChats = append(visibleChats, c)
		}
	}
	chats := reorderChatsWithPinnedList(visibleChats, favIDs, favChats)

	var selChatID int64
	var initLiveMsgs []storage.LiveMessage
	if len(chats) > 0 {
		selChatID = chats[0].ChatID
		initLiveMsgs = storage.DemoLiveMessages(chats[0])
	}

	chatAICfgs, err := storage.LoadChatAIConfigs(cfg.DataDir)
	if err != nil {
		initWarnings = append(initWarnings, fmt.Sprintf("Failed to load chat AI configs: %v", err))
	}
	skillOutputs, err := storage.LoadChatSkillOutputs(cfg.DataDir)
	if err != nil {
		initWarnings = append(initWarnings, fmt.Sprintf("Failed to load skill outputs: %v", err))
	}
	schedTasks, err := storage.LoadScheduledTasks(cfg.DataDir)
	if err != nil {
		initWarnings = append(initWarnings, fmt.Sprintf("Failed to load scheduled tasks: %v", err))
	}
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
		scheduledTasks:    schedTasks,
		sentMsgIDs:        make(map[int]bool),
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
		rawChats:          rawChats,
		hiddenChats:       hiddenChats,
		hiddenChatIDs:     hiddenIDs,
		cfgHiddenCursor:   0,
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
		aiStatusLine:  initialAIStatusLine(cfg),
		aiQuotaStatus: initialAIQuotaStatus(cfg),
	}

	_ = skills.EnsureDefaults(cfg.SkillsDir())
	m.reloadSkills()
	m.syncCurrentChatAISettings()

	m.addTrace(fmt.Sprintf("TelePilot Go TUI initialized (root=%s, model=%s)", cfg.ProjectRoot, cfg.ActiveModelLabel()))
	m.addTrace(fmt.Sprintf("Configured MTProto server: %s (DC %d, api_id=%d)", serverStr, cfg.TelegramDCID, cfg.TelegramAPIID))
	for _, w := range initWarnings {
		m.addTrace(w)
	}
	return m
}

/*
Init starts the Bubble Tea background ticks, UI event listener, auth check, and live Telegram stream.
*/
func (m *Model) Init() tea.Cmd {
	return tea.Batch(
		tickCmd(),
		liveSyncTickCmd(),
		waitForUIEvent(m.uiEventCh),
		m.checkInitialAuthCmd(),
		m.startLiveStreamCmd(),
	)
}

/*
Update handles all incoming Bubble Tea messages, timers, async results, and keyboard events.
*/
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
		var cmds []tea.Cmd
		cmds = append(cmds, tickCmd())
		if schedCmds := m.checkScheduledTasksCmd(); len(schedCmds) > 0 {
			cmds = append(cmds, schedCmds...)
		}
		return m, tea.Batch(cmds...)

	case liveSyncTickMsg:
		var cmds []tea.Cmd
		cmds = append(cmds, liveSyncTickCmd())
		if !m.statusBusy {
			if ch := m.activeChat(); ch != nil && !m.isChatHidden(ch.ChatID) {
				cmds = append(cmds, m.loadChatMessagesCmd(*ch))
			}
		}
		return m, tea.Batch(cmds...)

	case liveMessagesLoadedMsg:
		if m.isChatHidden(msg.chatID) {
			return m, nil
		}
		if msg.err == nil && msg.chatID == m.selectedChatID && len(msg.messages) > 0 {
			wasAtBottom := m.liveMsgCursor >= len(m.liveMessages)-1
			m.liveMessages = msg.messages
			m.lastLiveSync = time.Now()
			if wasAtBottom || m.liveMsgCursor >= len(m.liveMessages) {
				m.liveMsgCursor = maxInt(0, len(m.liveMessages)-1)
			}
			// Check if Auto-Reply should trigger on a new incoming message at the end of the chat
			last := m.liveMessages[len(m.liveMessages)-1]
			isSelfMsg := last.IsOutgoing || strings.EqualFold(strings.TrimSpace(last.Sender), "You") || m.sentMsgIDs[last.MessageID]
			if cfg, ok := m.chatAIConfigs[msg.chatID]; ok && cfg.Enabled && cfg.AutoReply {
				if isSelfMsg {
					if last.MessageID > cfg.LastAutoRepliedMsgID {
						cfg.LastAutoRepliedMsgID = last.MessageID
						m.chatAIConfigs[msg.chatID] = cfg
						_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
					}
				} else if cfg.LastAutoRepliedMsgID == 0 {
					cfg.LastAutoRepliedMsgID = last.MessageID
					m.chatAIConfigs[msg.chatID] = cfg
					_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
				} else if last.MessageID > cfg.LastAutoRepliedMsgID {
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
		if m.isChatHidden(msg.chatID) {
			return m, waitForUIEvent(m.uiEventCh)
		}
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

				isIncomingSelf := msg.msg.IsOutgoing || strings.EqualFold(strings.TrimSpace(msg.msg.Sender), "You") || m.sentMsgIDs[msg.msg.MessageID]
				if cfg, ok := m.chatAIConfigs[msg.chatID]; ok && cfg.Enabled && cfg.AutoReply {
					if isIncomingSelf {
						if msg.msg.MessageID > cfg.LastAutoRepliedMsgID {
							cfg.LastAutoRepliedMsgID = msg.msg.MessageID
							m.chatAIConfigs[msg.chatID] = cfg
							_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
						}
					} else if msg.msg.MessageID > cfg.LastAutoRepliedMsgID {
						cfg.LastAutoRepliedMsgID = msg.msg.MessageID
						m.chatAIConfigs[msg.chatID] = cfg
						_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
						if ch := m.activeChat(); ch != nil && ch.ChatID == msg.chatID {
							m.addTrace(fmt.Sprintf("Auto-Reply triggered for live incoming message #%d in '%s'", msg.msg.MessageID, ch.Title))
							autoReplyCmd = m.generateAIChatReplyCmd(*ch, m.liveMessages, cfg.SendMode)
						}
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
		m.updateAIQuotaStatus(msg.err)
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

		cleanReply, directive, found := ai.ExtractScheduleDirective(reply, time.Now())
		if found && directive != nil {
			isDuplicate := false
			for _, existing := range m.scheduledTasks {
				if existing.ChatID == msg.chat.ChatID && strings.EqualFold(strings.TrimSpace(existing.TaskPrompt), strings.TrimSpace(directive.TaskPrompt)) {
					isDuplicate = true
					break
				}
			}
			if !isDuplicate {
				task := storage.ScheduledTask{
					ID:          fmt.Sprintf("task_%d", time.Now().UnixNano()),
					ChatID:      msg.chat.ChatID,
					ChatTitle:   msg.chat.Title,
					TaskPrompt:  directive.TaskPrompt,
					DueAt:       directive.DueAt,
					IntervalStr: directive.RepeatStr,
					MaxRuns:     directive.MaxRuns,
					RunsCount:   0,
					Until:       directive.Until,
					CreatedAt:   time.Now(),
				}
				m.scheduledTasks = append(m.scheduledTasks, task)
				_ = storage.SaveScheduledTasks(m.cfg.DataDir, m.scheduledTasks)
				logTime := directive.InStr
				if directive.AtStr != "" {
					logTime = "@" + directive.AtStr
				}
				m.addTrace(fmt.Sprintf("⏰ Task scheduled for '%s' (%s): %s", msg.chat.Title, logTime, directive.TaskPrompt))
			}

			cleanTrim := strings.TrimSpace(cleanReply)
			if cleanTrim == "" || isPureConfirmation(cleanTrim) {
				m.statusText = fmt.Sprintf("⏰ Scheduled task set for '%s'", msg.chat.Title)
				m.chatInputBuffer = ""
				m.chatComposing = false
				return m, nil
			}
		}
		reply = cleanReply

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

	case scheduleDirectiveFoundMsg:
		isDuplicate := false
		for _, existing := range m.scheduledTasks {
			if existing.ChatID == msg.chatID && strings.EqualFold(strings.TrimSpace(existing.TaskPrompt), strings.TrimSpace(msg.directive.TaskPrompt)) {
				isDuplicate = true
				break
			}
		}
		if !isDuplicate {
			task := storage.ScheduledTask{
				ID:          fmt.Sprintf("task_%d", time.Now().UnixNano()),
				ChatID:      msg.chatID,
				ChatTitle:   msg.chatTitle,
				TaskPrompt:  msg.directive.TaskPrompt,
				DueAt:       msg.directive.DueAt,
				IntervalStr: msg.directive.RepeatStr,
				MaxRuns:     msg.directive.MaxRuns,
				RunsCount:   0,
				Until:       msg.directive.Until,
				CreatedAt:   time.Now(),
			}
			m.scheduledTasks = append(m.scheduledTasks, task)
			_ = storage.SaveScheduledTasks(m.cfg.DataDir, m.scheduledTasks)
			logTime := msg.directive.InStr
			if msg.directive.AtStr != "" {
				logTime = "@" + msg.directive.AtStr
			}
			m.addTrace(fmt.Sprintf("⏰ Task scheduled for '%s' (%s): %s", msg.chatTitle, logTime, msg.directive.TaskPrompt))
		}
		return m, nil

	case scheduledTaskExecutedMsg:
		m.updateAIQuotaStatus(msg.err)
		if msg.err != nil {
			m.addTrace(fmt.Sprintf("⏰ Scheduled task failed for '%s': %v", msg.task.ChatTitle, msg.err))
		} else {
			if msg.sentMsg.MessageID > 0 {
				m.sentMsgIDs[msg.sentMsg.MessageID] = true
				if cfg, ok := m.chatAIConfigs[msg.task.ChatID]; ok {
					if msg.sentMsg.MessageID > cfg.LastAutoRepliedMsgID {
						cfg.LastAutoRepliedMsgID = msg.sentMsg.MessageID
						m.chatAIConfigs[msg.task.ChatID] = cfg
						_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
					}
				}
			}
			m.addTrace(fmt.Sprintf("⏰ Executed scheduled task for '%s': %s", msg.task.ChatTitle, truncateSingleLine(msg.text, 60)))
			if msg.task.ChatID == m.selectedChatID {
				m.liveMessages = append(m.liveMessages, msg.sentMsg)
				m.liveMsgCursor = len(m.liveMessages) - 1
				m.lastLiveSync = time.Now()
			}
		}
		return m, nil

	case liveMessageSentMsg:
		if msg.err != nil {
			m.statusText = "Send failed"
			m.addTrace(fmt.Sprintf("Failed to send message: %v", msg.err))
			return m, waitForUIEvent(m.uiEventCh)
		}
		if msg.msg.MessageID > 0 {
			m.sentMsgIDs[msg.msg.MessageID] = true
			if cfg, ok := m.chatAIConfigs[msg.chatID]; ok {
				if msg.msg.MessageID > cfg.LastAutoRepliedMsgID {
					cfg.LastAutoRepliedMsgID = msg.msg.MessageID
					m.chatAIConfigs[msg.chatID] = cfg
					_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
				}
			}
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
		if msg.ok {
			m.updateAIQuotaStatus(nil)
		} else {
			m.updateAIQuotaStatus(fmt.Errorf("%s", msg.line))
		}
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
			m.rawChats = make([]storage.ChatInfo, len(msg.chats))
			copy(m.rawChats, msg.chats)
			m.refreshVisibleChats()
			if len(m.chats) > 0 {
				m.chatCursor = 0
				m.selectedChatID = m.chats[0].ChatID
				m.syncCurrentChatAISettings()
				m.statusText = "Live MTProto"
				m.addTrace(fmt.Sprintf("Loaded %d live dialogs from Telegram (%d hidden).", len(m.rawChats), len(m.hiddenChats)))
				return m, m.loadChatMessagesCmd(m.chats[0])
			}
			m.chatCursor = 0
			m.selectedChatID = 0
			m.statusText = "Live MTProto (all chats hidden)"
			return m, nil
		}
		return m, nil

	case parseLogMsg:
		m.addTrace(string(msg))
		m.statusText = string(msg)
		return m, waitForUIEvent(m.uiEventCh)

	case aiQuotaUpdatedMsg:
		m.updateAIQuotaStatus(msg.err)
		return m, waitForUIEvent(m.uiEventCh)

	case parseDoneMsg:
		m.statusBusy = false
		m.updateAIQuotaStatus(msg.err)
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

/*
SetQRModalForCapture sets the model state to show the QR modal for captures and previews.
*/
func (m *Model) SetQRModalForCapture(status, qrArt, qrPNGPath string) {
	m.activeModal = modalLoginQR
	m.qrStatus = status
	m.qrArt = qrArt
	m.qrPNGPath = qrPNGPath
}

/*
SetGlobalConfigModalForCapture configures the model to display the Global Configuration modal.
*/
func (m *Model) SetGlobalConfigModalForCapture(langIdx int) {
	m.activeModal = modalGlobalConfig
	m.cfgLangIndex = langIdx
}

/*
SetSkillPreviewModalForCapture configures the model to display the Skill Preview modal.
*/
func (m *Model) SetSkillPreviewModalForCapture(skillIdx int) {
	m.activeTab = 2
	if skillIdx >= 0 && skillIdx < len(m.skillList) {
		m.skillCursor = skillIdx
	}
	m.previewScroll = 0
	m.activeModal = modalSkillPreview
}
