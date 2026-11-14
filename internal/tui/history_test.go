package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"telepilot/internal/storage"
)

func newHistoryTestModel(t *testing.T) *Model {
	t.Helper()
	cfg := AppConfigStub(t.TempDir(), "en")
	m := NewModel(cfg, nil, nil, true)
	m.focus = focusMain
	m.activeTab = 1
	return m
}

func TestOlderMessagesPrependAndAnchorCursor(t *testing.T) {
	m := newHistoryTestModel(t)
	ch := m.activeChat()
	if ch == nil {
		t.Fatal("expected active demo chat")
	}

	initial := []storage.LiveMessage{
		{MessageID: 101, ChatID: ch.ChatID, Sender: "Alice", Text: "msg 101"},
		{MessageID: 102, ChatID: ch.ChatID, Sender: "You", Text: "msg 102"},
	}
	m.liveMessages = initial
	m.liveMsgCache[ch.ChatID] = initial
	m.liveMsgCursor = 0 // anchored on MessageID 101
	m.liveLoadingOlder = true

	older := []storage.LiveMessage{
		{MessageID: 98, ChatID: ch.ChatID, Sender: "Alice", Text: "msg 98"},
		{MessageID: 99, ChatID: ch.ChatID, Sender: "You", Text: "msg 99"},
		{MessageID: 100, ChatID: ch.ChatID, Sender: "Alice", Text: "msg 100"},
	}
	_, _ = m.Update(olderMessagesLoadedMsg{
		chatID:   ch.ChatID,
		messages: older,
		hasMore:  true,
	})

	if m.liveLoadingOlder {
		t.Fatal("expected liveLoadingOlder to reset to false")
	}
	if len(m.liveMessages) != 5 {
		t.Fatalf("expected 5 merged messages, got %d", len(m.liveMessages))
	}
	if m.liveMessages[m.liveMsgCursor].MessageID != 101 {
		t.Fatalf("expected cursor to remain anchored on MessageID 101, got %d (idx %d)",
			m.liveMessages[m.liveMsgCursor].MessageID, m.liveMsgCursor)
	}
}

func TestLiveSyncMergesWithoutDroppingOlderCache(t *testing.T) {
	m := newHistoryTestModel(t)
	ch := m.activeChat()
	if ch == nil {
		t.Fatal("expected active demo chat")
	}

	cached := []storage.LiveMessage{
		{MessageID: 50, ChatID: ch.ChatID, Text: "old 50"},
		{MessageID: 100, ChatID: ch.ChatID, Text: "msg 100"},
	}
	m.liveMessages = cached
	m.liveMsgCache[ch.ChatID] = cached
	m.liveMsgCursor = 0

	latest := []storage.LiveMessage{
		{MessageID: 100, ChatID: ch.ChatID, Text: "msg 100"},
		{MessageID: 101, ChatID: ch.ChatID, Text: "new 101"},
	}
	_, _ = m.Update(liveMessagesLoadedMsg{
		chatID:   ch.ChatID,
		messages: latest,
	})

	if len(m.liveMessages) != 3 {
		t.Fatalf("expected 3 messages after merge, got %d", len(m.liveMessages))
	}
	if m.liveMessages[0].MessageID != 50 || m.liveMessages[2].MessageID != 101 {
		t.Fatalf("unexpected merged range: %+v", m.liveMessages)
	}
}

func TestDateJumpAndReturnToLive(t *testing.T) {
	m := newHistoryTestModel(t)
	ch := m.activeChat()
	if ch == nil {
		t.Fatal("expected active demo chat")
	}

	m.liveLoadingOlder = true
	_, _ = m.Update(dateMessagesLoadedMsg{
		chatID:    ch.ChatID,
		dateLabel: "2026-10-15",
		messages: []storage.LiveMessage{
			{MessageID: 12, ChatID: ch.ChatID, Text: "october msg"},
		},
	})

	if m.liveHistoryByDate[ch.ChatID] != "2026-10-15" {
		t.Fatalf("expected liveHistoryByDate to be set, got %q", m.liveHistoryByDate[ch.ChatID])
	}

	// Background live sync must not overwrite the historical date view
	_, _ = m.Update(liveMessagesLoadedMsg{
		chatID:   ch.ChatID,
		messages: []storage.LiveMessage{{MessageID: 999, ChatID: ch.ChatID, Text: "latest"}},
	})
	if len(m.liveMessages) != 1 || m.liveMessages[0].MessageID != 12 {
		t.Fatalf("expected date snapshot to be preserved, got %+v", m.liveMessages)
	}

	// Pressing G in Tab 1 returns to live
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'G'}})
	if m.liveHistoryByDate[ch.ChatID] != "" {
		t.Fatal("expected G to clear liveHistoryByDate")
	}
}

func TestHistoryModalOpensAndValidates(t *testing.T) {
	m := newHistoryTestModel(t)
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'P'}})
	if m.activeModal != modalHistoryLoad {
		t.Fatalf("expected modalHistoryLoad, got %v", m.activeModal)
	}

	// Invalid input closes modal and sets statusText
	m.inputBuffer = "not-a-date"
	_, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.activeModal != modalNone {
		t.Fatalf("expected modalNone after enter, got %v", m.activeModal)
	}
	if m.statusText == "" {
		t.Fatal("expected statusText error on invalid history spec")
	}

	// Verify maybeLoadOlderCmd respects liveHasMore == false
	ch := m.activeChat()
	m.liveHasMore[ch.ChatID] = false
	if cmd := m.maybeLoadOlderCmd(); cmd != nil {
		t.Fatal("expected nil cmd when liveHasMore is false")
	}
	_ = time.Now()
}

func TestLiveChatSanitizesControlAndZeroWidthChars(t *testing.T) {
	m := newHistoryTestModel(t)
	m.width = 120
	m.height = 36
	ch := m.activeChat()
	m.liveMessages = []storage.LiveMessage{
		{
			MessageID: 1,
			ChatID:    ch.ChatID,
			Sender:    "Alice",
			Timestamp: "2026-11-14 12:00",
			Text:      "Hello\t\tworld\r\nwith\u200Bzero\uFE0Fwidth\u200Dand\x1b[31mCSI",
		},
	}
	m.liveMsgCursor = 0

	rendered := m.View()
	lines := strings.Split(rendered, "\n")
	if len(lines) > m.height {
		t.Fatalf("expected rendered lines <= %d, got %d", m.height, len(lines))
	}
	for i, line := range lines {
		if w := lipgloss.Width(line); w > m.width {
			t.Fatalf("line %d exceeds width %d (got %d)", i, m.width, w)
		}
		if strings.ContainsAny(line, "\t\r\u200B\uFE0F\u200D") {
			t.Fatalf("line %d contains unsanitized control/zero-width character: %q", i, line)
		}
	}
}

func TestPersistentHistorySurvivesRestart(t *testing.T) {
	cfg := AppConfigStub(t.TempDir(), "en")
	m1 := NewModel(cfg, nil, nil, true)
	ch := m1.activeChat()
	if ch == nil {
		t.Fatal("expected active demo chat")
	}

	_, _ = m1.Update(olderMessagesLoadedMsg{
		chatID: ch.ChatID,
		messages: []storage.LiveMessage{
			{MessageID: 5, ChatID: ch.ChatID, Sender: "Alice", Text: "persisted old message"},
			{MessageID: 6, ChatID: ch.ChatID, Sender: "You", Text: "persisted reply"},
		},
		hasMore: false,
	})

	// Simulate app restart with the same DataDir
	m2 := NewModel(cfg, nil, nil, true)
	if len(m2.liveMessages) == 0 || m2.liveMessages[0].MessageID != 5 {
		t.Fatalf("expected persisted message ID 5 after restart, got %+v", m2.liveMessages)
	}
	if hasMore, known := m2.liveHasMore[ch.ChatID]; !known || hasMore {
		t.Fatalf("expected persisted liveHasMore=false after restart, got known=%v hasMore=%v", known, hasMore)
	}
}
