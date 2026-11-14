package storage

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatHistorySaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	chatID := int64(424242)

	if _, found, err := LoadChatHistory(dir, chatID); err != nil || found {
		t.Fatalf("expected not found on empty dir, got found=%v err=%v", found, err)
	}

	msgs := []LiveMessage{
		{MessageID: 10, ChatID: chatID, Sender: "Alice", Text: "older"},
		{MessageID: 11, ChatID: chatID, Sender: "You", Text: "newer"},
	}
	if err := SaveChatHistory(dir, chatID, msgs, false); err != nil {
		t.Fatalf("SaveChatHistory failed: %v", err)
	}

	loaded, found, err := LoadChatHistory(dir, chatID)
	if err != nil || !found {
		t.Fatalf("LoadChatHistory failed: found=%v err=%v", found, err)
	}
	if loaded.HasMore {
		t.Fatal("expected HasMore=false")
	}
	if len(loaded.Messages) != 2 || loaded.Messages[0].MessageID != 10 || loaded.Messages[1].MessageID != 11 {
		t.Fatalf("unexpected loaded messages: %+v", loaded.Messages)
	}
}

func TestSaveChatSkillOutputsUsesRunDateInFilename(t *testing.T) {
	dir := t.TempDir()
	chatID := int64(101)

	outputs := map[int64]SkillOutputRecord{
		chatID: {
			ChatID:    chatID,
			ChatTitle: "Anna Kovaleva",
			Skill:     "rental-statement-consolidator",
			CreatedAt: "2026-11-14 20:00:00 UTC",
			Output:    "first run",
		},
	}
	if err := SaveChatSkillOutputs(dir, outputs); err != nil {
		t.Fatalf("first SaveChatSkillOutputs failed: %v", err)
	}
	firstPath := outputs[chatID].OutputPath
	wantSuffix := filepath.Join("anna_kovaleva", "20261114-output.md")
	if !strings.HasSuffix(firstPath, wantSuffix) {
		t.Fatalf("expected path ending with %q, got %q", wantSuffix, firstPath)
	}

	// Second run on a different timestamp must create a new file, not overwrite the first
	outputs[chatID] = SkillOutputRecord{
		ChatID:    chatID,
		ChatTitle: "Anna Kovaleva",
		Skill:     "rental-statement-consolidator",
		CreatedAt: "2026-11-14 21:30:00 UTC",
		Output:    "second run",
	}
	if err := SaveChatSkillOutputs(dir, outputs); err != nil {
		t.Fatalf("second SaveChatSkillOutputs failed: %v", err)
	}
	secondPath := outputs[chatID].OutputPath
	if secondPath == firstPath {
		t.Fatalf("expected second run to use a different filename, both got %q", secondPath)
	}
	if _, err := os.Stat(firstPath); err != nil {
		t.Fatalf("expected first output file %q to still exist: %v", firstPath, err)
	}
	if _, err := os.Stat(secondPath); err != nil {
		t.Fatalf("expected second output file %q to exist: %v", secondPath, err)
	}
}
