/*
File: internal/storage/storage_test.go
Description: Unit tests for favourite chats, hidden chats, and scheduled tasks storage.
*/

package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFavoriteChatsStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "fav_chats_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Loading from empty dir returns empty slice
	favs, err := LoadFavoriteChats(tempDir)
	if err != nil {
		t.Fatalf("unexpected error loading empty favorite chats: %v", err)
	}
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
	loaded, err := LoadFavoriteChats(tempDir)
	if err != nil {
		t.Fatalf("failed to load favorite chats: %v", err)
	}
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
	loadedEmpty, err := LoadFavoriteChats(tempDir)
	if err != nil {
		t.Fatalf("failed to load empty favorite chats: %v", err)
	}
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
	hidden, err := LoadHiddenChats(tempDir)
	if err != nil {
		t.Fatalf("unexpected error loading empty hidden chats: %v", err)
	}
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
	loaded, err := LoadHiddenChats(tempDir)
	if err != nil {
		t.Fatalf("failed to load hidden chats: %v", err)
	}
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
	loadedEmpty, err := LoadHiddenChats(tempDir)
	if err != nil {
		t.Fatalf("failed to load empty hidden chats: %v", err)
	}
	if len(loadedEmpty) != 0 {
		t.Errorf("expected 0 records after saving empty list, got %d", len(loadedEmpty))
	}
}

func TestScheduledTasksStorage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "sched_tasks_test_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// 1. Loading from empty dir returns empty slice
	tasks, err := LoadScheduledTasks(tempDir)
	if err != nil {
		t.Fatalf("unexpected error loading empty scheduled tasks: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 scheduled tasks initially, got %d", len(tasks))
	}

	// 2. Save records
	now := time.Now().Truncate(time.Second)
	records := []ScheduledTask{
		{
			ID:          "task_1",
			ChatID:      3001,
			ChatTitle:   "Work Chat",
			TaskPrompt:  "Check deployment status",
			DueAt:       now.Add(30 * time.Minute),
			IntervalStr: "none",
			CreatedAt:   now,
		},
		{
			ID:          "task_2",
			ChatID:      3002,
			ChatTitle:   "Health Chat",
			TaskPrompt:  "Drink water",
			DueAt:       now.Add(2 * time.Hour),
			IntervalStr: "2h",
			CreatedAt:   now,
		},
	}
	if err := SaveScheduledTasks(tempDir, records); err != nil {
		t.Fatalf("failed to save scheduled tasks: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tempDir, "scheduled_tasks.json")); err != nil {
		t.Errorf("scheduled_tasks.json file not created: %v", err)
	}

	// 3. Load back
	loaded, err := LoadScheduledTasks(tempDir)
	if err != nil {
		t.Fatalf("failed to load scheduled tasks: %v", err)
	}
	if len(loaded) != 2 {
		t.Fatalf("expected 2 loaded scheduled tasks, got %d", len(loaded))
	}
	if loaded[0].ID != "task_1" || loaded[0].ChatID != 3001 || loaded[0].TaskPrompt != "Check deployment status" {
		t.Errorf("unexpected task 0: %+v", loaded[0])
	}
	if loaded[1].ID != "task_2" || loaded[1].IntervalStr != "2h" {
		t.Errorf("unexpected task 1: %+v", loaded[1])
	}

	// 4. Save empty list
	if err := SaveScheduledTasks(tempDir, []ScheduledTask{}); err != nil {
		t.Fatalf("failed to save empty list: %v", err)
	}
	loadedEmpty, err := LoadScheduledTasks(tempDir)
	if err != nil {
		t.Fatalf("failed to load empty scheduled tasks: %v", err)
	}
	if len(loadedEmpty) != 0 {
		t.Errorf("expected 0 records after saving empty list, got %d", len(loadedEmpty))
	}
}

