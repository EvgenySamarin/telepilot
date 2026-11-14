package tui

import (
	"context"
	"sync"
	"time"

	"telepilot/internal/domain/ai"
	"telepilot/internal/domain/skills"
	"telepilot/internal/domain/storage"
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

type aiQuotaUpdatedMsg struct {
	err error
}

type parseDoneMsg struct {
	record *storage.SkillOutputRecord
	err    error
}

type scheduleDirectiveFoundMsg struct {
	chatID    int64
	chatTitle string
	directive ai.ScheduleDirective
}

type scheduledTaskExecutedMsg struct {
	task    storage.ScheduledTask
	sentMsg storage.LiveMessage
	text    string
	err     error
}
