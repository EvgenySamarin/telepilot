/*
File: internal/tui/ai_test.go
Description: TUI unit tests for AI auto-reply modes, single-shot replace/append/cancel,
skills live reload/import/reset, AI output tab, scheduled tasks, and quota status.
*/

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"telepilot/internal/config"
	"telepilot/internal/skills"
	"telepilot/internal/storage"
)

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

func TestConfigModalResetDefaultSkills(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{
		ProjectRoot: tmpDir,
		DataDir:     tmpDir,
		Language:    "en",
	}
	m := NewModel(cfg, nil, nil, true)

	initialCount := len(m.skillList)
	if initialCount == 0 {
		t.Fatalf("expected skills loaded in NewModel")
	}

	// Delete standup-digest from disk
	targetSkillDir := filepath.Join(cfg.SkillsDir(), "standup-digest")
	_ = os.RemoveAll(targetSkillDir)
	m.reloadSkills()

	if len(m.skillList) != initialCount-1 {
		t.Fatalf("expected skills count to decrease to %d, got %d", initialCount-1, len(m.skillList))
	}

	// Open Global Config modal
	mUpdated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'C'}})
	m = mUpdated.(*Model)
	if m.activeModal != modalGlobalConfig {
		t.Fatalf("expected modalGlobalConfig open, got %v", m.activeModal)
	}

	// Press Ctrl+R to reset default skills
	mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlR})
	m = mUpdated.(*Model)

	if len(m.skillList) != initialCount {
		t.Errorf("expected skills count to be restored to %d, got %d", initialCount, len(m.skillList))
	}
	if !strings.Contains(m.statusText, "restored") {
		t.Errorf("expected statusText to mention restored, got %q", m.statusText)
	}
}

func TestScheduledTasksLifecycle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "sched_tui_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	cfg := &config.AppConfig{
		ProjectRoot: tmpDir,
		DataDir:     tmpDir,
		Language:    "en",
	}
	m := NewModel(cfg, nil, nil, true)

	if len(m.chats) == 0 {
		t.Fatalf("expected demo chats loaded")
	}
	testChat := m.chats[0]

	// 1. Send an AI reply containing a one-shot <schedule> tag
	rawReply := "Sure, I'll remind you in 30 minutes!\n\n<schedule in=\"30m\" repeat=\"none\">Check oven status</schedule>"
	mUpdated, _ := m.Update(aiChatReplyGeneratedMsg{
		chat:     testChat,
		reply:    rawReply,
		sendMode: "draft",
	})
	m = mUpdated.(*Model)

	// Unprompted confirmation ("Sure, I will...") must be suppressed from chatInputBuffer
	if m.chatInputBuffer != "" {
		t.Errorf("expected chatInputBuffer to be empty (no unprompted confirmation sent to chat), got: %q", m.chatInputBuffer)
	}

	// Task must be recorded in m.scheduledTasks
	if len(m.scheduledTasks) != 1 {
		t.Fatalf("expected 1 scheduled task, got %d", len(m.scheduledTasks))
	}
	task := m.scheduledTasks[0]
	if task.ChatID != testChat.ChatID || task.TaskPrompt != "Check oven status" {
		t.Errorf("unexpected scheduled task properties: %+v", task)
	}
	if task.DueAt.Before(time.Now().Add(25 * time.Minute)) {
		t.Errorf("expected DueAt ~30m in future, got %v", task.DueAt)
	}

	// Header indicator must reflect active scheduled tasks
	renderedHeader := m.renderTopBar()
	if !strings.Contains(renderedHeader, "⏰1") {
		t.Errorf("expected top bar to show '⏰1' indicator, got: %s", renderedHeader)
	}

	// 2. Tick when task is not due: task stays in list
	cmds := m.checkScheduledTasksCmd()
	if len(cmds) != 0 {
		t.Errorf("expected 0 commands when task not due, got %d", len(cmds))
	}
	if len(m.scheduledTasks) != 1 {
		t.Errorf("expected task to remain pending, got %d", len(m.scheduledTasks))
	}

	// 3. Fast-forward DueAt into the past to simulate time elapsed
	m.scheduledTasks[0].DueAt = time.Now().Add(-10 * time.Second)
	cmds = m.checkScheduledTasksCmd()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 execution command when task is due, got %d", len(cmds))
	}
	// One-shot task should be removed from pending list
	if len(m.scheduledTasks) != 0 {
		t.Errorf("expected one-shot task to be removed after triggering, got %d", len(m.scheduledTasks))
	}

	// 4. Test Repeating Task Rescheduling
	repeatTask := storage.ScheduledTask{
		ID:          "task_repeat_test",
		ChatID:      testChat.ChatID,
		ChatTitle:   testChat.Title,
		TaskPrompt:  "Drink a glass of water",
		DueAt:       time.Now().Add(-5 * time.Second), // already due
		IntervalStr: "1h",
		CreatedAt:   time.Now().Add(-1 * time.Hour),
	}
	m.scheduledTasks = append(m.scheduledTasks, repeatTask)
	cmds = m.checkScheduledTasksCmd()
	if len(cmds) != 1 {
		t.Fatalf("expected 1 command for due repeat task, got %d", len(cmds))
	}
	// Repeat task should have been rescheduled to future
	if len(m.scheduledTasks) != 1 {
		t.Fatalf("expected repeat task to be retained in pending list, got %d", len(m.scheduledTasks))
	}
	if !m.scheduledTasks[0].DueAt.After(time.Now()) {
		t.Errorf("expected repeat task DueAt to be updated into future, got %v", m.scheduledTasks[0].DueAt)
	}

	// 5. Test scheduledTaskExecutedMsg handling
	m.selectedChatID = testChat.ChatID
	beforeMsgCount := len(m.liveMessages)
	sentMsg := storage.LiveMessage{
		MessageID:  99991,
		Sender:     "You",
		Timestamp:  "12:30",
		Text:       "Reminder: Check oven status",
		IsOutgoing: true,
	}
	mUpdated, _ = m.Update(scheduledTaskExecutedMsg{
		task:    task,
		sentMsg: sentMsg,
		text:    sentMsg.Text,
		err:     nil,
	})
	m = mUpdated.(*Model)

	if len(m.liveMessages) != beforeMsgCount+1 {
		t.Errorf("expected liveMessages count to increase by 1, got %d", len(m.liveMessages))
	}
	lastMsg := m.liveMessages[len(m.liveMessages)-1]
	if lastMsg.MessageID != 99991 || lastMsg.Text != "Reminder: Check oven status" {
		t.Errorf("unexpected last message appended: %+v", lastMsg)
	}

	// 6. Test MaxRuns stopping condition
	limitedTask := storage.ScheduledTask{
		ID:          "task_limited",
		ChatID:      testChat.ChatID,
		ChatTitle:   testChat.Title,
		TaskPrompt:  "Counted hello",
		DueAt:       time.Now().Add(-1 * time.Second),
		IntervalStr: "1s",
		MaxRuns:     2,
		RunsCount:   0,
	}
	m.scheduledTasks = []storage.ScheduledTask{limitedTask}
	// Run 1: RunsCount becomes 1, task rescheduled
	_ = m.checkScheduledTasksCmd()
	if len(m.scheduledTasks) != 1 || m.scheduledTasks[0].RunsCount != 1 {
		t.Fatalf("expected task retained after run 1 with RunsCount=1, got %+v", m.scheduledTasks)
	}
	// Run 2: RunsCount reaches MaxRuns (2), task stopped and removed
	m.scheduledTasks[0].DueAt = time.Now().Add(-1 * time.Second)
	_ = m.checkScheduledTasksCmd()
	if len(m.scheduledTasks) != 0 {
		t.Errorf("expected task to be stopped and removed after reaching MaxRuns=2, got %+v", m.scheduledTasks)
	}

	// 7. Test cancelling scheduled tasks
	m.scheduledTasks = []storage.ScheduledTask{
		{ID: "t1", ChatID: testChat.ChatID, ChatTitle: testChat.Title, DueAt: time.Now().Add(1 * time.Hour)},
		{ID: "t2", ChatID: 9999, ChatTitle: "Other Chat", DueAt: time.Now().Add(1 * time.Hour)},
	}
	stoppedChat := m.cancelScheduledTasksForChat(testChat.ChatID, testChat.Title)
	if stoppedChat != 1 {
		t.Errorf("expected 1 task stopped for testChat, got %d", stoppedChat)
	}
	if len(m.scheduledTasks) != 1 || m.scheduledTasks[0].ChatID != 9999 {
		t.Errorf("expected only other chat task to remain, got %+v", m.scheduledTasks)
	}

	stoppedAll := m.cancelAllScheduledTasks()
	if stoppedAll != 1 {
		t.Errorf("expected 1 remaining task stopped by cancelAllScheduledTasks, got %d", stoppedAll)
	}
	if len(m.scheduledTasks) != 0 {
		t.Errorf("expected 0 scheduled tasks after cancelAll, got %d", len(m.scheduledTasks))
	}
}

func TestAIQuotaStatusUpdatesInTab2(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := &config.AppConfig{
		DataDir:      tmpDir,
		ProjectRoot:  tmpDir,
		DefaultSkill: config.DefaultSkillName,
		Language:     "en",
	}
	m := NewModel(cfg, nil, nil, true)
	m.activeTab = 2

	// 1. Quota reached via chat reply failure
	mUpdated, _ := m.Update(aiChatReplyGeneratedMsg{
		chat: m.chats[0],
		err:  fmt.Errorf("exit status 3 (error: Individual quota reached. Please upgrade)"),
	})
	m = mUpdated.(*Model)
	if !strings.Contains(m.aiQuotaStatus, "[QUOTA REACHED]") {
		t.Fatalf("expected [QUOTA REACHED], got %q", m.aiQuotaStatus)
	}
	tab2 := strings.Join(m.renderRePromptTab(nil, 100), "\n")
	if !strings.Contains(tab2, "Quota & Health:") || !strings.Contains(tab2, "[QUOTA REACHED]") {
		t.Errorf("expected Tab 2 to render Quota & Health with [QUOTA REACHED], got:\n%s", tab2)
	}

	// 2. Timeout via single-shot aiQuotaUpdatedMsg
	mUpdated, _ = m.Update(aiQuotaUpdatedMsg{err: fmt.Errorf("signal: killed")})
	m = mUpdated.(*Model)
	if !strings.Contains(m.aiQuotaStatus, "[TIMEOUT]") {
		t.Fatalf("expected [TIMEOUT], got %q", m.aiQuotaStatus)
	}

	// 3. Recovers to [OK] on successful AI test
	mUpdated, _ = m.Update(aiTestDoneMsg{ok: true, line: "[OK] Connected"})
	m = mUpdated.(*Model)
	if !strings.Contains(m.aiQuotaStatus, "[OK]") {
		t.Fatalf("expected [OK] after recovery, got %q", m.aiQuotaStatus)
	}
}
