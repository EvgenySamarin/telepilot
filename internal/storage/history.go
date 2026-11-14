/*
File: internal/storage/history.go
Description: Per-chat persistent live message history cache stored under data/live_history/<chatID>.json.
*/

package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const maxPersistedHistoryMessages = 5000

// ChatHistoryCache stores persisted live messages and pagination state for a single chat.
type ChatHistoryCache struct {
	ChatID   int64         `json:"chat_id"`
	HasMore  bool          `json:"has_more"`
	Messages []LiveMessage `json:"messages"`
}

func chatHistoryPath(dataDir string, chatID int64) string {
	return filepath.Join(dataDir, "live_history", fmt.Sprintf("%d.json", chatID))
}

// LoadChatHistory loads persisted messages for chatID from <dataDir>/live_history/<chatID>.json.
func LoadChatHistory(dataDir string, chatID int64) (ChatHistoryCache, bool, error) {
	if dataDir == "" || chatID == 0 {
		return ChatHistoryCache{HasMore: true}, false, nil
	}
	raw, err := os.ReadFile(chatHistoryPath(dataDir, chatID))
	if err != nil {
		if os.IsNotExist(err) {
			return ChatHistoryCache{HasMore: true}, false, nil
		}
		return ChatHistoryCache{HasMore: true}, false, err
	}
	var cache ChatHistoryCache
	if err := json.Unmarshal(raw, &cache); err != nil {
		return ChatHistoryCache{HasMore: true}, false, err
	}
	if len(cache.Messages) == 0 {
		return ChatHistoryCache{HasMore: true}, false, nil
	}
	return cache, true, nil
}

// SaveChatHistory atomically persists messages (up to maxPersistedHistoryMessages) for chatID.
func SaveChatHistory(dataDir string, chatID int64, messages []LiveMessage, hasMore bool) error {
	if dataDir == "" || chatID == 0 || len(messages) == 0 {
		return nil
	}
	dir := filepath.Join(dataDir, "live_history")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	toSave := messages
	if len(toSave) > maxPersistedHistoryMessages {
		toSave = toSave[len(toSave)-maxPersistedHistoryMessages:]
	}
	payload := ChatHistoryCache{
		ChatID:   chatID,
		HasMore:  hasMore,
		Messages: toSave,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	target := chatHistoryPath(dataDir, chatID)
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}
