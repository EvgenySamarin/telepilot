/*
File: internal/config/config_test.go
Description: Unit tests for default language configuration and persistence.
*/

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigLanguageDefaultAndSave(t *testing.T) {
	tempDir := t.TempDir()
	envPath := filepath.Join(tempDir, ".env")

	cfg := &AppConfig{
		ProjectRoot:        tempDir,
		TelegramAPIID:      12345,
		TelegramAPIHash:    "abcde12345",
		TelegramServerIP:   "149.154.167.50",
		TelegramServerPort: 443,
		AgyModel:           "auto",
		Language:           "en",
	}

	// 1. Initial save with language "en"
	cfg.SaveCredentials(cfg.TelegramAPIID, cfg.TelegramAPIHash, cfg.TelegramServerIP, cfg.TelegramServerPort, cfg.AgyModel)

	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}
	if !strings.Contains(string(data), "APP_LANGUAGE=en") {
		t.Errorf("expected .env to contain APP_LANGUAGE=en, got:\n%s", string(data))
	}
	if cfg.IsRussian() {
		t.Errorf("expected IsRussian() to be false for 'en'")
	}

	// 2. Switch language to Russian
	cfg.SaveLanguage("ru")
	if !cfg.IsRussian() {
		t.Errorf("expected IsRussian() to be true after SaveLanguage('ru')")
	}

	data, err = os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("failed to read .env: %v", err)
	}
	if !strings.Contains(string(data), "APP_LANGUAGE=ru") {
		t.Errorf("expected .env to contain APP_LANGUAGE=ru, got:\n%s", string(data))
	}

	// 3. Test Load reading from .env
	t.Setenv("TELEPILOT_HOME", tempDir)
	loaded := Load()
	if loaded.Language != "ru" {
		t.Errorf("Load() got Language = %q; want 'ru'", loaded.Language)
	}
	if !loaded.IsRussian() {
		t.Errorf("loaded.IsRussian() should be true")
	}

	// 4. Switch back to En
	cfg.SaveLanguage("en")
	loaded = Load()
	if loaded.Language != "en" {
		t.Errorf("Load() got Language = %q; want 'en'", loaded.Language)
	}
	if loaded.IsRussian() {
		t.Errorf("loaded.IsRussian() should be false")
	}
}
