package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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
	focusPeriods
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

	focus      focusArea
	activeTab  int // 1..5
	activeModal modalKind

	chats            []storage.ChatInfo
	selectedChats    map[int64]bool
	chatCursor       int
	chatScrollOffset int

	periods            []storage.PeriodSummaryRecord
	periodCursor       int
	periodScrollOffset int
	mediaCursor        int

	mainScroll int

	statusText  string
	statusBusy  bool
	authUser    string
	periodSpec  string
	runNotes    string // extra instructions appended to the active summary skill
	jobDone     int
	jobTotal    int
	startTime   time.Time
	marqueeTick int

	traceLogs []string

	// Login / Connection Test Modal State
	formFields  [4]string // 0: API ID, 1: API Hash, 2: Server IP:Port, 3: Gemini Key
	formFocus   int
	connTesting bool
	connPassed  bool
	connLines   []string

	qrArt      string
	qrURL      string
	qrPNGPath  string
	qrStatus   string
	qrCancel   context.CancelFunc
	uiEventCh  chan tea.Msg

	// Period & Prompt modal input buffers
	inputBuffer string

	// Delete confirmation modal button focus (0 = Confirm Delete, 1 = Cancel)
	deleteBtnFocus int

	// AI Engine Configuration modal state
	aiFormFields [2]string // 0: Gemini API Key, 1: Gemini Model
	aiFormFocus  int
	aiTesting    bool
	aiPassed     bool
	aiStatusLine string

	// Skills (Re-Prompt tab): loaded from <project>/.agents/skills
	skillList         []skills.Skill
	skillErrs         []error
	skillCursor       int
	skillScrollOffset int
	activeSkill       string // summary skill used for parsing & re-runs
	previewScroll     int

	// Global Configuration modal state (Language selection: 0 = En, 1 = Ru)
	cfgLangIndex int
}

func (m *Model) T(key string, args ...any) string {
	return i18n.T(m.cfg.Language, key, args...)
}

func NewModel(cfg *config.AppConfig, aiEngine *ai.Engine, mtSvc *mtproto.Service, forceDemo bool) *Model {
	var periods []storage.PeriodSummaryRecord
	if forceDemo {
		periods = storage.EnsureDemoPeriodFolders(cfg)
	} else {
		periods = storage.DiscoverAllPeriodFolders(cfg.ExportsDir)
	}
	chats := make([]storage.ChatInfo, len(storage.DemoChats))
	copy(chats, storage.DemoChats)

	selected := make(map[int64]bool)
	if len(chats) > 0 {
		selected[chats[0].ChatID] = true
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
		cfg:           cfg,
		ai:            aiEngine,
		mtproto:       mtSvc,
		width:         140,
		height:        38,
		focus:         focusChats,
		activeTab:     1,
		activeModal:   initModal,
		chats:         chats,
		selectedChats: selected,
		periods:       periods,
		statusText:    i18n.T(cfg.Language, "status_ready"),
		periodSpec:    cfg.DefaultPeriod,
		runNotes:      cfg.DefaultNotes,
		activeSkill:   cfg.DefaultSkill,
		cfgLangIndex:  initLangIdx,
		jobDone:       1,
		jobTotal:      len(chats),
		startTime:     time.Now(),
		uiEventCh:     make(chan tea.Msg, 64),
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

	m.addTrace(fmt.Sprintf("TelePilot Go TUI initialized (root=%s, model=%s)", cfg.ProjectRoot, cfg.ActiveModelLabel()))
	m.addTrace(fmt.Sprintf("Configured MTProto server: %s (DC %d, api_id=%d)", serverStr, cfg.TelegramDCID, cfg.TelegramAPIID))
	return m
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
		waitForUIEvent(m.uiEventCh),
		m.checkInitialAuthCmd(),
	)
}

func tickCmd() tea.Cmd {
	return tea.Tick(220*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func waitForUIEvent(ch chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-ch
	}
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

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.marqueeTick++
		return m, tickCmd()

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
		return m, tea.Batch(waitForUIEvent(m.uiEventCh), m.loadDialogsCmd())

	case dialogsLoadedMsg:
		if msg.err == nil && len(msg.chats) > 0 {
			m.chats = msg.chats
			m.selectedChats = make(map[int64]bool)
			m.selectedChats[m.chats[0].ChatID] = true
			m.chatCursor = 0
			m.jobTotal = len(m.chats)
			m.statusText = "Live MTProto"
			m.addTrace(fmt.Sprintf("Loaded %d live dialogs from Telegram.", len(m.chats)))
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
			m.addTrace(fmt.Sprintf("Batch parse failed: %v", msg.err))
		} else {
			m.periods = storage.DiscoverAllPeriodFolders(m.cfg.ExportsDir)
			m.periodCursor = 0
			if len(msg.records) > 0 && msg.records[0] != nil {
				for idx, p := range m.periods {
					if p.FolderPath == msg.records[0].FolderPath {
						m.periodCursor = idx
						break
					}
				}
			}
			m.statusText = m.T("parsed_chats", len(msg.records))
			m.addTrace(fmt.Sprintf("Completed period summarization for %d chats.", len(msg.records)))
		}
		return m, waitForUIEvent(m.uiEventCh)

	case tea.KeyMsg:
		if m.activeModal != modalNone {
			return m.handleModalKey(msg)
		}
		return m.handleMainKey(msg)
	}

	return m, nil
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
			m.addTrace("Updated active period window to: " + val)
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
			val := strings.TrimSpace(m.inputBuffer)
			m.runNotes = val
			m.cfg.SaveSkillSettings(m.activeSkill, m.runNotes)
			if rec := m.activePeriodRecord(); rec != nil {
				rec.Prompt = val
				_ = storage.SavePeriodRun(*rec)
			}
			m.addTrace("Updated extra run notes for summarization.")
			m.activeModal = modalNone
		case "ctrl+r":
			val := strings.TrimSpace(m.inputBuffer)
			m.runNotes = val
			m.cfg.SaveSkillSettings(m.activeSkill, m.runNotes)
			m.activeModal = modalNone
			return m, m.rePromptActivePeriodCmd()
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
			// Only trigger shortcut 't' if user isn't typing into API Hash or Gemini Key (or allow Enter / Ctrl+T anytime, and 't' when in numeric fields 0 or 2)
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
			// Switch cleanly back to Credentials & Connection Test Form View!
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
	case "q", "ctrl+c":
		if m.qrCancel != nil {
			m.qrCancel()
		}
		return m, tea.Quit

	case "tab":
		switch m.focus {
		case focusChats:
			m.focus = focusPeriods
		case focusPeriods:
			m.focus = focusMain
		default:
			m.focus = focusChats
		}
		return m, nil

	case "shift+tab":
		switch m.focus {
		case focusChats:
			m.focus = focusMain
		case focusPeriods:
			m.focus = focusChats
		default:
			m.focus = focusPeriods
		}
		return m, nil

	case "1":
		m.activeTab = 1
		m.mainScroll = 0
	case "2":
		m.activeTab = 2
		m.mainScroll = 0
	case "3":
		m.activeTab = 3
		m.mainScroll = 0
	case "4":
		m.activeTab = 4
		m.mainScroll = 0
	case "5":
		m.activeTab = 5
		m.mainScroll = 0

	case "L":
		m.activeModal = modalLoginForm
		m.formFocus = 2
		return m, m.startConnectionTestCmd()

	case "h", "left":
		if m.focus == focusMain {
			m.focus = focusPeriods
		} else if m.focus == focusPeriods {
			m.focus = focusChats
		}

	case "l", "right":
		if m.focus == focusChats {
			m.focus = focusPeriods
		} else if m.focus == focusPeriods {
			m.focus = focusMain
		}

	case "g":
		switch m.focus {
		case focusChats:
			m.chatCursor = 0
			m.marqueeTick = 0
		case focusPeriods:
			m.periodCursor = 0
			m.mediaCursor = 0
			m.mainScroll = 0
			m.marqueeTick = 0
		case focusMain:
			if m.activeTab == 2 {
				m.mediaCursor = 0
			} else if m.activeTab == 4 {
				m.skillCursor = 0
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
		case focusPeriods:
			if len(m.periods) > 0 {
				m.periodCursor = len(m.periods) - 1
				m.mediaCursor = 0
				m.mainScroll = 0
				m.marqueeTick = 0
			}
		case focusMain:
			if m.activeTab == 2 {
				rec := m.activePeriodRecord()
				if rec != nil && len(rec.MediaItems) > 0 {
					m.mediaCursor = len(rec.MediaItems) - 1
				}
			} else {
				if m.activeTab == 4 && len(m.skillList) > 0 {
					m.skillCursor = len(m.skillList) - 1
				}
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
		case focusPeriods:
			if len(m.periods) > 0 {
				m.periodCursor = minInt(len(m.periods)-1, m.periodCursor+4)
				m.mediaCursor = 0
				m.mainScroll = 0
				m.marqueeTick = 0
			}
		case focusMain:
			if m.activeTab == 2 {
				rec := m.activePeriodRecord()
				if rec != nil && len(rec.MediaItems) > 0 {
					m.mediaCursor = minInt(len(rec.MediaItems)-1, m.mediaCursor+3)
				}
			} else {
				m.mainScroll += 8
			}
		}

	case "ctrl+u":
		switch m.focus {
		case focusChats:
			m.chatCursor = maxInt(0, m.chatCursor-6)
			m.marqueeTick = 0
		case focusPeriods:
			m.periodCursor = maxInt(0, m.periodCursor-4)
			m.mediaCursor = 0
			m.mainScroll = 0
			m.marqueeTick = 0
		case focusMain:
			if m.activeTab == 2 {
				m.mediaCursor = maxInt(0, m.mediaCursor-3)
			} else {
				m.mainScroll = maxInt(0, m.mainScroll-8)
			}
		}

	case "[", "k", "up":
		if msg.String() == "[" {
			if m.mainScroll > 0 {
				m.mainScroll--
			}
			return m, nil
		}
		switch m.focus {
		case focusChats:
			if m.chatCursor > 0 {
				m.chatCursor--
				m.marqueeTick = 0
			}
		case focusPeriods:
			if m.periodCursor > 0 {
				m.periodCursor--
				m.mediaCursor = 0
				m.mainScroll = 0
				m.marqueeTick = 0
			}
		case focusMain:
			if m.activeTab == 2 {
				if m.mediaCursor > 0 {
					m.mediaCursor--
				}
			} else if m.activeTab == 4 {
				targetLine := 9 + m.skillCursor
				if m.mainScroll > targetLine {
					m.mainScroll--
				} else if m.skillCursor > 0 {
					m.skillCursor--
					newTarget := 9 + m.skillCursor
					if newTarget < m.mainScroll+1 {
						m.mainScroll = maxInt(0, newTarget-1)
					}
				} else if m.mainScroll > 0 {
					m.mainScroll--
				}
			} else if m.mainScroll > 0 {
				m.mainScroll--
			}
		}

	case "]", "j", "down":
		if msg.String() == "]" {
			m.mainScroll++
			return m, nil
		}
		switch m.focus {
		case focusChats:
			if m.chatCursor < len(m.chats)-1 {
				m.chatCursor++
				m.marqueeTick = 0
			} else if len(m.periods) > 0 {
				m.focus = focusPeriods
				m.marqueeTick = 0
			}
		case focusPeriods:
			if m.periodCursor < len(m.periods)-1 {
				m.periodCursor++
				m.mediaCursor = 0
				m.mainScroll = 0
				m.marqueeTick = 0
			}
		case focusMain:
			if m.activeTab == 2 {
				rec := m.activePeriodRecord()
				if rec != nil && m.mediaCursor < len(rec.MediaItems)-1 {
					m.mediaCursor++
				}
			} else if m.activeTab == 4 {
				visibleRows := maxInt(1, m.height-8)
				if m.skillCursor < len(m.skillList)-1 {
					m.skillCursor++
					targetLine := 9 + m.skillCursor
					if targetLine >= m.mainScroll+visibleRows-2 {
						m.mainScroll = targetLine - visibleRows + 3
					}
				} else {
					m.mainScroll++
				}
			} else {
				m.mainScroll++
			}
		}

	case "J":
		m.mainScroll += 4

	case "K":
		m.mainScroll = maxInt(0, m.mainScroll-4)

	case "d", "x", "backspace", "delete":
		if rec := m.activePeriodRecord(); rec != nil {
			m.deleteBtnFocus = 0
			m.activeModal = modalConfirmDelete
		}

	case " ":
		if m.focus == focusChats && m.chatCursor >= 0 && m.chatCursor < len(m.chats) {
			cid := m.chats[m.chatCursor].ChatID
			if m.selectedChats[cid] {
				delete(m.selectedChats, cid)
			} else {
				m.selectedChats[cid] = true
			}
		}

	case "a":
		if m.focus == focusChats {
			if len(m.selectedChats) == len(m.chats) {
				m.selectedChats = make(map[int64]bool)
			} else {
				for _, c := range m.chats {
					m.selectedChats[c.ChatID] = true
				}
			}
		} else {
			m.activeModal = modalPrompt
			if rec := m.activePeriodRecord(); rec != nil && strings.TrimSpace(rec.Prompt) != "" {
				m.inputBuffer = rec.Prompt
			} else {
				m.inputBuffer = m.runNotes
			}
		}

	case "e":
		if len(m.skillList) > 0 {
			if m.skillCursor < 0 || m.skillCursor >= len(m.skillList) {
				m.skillCursor = 0
			}
			sk := m.skillList[m.skillCursor]
			m.addTrace("Opening skill in $EDITOR: " + sk.Path)
			return m, openEditorCmd(sk.Path)
		}

	case "n":
		m.inputBuffer = ""
		m.activeModal = modalNewSkill

	case "p":
		m.activeModal = modalPeriod
		m.inputBuffer = m.periodSpec

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

	case "m":
		m.activeModal = modalPrompt
		if rec := m.activePeriodRecord(); rec != nil && strings.TrimSpace(rec.Prompt) != "" {
			m.inputBuffer = rec.Prompt
		} else {
			m.inputBuffer = m.runNotes
		}

	case "r":
		if !m.statusBusy {
			return m, m.rePromptActivePeriodCmd()
		}

	case "?":
		m.activeModal = modalHelp

	case "s", "enter":
		if m.focus == focusPeriods && msg.String() == "enter" {
			m.focus = focusMain
			return m, nil
		}
		if m.focus == focusMain && m.activeTab == 4 && msg.String() == "enter" {
			if m.skillCursor >= 0 && m.skillCursor < len(m.skillList) {
				sel := m.skillList[m.skillCursor]
				if sel.Kind == skills.KindSummary {
					m.activeSkill = sel.Name
					m.cfg.SaveSkillSettings(m.activeSkill, m.runNotes)
					m.addTrace("Activated summary skill: " + sel.Name)
				} else {
					m.addTrace(fmt.Sprintf("Media skill '%s' covers: %s", sel.Name, strings.Join(sel.Media, ", ")))
				}
			}
			return m, nil
		}
		if !m.statusBusy {
			return m, m.startBatchParseCmd()
		}

	case "o":
		rec := m.activePeriodRecord()
		if rec != nil && rec.FolderPath != "" {
			_ = exec.Command("open", rec.FolderPath).Start()
			m.addTrace("Opened in Finder: " + rec.FolderPath)
		}

	case "v":
		if m.activeTab == 4 {
			if len(m.skillList) > 0 {
				if m.skillCursor < 0 || m.skillCursor >= len(m.skillList) {
					m.skillCursor = 0
				}
				m.previewScroll = 0
				m.activeModal = modalSkillPreview
			}
			return m, nil
		}
		rec := m.activePeriodRecord()
		if rec != nil {
			if m.activeTab == 2 && m.mediaCursor >= 0 && m.mediaCursor < len(rec.MediaItems) {
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

func (m *Model) startBatchParseCmd() tea.Cmd {
	var targets []storage.ChatInfo
	for _, c := range m.chats {
		if m.selectedChats[c.ChatID] {
			targets = append(targets, c)
		}
	}
	if len(targets) == 0 && m.chatCursor >= 0 && m.chatCursor < len(m.chats) {
		targets = append(targets, m.chats[m.chatCursor])
	}
	if len(targets) == 0 {
		return nil
	}

	m.statusBusy = true
	m.jobDone = 0
	m.jobTotal = len(targets)
	m.statusText = fmt.Sprintf("Parsing 0/%d...", len(targets))
	periodSpec := m.periodSpec
	skillName := m.activeSkill
	notes := m.runNotes

	go func() {
		ctx := context.Background()
		var out []*storage.PeriodSummaryRecord
		for idx, chat := range targets {
			m.uiEventCh <- parseLogMsg(fmt.Sprintf("(%d/%d) Parsing %s [%s] via skill '%s'...", idx+1, len(targets), chat.Title, periodSpec, skillName))
			rec, err := m.mtproto.ParseChatPeriod(ctx, chat, periodSpec, skillName, notes, func(logLine string) {
				m.uiEventCh <- parseLogMsg(logLine)
			})
			if err != nil {
				m.uiEventCh <- parseDoneMsg{records: out, err: err}
				return
			}
			out = append(out, rec)
		}
		m.uiEventCh <- parseDoneMsg{records: out, err: nil}
	}()

	return nil
}

func (m *Model) rePromptActivePeriodCmd() tea.Cmd {
	rec := m.activePeriodRecord()
	if rec == nil {
		return nil
	}
	m.statusBusy = true
	m.statusText = "Re-fetching AI..."
	skillName := m.activeSkill
	if skillName == "" {
		skillName = rec.Skill
	}
	if skillName == "" {
		skillName = m.cfg.DefaultSkill
	}
	notes := strings.TrimSpace(m.runNotes)
	if notes == "" {
		notes = rec.Prompt
	}
	recCopy := *rec
	recCopy.Messages = append([]storage.MessageRecord(nil), rec.Messages...)
	recCopy.MediaItems = append([]storage.MediaItemRecord(nil), rec.MediaItems...)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		m.uiEventCh <- parseLogMsg(fmt.Sprintf("Re-running AI fetching (skill=%s, model=%s): %s", skillName, m.cfg.ActiveModelLabel(), recCopy.FolderPath))

		// 1. Transcribe / analyze media items in the selected period folder with media skills
		msgByMediaFile := make(map[string]int, len(recCopy.Messages))
		msgByID := make(map[int]int, len(recCopy.Messages))
		for idx, msg := range recCopy.Messages {
			if msg.MediaFile != "" {
				msgByMediaFile[msg.MediaFile] = idx
			}
			if msg.MessageID != 0 {
				msgByID[msg.MessageID] = idx
			}
		}

		for i := range recCopy.MediaItems {
			it := &recCopy.MediaItems[i]
			if strings.TrimSpace(it.FilePath) == "" {
				continue
			}
			caption := ""
			if mIdx, ok := msgByID[it.MessageID]; ok {
				caption = recCopy.Messages[mIdx].Text
			} else if mIdx, ok := msgByMediaFile[it.FileName]; ok {
				caption = recCopy.Messages[mIdx].Text
			}

			m.uiEventCh <- parseLogMsg(fmt.Sprintf("AI analyzing media (%d/%d): %s...", i+1, len(recCopy.MediaItems), it.FileName))
			res, _ := m.ai.AnalyzeMediaItem(ctx, *it, caption, false)
			it.AIText = res.Text
			it.Skill = res.Skill
			it.SkillHash = res.SkillHash

			if mIdx, ok := msgByID[it.MessageID]; ok {
				if it.MediaType == "voice" {
					recCopy.Messages[mIdx].VoiceTranscript = res.Text
				} else {
					recCopy.Messages[mIdx].MediaAnalysis = res.Text
				}
			} else if mIdx, ok := msgByMediaFile[it.FileName]; ok {
				if it.MediaType == "voice" {
					recCopy.Messages[mIdx].VoiceTranscript = res.Text
				} else {
					recCopy.Messages[mIdx].MediaAnalysis = res.Text
				}
			}
		}

		// 2. Re-synthesize executive period summary using active summary skill & updated media captures
		m.uiEventCh <- parseLogMsg(fmt.Sprintf("Generating AI period summary via skill '%s' (%s)...", skillName, m.cfg.ActiveModelLabel()))
		sum := m.ai.GeneratePeriodSummary(
			ctx,
			recCopy.ChatTitle,
			fmt.Sprintf("%s -> %s (%s)", recCopy.PeriodStart, recCopy.PeriodEnd, recCopy.PeriodSpec),
			skillName,
			notes,
			recCopy.Messages,
			recCopy.MediaItems,
		)
		recCopy.Prompt = notes
		recCopy.Skill = sum.Skill
		recCopy.SkillHash = sum.SkillHash
		recCopy.ModelUsed = m.cfg.ActiveModelLabel()
		recCopy.ExecutiveSummary = sum.ExecutiveSummary
		recCopy.KeyDecisions = sum.KeyDecisions
		recCopy.VoiceMediaInsights = sum.VoiceMediaInsights
		recCopy.ActionItems = sum.ActionItems
		_ = storage.SavePeriodRun(recCopy)
		m.uiEventCh <- parseDoneMsg{records: []*storage.PeriodSummaryRecord{&recCopy}, err: nil}
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
	m.periods = storage.DiscoverAllPeriodFolders(m.cfg.ExportsDir)
	if m.periodCursor >= len(m.periods) {
		m.periodCursor = maxInt(0, len(m.periods)-1)
	}
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
	left := fmt.Sprintf(" %s │ %s: %s │ %s",
		lipgloss.NewStyle().Foreground(statusColor).Bold(true).Render(icon+" "+m.statusText),
		m.T("period_label"),
		lipgloss.NewStyle().Foreground(colGreen).Bold(true).Render(m.periodSpec),
		m.T("selected_chats", len(m.selectedChats)),
	)

	elapsedSec := int(time.Since(m.startTime).Seconds())
	right := fmt.Sprintf("%s   %d/%d   %ds ",
		lipgloss.NewStyle().Foreground(colPurple).Render(m.cfg.ActiveModelLabel()),
		m.jobDone,
		m.jobTotal,
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

	// When resized narrow, wrap top status bar into 2 lines so nothing is cropped
	line1 := padOrTruncate(left, m.width)
	line2 := padOrTruncate(" "+strings.TrimSpace(right), m.width)
	return style.Render(line1 + "\n" + line2)
}

func (m *Model) renderBottomBar() string {
	vLabel := m.T("key_quicklook")
	if m.activeTab == 4 {
		vLabel = m.T("key_preview")
	}

	keys := []struct {
		k string
		v string
	}{
		{"q", m.T("key_quit")},
		{"h/j/k/l", m.T("key_vim_nav")},
		{"L", m.T("key_login")},
		{"s", m.T("key_start_parse")},
		{"Space", m.T("key_select_chat")},
		{"d/x", m.T("key_delete_period")},
		{"p", fmt.Sprintf("%s(%s)", m.T("key_period"), m.periodSpec)},
		{"c", m.T("key_ai_config")},
		{"C", m.T("key_global_config")},
		{"v", vLabel},
		{"e", m.T("key_edit_skill")},
		{"n", m.T("key_new_skill")},
		{"a/m", m.T("key_notes")},
		{"r", m.T("key_rerun_ai")},
		{"1-5", m.T("key_views")},
		{"o", m.T("key_finder")},
		{"?", m.T("key_help")},
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

	// Wrap hotkey items across lines so all options remain 100% visible when window is resized
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

func (m *Model) renderSidebar(width, totalHeight int) string {
	chatsH := totalHeight / 2
	if chatsH < 6 {
		chatsH = 6
	}
	periodsH := totalHeight - chatsH
	if periodsH < 6 {
		periodsH = 6
	}

	innerW := width - 2
	if innerW < 12 {
		innerW = 12
	}

	// 1. Chats Panel (with sliding scroll window so cursor never disappears)
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
		checked := "[ ]"
		if m.selectedChats[c.ChatID] {
			checked = "[✓]"
		}
		cursor := "  "
		isFocusedRow := m.focus == focusChats && m.chatCursor == i
		if isFocusedRow {
			cursor = "▸ "
		}
		cleanTitle := sanitizeTerminalText(c.Title)
		rawRow := fmt.Sprintf("%s%s %s (%d)", cursor, checked, cleanTitle, c.UnreadCount)
		var formatted string
		if isFocusedRow {
			formatted = marqueeSingleLine(rawRow, innerW, m.marqueeTick)
			formatted = lipgloss.NewStyle().Foreground(colCyan).Bold(true).Background(colHighlight).Width(innerW).Render(formatted)
		} else {
			formatted = padOrTruncate(rawRow, innerW)
			if m.selectedChats[c.ChatID] {
				formatted = lipgloss.NewStyle().Foreground(colText).Render(formatted)
			} else {
				formatted = lipgloss.NewStyle().Foreground(colMuted).Render(formatted)
			}
		}
		chatLines = append(chatLines, formatted)
	}
	for len(chatLines) < chatsH-2 {
		chatLines = append(chatLines, strings.Repeat(" ", innerW))
	}

	chatsBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(chatsBorderCol).
		Width(innerW).
		Height(chatsH - 2).
		Render(strings.Join(chatLines, "\n"))

	// 2. Period Folders Panel (with sliding scroll window + single-line marquee on active selection)
	periodsBorderCol := colBorder
	if m.focus == focusPeriods {
		periodsBorderCol = colBorderFoc
	}
	maxPeriodRows := periodsH - 3
	if maxPeriodRows < 1 {
		maxPeriodRows = 1
	}

	if m.periodCursor < m.periodScrollOffset {
		m.periodScrollOffset = m.periodCursor
	}
	if m.periodCursor >= m.periodScrollOffset+maxPeriodRows {
		m.periodScrollOffset = m.periodCursor - maxPeriodRows + 1
	}
	if m.periodScrollOffset < 0 {
		m.periodScrollOffset = 0
	}

	periodHdr := m.T("sidebar_periods_hdr", minInt(m.periodCursor+1, len(m.periods)), len(m.periods))
	var pLines []string
	pLines = append(pLines, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(padOrTruncate(periodHdr, innerW)))

	for i := m.periodScrollOffset; i < len(m.periods) && len(pLines)-1 < maxPeriodRows; i++ {
		rec := m.periods[i]
		isSelected := m.periodCursor == i
		prefix := "  "
		if isSelected {
			prefix = "▸ "
		}
		shortStart := rec.PeriodStart
		if len(shortStart) >= 10 {
			shortStart = shortStart[5:]
		}
		shortEnd := rec.PeriodEnd
		if len(shortEnd) >= 10 {
			shortEnd = shortEnd[5:]
		}
		cleanTitle := sanitizeTerminalText(rec.ChatTitle)
		fullSingleLine := fmt.Sprintf("%s%s [%s->%s] (%dv %di %dd)",
			prefix, cleanTitle, shortStart, shortEnd, rec.VoiceCount, rec.ImageCount, rec.DocumentCount)

		var rowRendered string
		if isSelected {
			scrolled := marqueeSingleLine(fullSingleLine, innerW, m.marqueeTick)
			style := lipgloss.NewStyle().Foreground(colGreen).Bold(true).Width(innerW)
			if m.focus == focusPeriods {
				style = style.Background(colHighlight)
			}
			rowRendered = style.Render(scrolled)
		} else {
			rowRendered = lipgloss.NewStyle().
				Foreground(colMuted).
				Render(padOrTruncate(fullSingleLine, innerW))
		}
		pLines = append(pLines, rowRendered)
	}
	for len(pLines) < periodsH-2 {
		pLines = append(pLines, strings.Repeat(" ", innerW))
	}

	periodsBox := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(periodsBorderCol).
		Width(innerW).
		Height(periodsH - 2).
		Render(strings.Join(pLines, "\n"))

	return lipgloss.JoinVertical(lipgloss.Left, chatsBox, periodsBox)
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

	// Render Tab Header (wraps cleanly onto 2 lines when window is resized narrow so all 5 tabs stay visible)
	tabs := []struct {
		id    int
		label string
	}{
		{1, m.T("tab_1")},
		{2, m.T("tab_2")},
		{3, m.T("tab_3")},
		{4, m.T("tab_4")},
		{5, m.T("tab_5")},
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

	var contentLines []string
	rec := m.activePeriodRecord()
	visibleRows := innerH - len(headerLines) - 1
	if visibleRows < 1 {
		visibleRows = 1
	}

	switch m.activeTab {
	case 1:
		contentLines = m.renderSummaryTab(rec, innerW)
	case 2:
		contentLines = m.renderMediaVaultTab(rec, innerW, visibleRows)
	case 3:
		contentLines = m.renderTranscriptTab(rec, innerW)
	case 4:
		contentLines = m.renderRePromptTab(rec, innerW)
	case 5:
		contentLines = m.renderTraceTab(innerW)
	}

	// Apply vertical scrolling
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

func (m *Model) renderSummaryTab(rec *storage.PeriodSummaryRecord, innerW int) []string {
	if rec == nil {
		return []string{m.T("sum_empty")}
	}

	cardW := innerW - 4
	if cardW < 20 {
		cardW = 20
	}

	var out []string
	title := lipgloss.NewStyle().Foreground(colText).Bold(true).Render(m.T("sum_hdr", rec.ChatTitle))
	meta := lipgloss.NewStyle().Foreground(colMuted).Render(fmt.Sprintf("   ID: %d  │  Created: %s  │  Model: %s", rec.ChatID, rec.CreatedAt, rec.ModelUsed))
	out = append(out, truncateSingleLine(title, innerW), truncateSingleLine(meta, innerW), "")

	// Status & Accounting Card (Pure ASCII badges so borders never shift!)
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

func (m *Model) renderMediaVaultTab(rec *storage.PeriodSummaryRecord, innerW int, visibleRows int) []string {
	if rec == nil || len(rec.MediaItems) == 0 {
		return []string{m.T("media_empty")}
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
		m.T("media_hdr", m.mediaCursor+1, len(rec.MediaItems), rec.FolderPath),
	), "")

	selStart := 0
	selEnd := 0

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
		if idx == m.mediaCursor {
			selStart = len(out)
			selEnd = len(out) + len(cardLines)
		}
		out = append(out, cardLines...)
		out = append(out, "")
	}

	// Keep the currently selected media card visible in the viewport as the user scrolls with j/k
	if m.mediaCursor == 0 {
		m.mainScroll = 0
	} else {
		if selStart < m.mainScroll {
			m.mainScroll = selStart
		}
		if selEnd >= m.mainScroll+visibleRows {
			m.mainScroll = selEnd - visibleRows + 1
		}
		if m.mainScroll < 0 {
			m.mainScroll = 0
		}
	}

	return out
}

func (m *Model) renderTranscriptTab(rec *storage.PeriodSummaryRecord, innerW int) []string {
	if rec == nil || len(rec.Messages) == 0 {
		return []string{m.T("transcript_empty")}
	}
	cardW := innerW - 4
	var out []string
	out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(
		m.T("transcript_hdr", len(rec.Messages)),
	), "")

	for _, msg := range rec.Messages {
		hdr := fmt.Sprintf("[#%d] %s  •  %s", msg.MessageID, msg.Sender, msg.Timestamp)
		var lines []string
		if strings.TrimSpace(msg.Text) != "" {
			lines = append(lines, msg.Text)
		}
		if msg.MediaType == "voice" || msg.VoiceTranscript != "" {
			aiCap := formatAICaptureText(msg.VoiceTranscript, m.cfg.Language)
			lines = append(lines, fmt.Sprintf("[VOICE - %s]: %s", msg.MediaFile, aiCap))
		} else if msg.MediaType != "" || msg.MediaAnalysis != "" {
			mTag := strings.ToUpper(msg.MediaType)
			if mTag == "DOCUMENT" {
				mTag = "DOC"
			}
			aiCap := formatAICaptureText(msg.MediaAnalysis, m.cfg.Language)
			lines = append(lines, fmt.Sprintf("[%s - %s]: %s", mTag, msg.MediaFile, aiCap))
		}
		out = append(out, renderInteriorCard(hdr, strings.Join(lines, "\n"), cardW, colBorder)...)
		out = append(out, "")
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

func (m *Model) renderRePromptTab(rec *storage.PeriodSummaryRecord, innerW int) []string {
	cardW := innerW - 4
	if cardW < 20 {
		cardW = 20
	}
	var out []string
	out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("skills_tab_hdr")), "")

	// 1. Current AI Configuration Card (Compact)
	cliPath := m.cfg.AgyCLIBin()
	apiStatus := "[NOT CONNECTED] agy CLI not found — press 'c'"
	if m.cfg.IsRussian() {
		apiStatus = "[НЕ ПОДКЛЮЧЕН] agy CLI не найден — нажмите 'c'"
	}
	if cliPath != "" {
		if m.cfg.IsRussian() {
			apiStatus = fmt.Sprintf("[ПОДКЛЮЧЕН] %s", cliPath)
		} else {
			apiStatus = fmt.Sprintf("[CONNECTED] %s", cliPath)
		}
	}
	modelName := strings.TrimSpace(m.cfg.AgyModel)
	if modelName == "" {
		modelName = "auto"
	}
	activeSkillLabel := m.activeSkill
	if activeSkillLabel == "" {
		activeSkillLabel = config.DefaultSkillName
	}
	aiConfigBody := strings.Join([]string{
		m.T("skills_lbl_engine", apiStatus, m.cfg.ActiveModelLabel(), modelName),
		m.T("skills_lbl_skill", activeSkillLabel, m.cfg.SkillsDir()),
		m.T("skills_lbl_diag", m.aiStatusLine),
	}, "\n")
	out = append(out, renderInteriorCard(m.T("skills_card_ai_config"), aiConfigBody, cardW, colGreen)...)
	out = append(out, "")

	// 2. Skills Catalog Card
	var skillRows []string
	if len(m.skillList) == 0 {
		skillRows = append(skillRows, m.T("skills_empty_lib"))
	} else {
		for idx, sk := range m.skillList {
			prefix := "  "
			isHovered := idx == m.skillCursor
			if isHovered {
				prefix = "▸ "
			}
			activeBadge := "         "
			if sk.Name == m.activeSkill && sk.Kind == skills.KindSummary {
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
			} else if sk.Name == m.activeSkill && sk.Kind == skills.KindSummary {
				skillRows = append(skillRows, lipgloss.NewStyle().Foreground(colGreen).Render(row))
			} else {
				skillRows = append(skillRows, lipgloss.NewStyle().Foreground(colText).Render(row))
			}
		}
	}

	skillsTitle := m.T("skills_card_catalog", minInt(m.skillCursor+1, len(m.skillList)), len(m.skillList))
	out = append(out, renderInteriorCard(skillsTitle, strings.Join(skillRows, "\n"), cardW, colCyan)...)
	out = append(out, "")

	// 3. Selected Period & Run Settings Card
	runNotesText := strings.TrimSpace(m.runNotes)
	if runNotesText == "" {
		runNotesText = m.T("skills_notes_none")
	}
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
		out = append(out, renderInteriorCard(m.T("skills_card_run"), strings.Join(periodLines, "\n"), cardW, colBlue)...)
	} else {
		noPeriodBody := m.T("skills_period_none", runNotesText)
		out = append(out, renderInteriorCard(m.T("skills_card_run"), noPeriodBody, cardW, colBlue)...)
	}
	out = append(out, "")

	// 4. Actions & Shortcuts Card
	var actionsBody string
	if m.cfg.IsRussian() {
		actionsBody = strings.Join([]string{
			"[v]     Просмотр навыка во встроенном окне: Полные инструкции и текст промпта",
			"[Enter] Активировать навык сводки         : Сделать выбранный навык активным для анализа",
			"[e]     Редактировать навык в $EDITOR     : Открыть SKILL.md в системном редакторе (vim, nano, code)",
			"[n]     Создать новый навык               : Создать шаблон навыка в .agents/skills/",
			"[a / m] Изменить заметки для запуска      : Добавить временные инструкции поверх навыка",
			"[r]     Перезапустить ИИ-обработку        : Повторный анализ медиа и сводка периода",
			"[c]     Настройка движка ИИ               : Путь к agy CLI и переключение моделей (F2)",
			"[C]     Глобальные настройки              : Смена языка интерфейса (En/Ru) и параметры",
		}, "\n")
	} else {
		actionsBody = strings.Join([]string{
			"[v]     Preview Skill in Embedded Window: Full instructions & prompt viewer",
			"[Enter] Activate Summary Skill          : Set highlighted summary skill as active for parsing",
			"[e]     Edit Skill in $EDITOR            : Open SKILL.md in system editor (vim, nano, code)",
			"[n]     Create New Custom Skill          : Scaffold a new skill in .agents/skills/ and edit",
			"[a / m] Edit Extra Run Notes             : Add temporary per-run instructions on top of skill",
			"[r]     Re-Run AI Fetching               : Re-analyze media and re-summarize active period",
			"[c]     Connect / Configure AI Engine    : Configure agy CLI binary and switch models (F2)",
			"[C]     Global Configuration             : Change interface language (En/Ru) and settings",
		}, "\n")
	}
	out = append(out, renderInteriorCard(m.T("skills_card_actions"), actionsBody, cardW, colPurple)...)
	return out
}

func (m *Model) renderTraceTab(innerW int) []string {
	var out []string
	out = append(out, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("trace_hdr")), "")
	for _, line := range m.traceLogs {
		out = append(out, "  "+truncateSingleLine(line, innerW-4))
	}
	return out
}

func renderInteriorCard(title string, body string, cardWidth int, titleCol lipgloss.Color) []string {
	innerTextW := cardWidth - 4
	if innerTextW < 10 {
		innerTextW = 10
	}

	wrappedLines := wrapTextBlock(body, innerTextW)
	var paddedLines []string
	for _, l := range wrappedLines {
		paddedLines = append(paddedLines, padOrTruncate(l, innerTextW))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(colBorder).
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
		modalBox = m.renderInputModal(
			m.T("input_notes_title"),
			m.T("input_notes_sub"),
			m.inputBuffer,
			m.T("input_notes_foot"),
		)
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

	// Calculate scrolling for modal body
	maxVisible := innerH - 1 // reserve 1 line for footer
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
		if i == 1 && len(val) > 0 { // Field 1: API Hash masking for privacy & safe streaming
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

	// 3-Step Connection Diagnostic Box
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

	// Language Selector Column
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
		enStyle = enStyle.Background(colGreen).Foreground(colBg).Bold(true)
		ruStyle = ruStyle.Background(colHighlight).Foreground(colMuted)
	} else {
		ruPrefix = "▸ "
		enStyle = enStyle.Background(colHighlight).Foreground(colMuted)
		ruStyle = ruStyle.Background(colGreen).Foreground(colBg).Bold(true)
	}

	btnEn := enStyle.Render(enPrefix + "[1] En — English (Default)")
	btnRu := ruStyle.Render(ruPrefix + "[2] Ru — Русский (Russian)")
	rows = append(rows, btnEn, btnRu, "")

	// System Information Card
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

	// Action buttons
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
			"  j / k или Вверх/Вниз: Перемещение курсора / прокрутка активного окна",
			"  J / K               : Быстрая прокрутка окна вниз / вверх",
			"  g / G               : В начало (g) или в конец (G) списка / окна",
			"  Ctrl+d / Ctrl+u     : Прокрутка на полстраницы вниз / вверх",
			"  h / l / Tab         : Переключение фокуса (Чаты, Папки периодов, Детали)",
			"  1 - 5               : Переключение вкладок (Сводка, Медиа, Стенограмма, Навыки, Логи)",
			"  Space / a           : Выбор чата (Space) / выбрать все (a)",
			"  s / Enter           : Запуск анализа выбранных чатов (или выбор навыка во вкл. 4)",
			"  d / x               : Удалить выбранную папку периода (с подтверждением)",
			"  L (Shift+L)         : Диагностика MTProto и вход по QR-коду с телефона",
			"  c                   : Настройка и тестирование движка ИИ (Antigravity CLI — agy)",
			"  C (Shift+C)         : Глобальные настройки (выбор языка En/Ru и параметры)",
			"  e                   : Открыть выбранный навык в $EDITOR (вкл. 4)",
			"  n                   : Создать новый навык в .agents/skills/ (вкл. 4)",
			"  a / m               : Редактировать дополнительные заметки для сводки",
			"  r                   : Перезапустить ИИ-обработку (медиа и сводка периода)",
			"  p                   : Настройка окна периода (по умолчанию 1m = 1 месяц)",
			"  o / v               : Открыть Finder (o) или Просмотр файла/навыка (v)",
			"  q / Esc             : Выход или закрытие модального окна",
		)
	} else {
		lines = append(lines,
			"  j / k or Up/Dn      : Move cursor down / up or scroll active view",
			"  J / K               : Fast scroll view down / up",
			"  g / G               : Jump to top (g) or bottom (G) of active list/view",
			"  Ctrl+d / Ctrl+u     : Half-page down / half-page up",
			"  h / l / Tab         : Move focus left / right between Chats, Period Folders, and Details",
			"  1 - 5               : Switch views (Summary, Media Vault, Transcript, Skills & Re-Run, Trace)",
			"  Space / a           : Toggle chat selection (a = select/deselect all in chats view)",
			"  s / Enter           : Start parsing & summarizing selected chats (or activate skill in Tab 4)",
			"  d / x               : Delete selected Period Folder (opens confirmation alert)",
			"  L (Shift+L)         : Open MTProto Connection Test & Phone QR Login modal",
			"  c                   : Connect & Configure AI Engine (Antigravity CLI — agy)",
			"  C (Shift+C)         : Global Configuration (Language En/Ru and system preferences)",
			"  e                   : Open selected skill in $EDITOR (Tab 4)",
			"  n                   : Create a new custom skill in .agents/skills/ (Tab 4)",
			"  a / m               : Edit extra run notes / instructions for summarization",
			"  r                   : Re-run AI fetching via active skill (transcribe media & summarize)",
			"  p                   : Configure period window (default 1m = 1 month)",
			"  o / v               : Open Finder (o) or QuickLook / Preview skill in TUI (v)",
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

// String Width & Marquee Helpers (Rune-safe, single-line strict)
func sanitizeTerminalText(s string) string {
	var b strings.Builder
	for _, r := range s {
		// Strip variation selectors, zero-width joiners, and regional indicator flag symbols
		// that have ambiguous cell widths in macOS Terminal.app and shift box borders
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
		words := strings.Fields(para)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}
		cur := words[0]
		for _, w := range words[1:] {
			if lipgloss.Width(cur)+1+lipgloss.Width(w) <= width {
				cur += " " + w
			} else {
				// If a single word/path is longer than width, split it cleanly
				for lipgloss.Width(cur) > width {
					r := []rune(cur)
					cut := minInt(len(r), width)
					result = append(result, string(r[:cut]))
					cur = string(r[cut:])
				}
				result = append(result, cur)
				cur = w
			}
		}
		for lipgloss.Width(cur) > width {
			r := []rune(cur)
			cut := minInt(len(r), width)
			result = append(result, string(r[:cut]))
			cur = string(r[cut:])
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
	m.activeTab = 4
	if skillIdx >= 0 && skillIdx < len(m.skillList) {
		m.skillCursor = skillIdx
	}
	m.previewScroll = 0
	m.activeModal = modalSkillPreview
}


