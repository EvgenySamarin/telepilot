package storage

import "testing"

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
