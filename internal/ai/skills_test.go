/*
File: internal/ai/skills_test.go
Description: Unit tests verifying that AI skill prompts and outputs respect the
configured application language.
*/

package ai

import (
	"context"
	"strings"
	"testing"

	"telepilot/internal/config"
	"telepilot/internal/storage"
)

func TestSkillsFollowPreferredLanguage(t *testing.T) {
	tempDir := t.TempDir()

	// 1. English Config
	cfgEn := &config.AppConfig{
		ProjectRoot: tempDir,
		Language:    "en",
		AgyModel:    "auto",
	}
	engineEn := NewEngine(cfgEn)

	resEn := engineEn.GeneratePeriodSummary(
		context.Background(),
		"Test Chat",
		"2026-10-01 -> 2026-11-01 (1m)",
		"period-summary",
		"",
		[]storage.MessageRecord{
			{MessageID: 1, Sender: "Alice", Text: "Hello world"},
		},
		[]storage.MediaItemRecord{},
	)

	// Since agy CLI is not configured, it triggers the deterministic local summary fallback.
	// In En mode, it should contain English text.
	if !strings.Contains(resEn.ExecutiveSummary, "Parsed 1 messages") {
		t.Errorf("expected English summary fallback, got: %s", resEn.ExecutiveSummary)
	}
	if !strings.Contains(resEn.ActionItems, "Archive messages") {
		t.Errorf("expected English action items, got: %s", resEn.ActionItems)
	}

	// 2. Russian Config
	cfgRu := &config.AppConfig{
		ProjectRoot: tempDir,
		Language:    "ru",
		AgyModel:    "auto",
	}
	engineRu := NewEngine(cfgRu)

	resRu := engineRu.GeneratePeriodSummary(
		context.Background(),
		"Тестовый чат",
		"2026-10-01 -> 2026-11-01 (1m)",
		"period-summary",
		"",
		[]storage.MessageRecord{
			{MessageID: 1, Sender: "Алиса", Text: "Привет мир"},
		},
		[]storage.MediaItemRecord{},
	)

	// In Ru mode, fallback must follow Russian language
	if !strings.Contains(resRu.ExecutiveSummary, "Обработано сообщений: 1") {
		t.Errorf("expected Russian summary fallback, got: %s", resRu.ExecutiveSummary)
	}
	if !strings.Contains(resRu.ActionItems, "Архивировать сообщения") {
		t.Errorf("expected Russian action items, got: %s", resRu.ActionItems)
	}
	if !strings.Contains(resRu.VoiceMediaInsights, "В этом периоде нет голосовых сообщений") {
		t.Errorf("expected Russian media insights, got: %s", resRu.VoiceMediaInsights)
	}
}
