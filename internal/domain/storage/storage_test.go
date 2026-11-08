package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFavoriteChatsStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fav_chats_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Loading from empty dir returns empty slice
	favs := LoadFavoriteChats(tempDir)
	if len(favs) != 0 {
		t.Errorf("expected 0 favorite chats initially, got %d", len(favs))
	}

	// 2. Save records
	records := []FavoriteChatRecord{
		{ChatID: 1001, Title: "Chat 1", ChatType: "group", PinnedAt: "2026-11-08 10:00:00"},
		{ChatID: 1002, Title: "Chat 2", ChatType: "dm", Username: "user2", PinnedAt: "2026-11-08 10:05:00"},
	}
	if err := SaveFavoriteChats(tempDir, records); err != nil {
		t.Fatalf("failed to save favorite chats: %v", err)
	}

	// File should exist
	if _, err := os.Stat(filepath.Join(tempDir, "favorite_chats.json")); err != nil {
		t.Errorf("favorite_chats.json file not created: %v", err)
	}

	// 3. Load back
	loaded := LoadFavoriteChats(tempDir)
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded favorite chats, got %d", len(loaded))
	}
	if loaded[0].ChatID != 1001 || loaded[0].Title != "Chat 1" {
		t.Errorf("unexpected record 0: %+v", loaded[0])
	}
	if loaded[1].ChatID != 1002 || loaded[1].Username != "user2" {
		t.Errorf("unexpected record 1: %+v", loaded[1])
	}

	// 4. Save empty list
	if err := SaveFavoriteChats(tempDir, []FavoriteChatRecord{}); err != nil {
		t.Fatalf("failed to save empty list: %v", err)
	}
	loadedEmpty := LoadFavoriteChats(tempDir)
	if len(loadedEmpty) != 0 {
		t.Errorf("expected 0 records after saving empty list, got %d", len(loadedEmpty))
	}
}

func TestHiddenChatsStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "hidden_chats_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Loading from empty dir returns empty slice
	hidden := LoadHiddenChats(tempDir)
	if len(hidden) != 0 {
		t.Errorf("expected 0 hidden chats initially, got %d", len(hidden))
	}

	// 2. Save records
	records := []HiddenChatRecord{
		{ChatID: 2001, Title: "Spam Channel", ChatType: "channel", HiddenAt: "2026-11-08 11:00:00"},
		{ChatID: 2002, Title: "Muted Group", ChatType: "group", Username: "muted_grp", HiddenAt: "2026-11-08 11:05:00"},
	}
	if err := SaveHiddenChats(tempDir, records); err != nil {
		t.Fatalf("failed to save hidden chats: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "hidden_chats.json")); err != nil {
		t.Errorf("hidden_chats.json file not created: %v", err)
	}

	// 3. Load back
	loaded := LoadHiddenChats(tempDir)
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded hidden chats, got %d", len(loaded))
	}
	if loaded[0].ChatID != 2001 || loaded[0].Title != "Spam Channel" {
		t.Errorf("unexpected record 0: %+v", loaded[0])
	}

	// 4. Save empty list
	if err := SaveHiddenChats(tempDir, []HiddenChatRecord{}); err != nil {
		t.Fatalf("failed to save empty list: %v", err)
	}
	loadedEmpty := LoadHiddenChats(tempDir)
	if len(loadedEmpty) != 0 {
		t.Errorf("expected 0 records after saving empty list, got %d", len(loadedEmpty))
	}
}

