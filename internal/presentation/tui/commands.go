package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"telepilot/internal/domain/config"
	"telepilot/internal/domain/skills"
	"telepilot/internal/domain/storage"
)

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

func isMacOS() bool {
	return runtime.GOOS == "darwin"
}

// openFileInViewer opens the target file in its native viewer application (Preview, QuickTime, default viewer)
// using macOS `open <path>`. Unlike `qlmanage -p`, `open` reliably delegates to macOS LaunchServices and avoids
// SIGABRT crashes caused by QuickLook sandbox assertions on modern macOS versions.
func openFileInViewer(filePath string) error {
	if !isMacOS() {
		return exec.Command("xdg-open", filePath).Start()
	}
	return exec.Command("open", filePath).Start()
}

// playVoiceAudio plays voice notes using native audio tools (afplay on macOS, aplay on Linux).
func playVoiceAudio(filePath string) error {
	if !isMacOS() {
		return exec.Command("aplay", filePath).Start()
	}
	return exec.Command("afplay", filePath).Start()
}

func (m *Model) openSelectedSkillInFinder() {
	_ = os.MkdirAll(m.cfg.SkillsDir(), 0o755)
	if len(m.skillList) > 0 {
		if m.skillCursor < 0 || m.skillCursor >= len(m.skillList) {
			m.skillCursor = 0
		}
		sk := m.skillList[m.skillCursor]
		if isMacOS() {
			_ = exec.Command("open", "-R", sk.Path).Start()
		}
		m.statusText = "Opened in Finder"
		m.addTrace("Revealed skill in Finder: " + sk.Path)
		return
	}
	if isMacOS() {
		_ = exec.Command("open", m.cfg.SkillsDir()).Start()
	}
	m.statusText = "Opened in Finder"
	m.addTrace("Opened skills folder in Finder: " + m.cfg.SkillsDir())
}

func (m *Model) openSelectedSkillInMacEditor() {
	if len(m.skillList) == 0 {
		return
	}
	if m.skillCursor < 0 || m.skillCursor >= len(m.skillList) {
		m.skillCursor = 0
	}
	sk := m.skillList[m.skillCursor]
	if isMacOS() {
		_ = exec.Command("open", "-t", sk.Path).Start()
	}
	m.statusText = "Opened in macOS editor"
	m.addTrace("Opened skill in macOS default text editor: " + sk.Path)
}

func expandLocalPath(raw string) string {
	p := strings.TrimSpace(raw)
	p = strings.Trim(p, `"'`)
	// Handle shell-escaped spaces when dragging a file into macOS Terminal
	p = strings.ReplaceAll(p, `\ `, " ")
	if strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			p = filepath.Join(home, p[2:])
		}
	}
	return p
}

func isLikelyLocalPath(raw string) bool {
	p := strings.TrimSpace(raw)
	p = strings.Trim(p, `"'`)
	if strings.HasPrefix(p, "/") || strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "./") || strings.HasPrefix(p, "../") {
		return true
	}
	if strings.HasSuffix(strings.ToLower(p), ".md") || strings.Contains(p, string(filepath.Separator)) {
		return true
	}
	return false
}

func (m *Model) importSkillFromMacPickerCmd() tea.Cmd {
	if !isMacOS() {
		m.inputBuffer = ""
		m.activeModal = modalNewSkill
		return nil
	}
	skillsDir := m.cfg.SkillsDir()
	_ = os.MkdirAll(skillsDir, 0o755)
	if m.cfg.IsRussian() {
		m.statusText = "Выберите навык в Finder..."
	} else {
		m.statusText = "Select skill in Finder..."
	}
	m.addTrace("Opening native macOS file chooser to import AI skill...")

	prompt := "Select a SKILL.md or .md prompt file to import into TelePilot (.agents/skills):"
	if m.cfg.IsRussian() {
		prompt = "Выберите файл SKILL.md или .md для импорта навыка в TelePilot (.agents/skills):"
	}

	return func() tea.Msg {
		script := fmt.Sprintf(`POSIX path of (choose file with prompt %q)`, prompt)
		out, err := exec.Command("osascript", "-e", script).CombinedOutput()
		if err != nil {
			outStr := strings.ToLower(string(out) + " " + err.Error())
			if strings.Contains(outStr, "-128") || strings.Contains(outStr, "cancel") {
				return skillImportedMsg{canceled: true}
			}
			return skillImportedMsg{err: fmt.Errorf("macOS Finder picker failed: %w (%s)", err, strings.TrimSpace(string(out)))}
		}
		chosenPath := strings.TrimSpace(string(out))
		if chosenPath == "" {
			return skillImportedMsg{canceled: true}
		}
		imported, impErr := skills.Import(skillsDir, chosenPath)
		return skillImportedMsg{imported: imported, err: impErr}
	}
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
			chatID:         chat.ChatID,
			messageID:      msg.MessageID,
			mediaType:      msg.MediaType,
			filePath:       path,
			revealInFinder: false,
			err:            err,
		}
	}
}

func (m *Model) downloadAndRevealLiveMediaInFinderCmd(chat storage.ChatInfo, msg storage.LiveMessage) tea.Cmd {
	m.statusText = "Downloading media for Finder..."
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		path, err := m.mtproto.DownloadMessageMedia(ctx, chat, msg)
		return liveMediaDownloadedMsg{
			chatID:         chat.ChatID,
			messageID:      msg.MessageID,
			mediaType:      msg.MediaType,
			filePath:       path,
			revealInFinder: true,
			err:            err,
		}
	}
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

// runChatAICmd executes the active AI skill on the currently selected chat and displays its output in Tab 3.
func (m *Model) runChatAICmd() tea.Cmd {
	ch := m.activeChat()
	if ch == nil {
		return nil
	}
	target := *ch

	skillName := config.DefaultSkillName
	notes := m.runNotes
	if cfg, ok := m.activeChatAIConfig(); ok {
		if cfg.Skill != "" {
			skillName = cfg.Skill
		}
		if cfg.Notes != "" {
			notes = cfg.Notes
		}
	} else if m.activeSkill != "" {
		skillName = m.activeSkill
	}

	msgsCopy := make([]storage.LiveMessage, len(m.liveMessages))
	copy(msgsCopy, m.liveMessages)

	m.statusBusy = true
	m.jobDone = 0
	m.jobTotal = 1
	m.statusText = fmt.Sprintf("Running skill (%s)...", skillName)

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		m.uiEventCh <- parseLogMsg(fmt.Sprintf("Running AI skill '%s' on chat '%s' (%d messages)...", skillName, target.Title, len(msgsCopy)))
		rec, err := m.ai.ExecuteChatSkill(ctx, target, skillName, notes, msgsCopy)
		if err != nil {
			m.uiEventCh <- parseDoneMsg{record: nil, err: err}
			return
		}
		m.uiEventCh <- parseDoneMsg{record: &rec, err: nil}
	}()

	return nil
}
