/*
File: internal/presentation/tui/ai_state.go
Description: Per-chat AI configuration helpers, skills reload/reset/disk-sync, initial
AI/quota status lines, trace logging, and confirmation-only reply detection.
*/

package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"telepilot/internal/domain/config"
	"telepilot/internal/domain/skills"
	"telepilot/internal/domain/storage"
)

func (m *Model) isChatAIEnabled(chatID int64) bool {
	if m.chatAIConfigs == nil {
		return false
	}
	cfg, ok := m.chatAIConfigs[chatID]
	return ok && cfg.Enabled
}

func (m *Model) activeChatAIConfig() (storage.ChatAIConfig, bool) {
	ch := m.activeChat()
	if ch == nil || m.chatAIConfigs == nil {
		return storage.ChatAIConfig{}, false
	}
	cfg, ok := m.chatAIConfigs[ch.ChatID]
	if !ok || !cfg.Enabled {
		return storage.ChatAIConfig{
			ChatID:     ch.ChatID,
			ChatTitle:  ch.Title,
			Enabled:    false,
			Skill:      config.DefaultSkillName,
			Notes:      "",
			PeriodSpec: m.cfg.DefaultPeriod,
		}, false
	}
	if cfg.PeriodSpec == "" {
		cfg.PeriodSpec = m.cfg.DefaultPeriod
	}
	if cfg.Skill == "" {
		cfg.Skill = config.DefaultSkillName
	}
	if cfg.SendMode == "" {
		cfg.SendMode = "draft"
	}
	return cfg, true
}

func (m *Model) syncCurrentChatAISettings() {
	if cfg, ok := m.activeChatAIConfig(); ok {
		m.activeSkill = cfg.Skill
		m.runNotes = cfg.Notes
		if cfg.PeriodSpec != "" {
			m.periodSpec = cfg.PeriodSpec
		}
	} else {
		m.activeSkill = m.cfg.DefaultSkill
		if m.activeSkill == "" {
			m.activeSkill = config.DefaultSkillName
		}
		m.runNotes = ""
		if m.cfg.DefaultPeriod != "" {
			m.periodSpec = m.cfg.DefaultPeriod
		} else {
			m.periodSpec = "1m"
		}
	}
}

func (m *Model) saveCurrentChatAIConfig(skillName, notes, periodSpec string, enabled bool) {
	ch := m.activeChat()
	if ch == nil {
		return
	}
	if m.chatAIConfigs == nil {
		m.chatAIConfigs = make(map[int64]storage.ChatAIConfig)
	}
	if !enabled {
		delete(m.chatAIConfigs, ch.ChatID)
		_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
		m.syncCurrentChatAISettings()
		return
	}
	if strings.TrimSpace(skillName) == "" {
		skillName = config.DefaultSkillName
	}
	if strings.TrimSpace(periodSpec) == "" {
		periodSpec = "1m"
	}
	prev := m.chatAIConfigs[ch.ChatID]
	sendMode := prev.SendMode
	if sendMode == "" {
		sendMode = "draft"
	}
	cfg := storage.ChatAIConfig{
		ChatID:               ch.ChatID,
		ChatTitle:            ch.Title,
		Enabled:              true,
		Skill:                strings.TrimSpace(skillName),
		Notes:                strings.TrimSpace(notes),
		PeriodSpec:           strings.TrimSpace(periodSpec),
		AutoReply:            prev.AutoReply,
		SendMode:             sendMode,
		LastAutoRepliedMsgID: prev.LastAutoRepliedMsgID,
		UpdatedAt:            time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	}
	m.chatAIConfigs[ch.ChatID] = cfg
	_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
	m.syncCurrentChatAISettings()
}

func (m *Model) toggleChatAutoReply() {
	ch := m.activeChat()
	if ch == nil {
		return
	}
	if m.chatAIConfigs == nil {
		m.chatAIConfigs = make(map[int64]storage.ChatAIConfig)
	}
	cur, ok := m.activeChatAIConfig()
	if !ok {
		cur = storage.ChatAIConfig{
			ChatID:     ch.ChatID,
			ChatTitle:  ch.Title,
			Enabled:    true,
			Skill:      m.activeSkill,
			Notes:      m.runNotes,
			PeriodSpec: m.periodSpec,
			SendMode:   "draft",
		}
	}
	cur.Enabled = true
	cur.AutoReply = !cur.AutoReply
	if cur.SendMode == "" {
		cur.SendMode = "draft"
	}
	if len(m.liveMessages) > 0 {
		cur.LastAutoRepliedMsgID = m.liveMessages[len(m.liveMessages)-1].MessageID
	}
	cur.UpdatedAt = time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	m.chatAIConfigs[ch.ChatID] = cur
	_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
	m.syncCurrentChatAISettings()
	m.addTrace(fmt.Sprintf("Chat '%s': AI Auto-Reply set to %v (SendMode=%s)", ch.Title, cur.AutoReply, cur.SendMode))
}

func (m *Model) toggleChatSendMode() {
	ch := m.activeChat()
	if ch == nil {
		return
	}
	if m.chatAIConfigs == nil {
		m.chatAIConfigs = make(map[int64]storage.ChatAIConfig)
	}
	cur, ok := m.activeChatAIConfig()
	if !ok {
		cur = storage.ChatAIConfig{
			ChatID:     ch.ChatID,
			ChatTitle:  ch.Title,
			Enabled:    true,
			Skill:      m.activeSkill,
			Notes:      m.runNotes,
			PeriodSpec: m.periodSpec,
			SendMode:   "draft",
		}
	}
	cur.Enabled = true
	if cur.SendMode == "auto_send" {
		cur.SendMode = "draft"
	} else {
		cur.SendMode = "auto_send"
	}
	cur.UpdatedAt = time.Now().UTC().Format("2006-01-02 15:04:05 UTC")
	m.chatAIConfigs[ch.ChatID] = cur
	_ = storage.SaveChatAIConfigs(m.cfg.DataDir, m.chatAIConfigs)
	m.syncCurrentChatAISettings()
	m.addTrace(fmt.Sprintf("Chat '%s': AI Send Mode set to '%s'", ch.Title, cur.SendMode))
}

func (m *Model) activeSkillOutputRecord() *storage.SkillOutputRecord {
	ch := m.activeChat()
	if ch == nil || m.skillOutputs == nil {
		return nil
	}
	rec, ok := m.skillOutputs[ch.ChatID]
	if !ok || strings.TrimSpace(rec.Output) == "" {
		return nil
	}
	return &rec
}

func (m *Model) reloadSkills() {
	list, errs := skills.Load(m.cfg.SkillsDir())
	m.skillList = list
	m.skillErrs = errs
	if m.skillCursor >= len(m.skillList) {
		m.skillCursor = maxInt(0, len(m.skillList)-1)
	}
	if m.activeSkill == "" {
		m.activeSkill = m.cfg.DefaultSkill
	}
	if m.activeSkill == "" {
		m.activeSkill = config.DefaultSkillName
	}
}

func (m *Model) resetDefaultSkills() {
	if err := skills.ResetDefaults(m.cfg.SkillsDir()); err != nil {
		m.statusText = fmt.Sprintf("Failed to reset skills: %v", err)
		m.addTrace(fmt.Sprintf("Failed to reset default skills: %v", err))
		return
	}
	m.reloadSkills()
	m.statusText = m.T("cfg_modal_reset_skills_success")
	m.addTrace("Restored all bundled default AI skills into " + m.cfg.SkillsDir())
}

// syncSkillsFromDisk checks whether any skills were added, removed, or modified externally
// (e.g. via macOS Finder or a GUI editor) and reloads them in real time without losing cursor state.
func (m *Model) syncSkillsFromDisk() bool {
	list, errs := skills.Load(m.cfg.SkillsDir())
	changed := len(list) != len(m.skillList)
	if !changed {
		for i := range list {
			if list[i].Name != m.skillList[i].Name ||
				list[i].Hash != m.skillList[i].Hash ||
				list[i].Path != m.skillList[i].Path {
				changed = true
				break
			}
		}
	}
	if !changed {
		return false
	}

	var prevSelectedName string
	if m.skillCursor >= 0 && m.skillCursor < len(m.skillList) {
		prevSelectedName = m.skillList[m.skillCursor].Name
	}
	prevNames := make(map[string]bool, len(m.skillList))
	for _, sk := range m.skillList {
		prevNames[sk.Name] = true
	}

	m.skillList = list
	m.skillErrs = errs

	// If a new skill appeared on disk (e.g. added via Finder), move cursor to it; otherwise preserve selection
	selectedIdx := -1
	for idx, sk := range m.skillList {
		if !prevNames[sk.Name] {
			selectedIdx = idx
			break
		}
	}
	if selectedIdx < 0 && prevSelectedName != "" {
		for idx, sk := range m.skillList {
			if sk.Name == prevSelectedName {
				selectedIdx = idx
				break
			}
		}
	}
	if selectedIdx >= 0 {
		m.skillCursor = selectedIdx
	} else if m.skillCursor >= len(m.skillList) {
		m.skillCursor = maxInt(0, len(m.skillList)-1)
	}

	m.addTrace(fmt.Sprintf("Live-reloaded skills from %s (%d skills)", m.cfg.SkillsDir(), len(m.skillList)))
	return true
}

func initialAIStatusLine(cfg *config.AppConfig) string {
	if bin := cfg.AgyCLIBin(); bin != "" {
		model := strings.TrimSpace(cfg.AgyModel)
		if model == "" {
			model = "auto"
		}
		if cfg.IsRussian() {
			return fmt.Sprintf("[OK] Antigravity CLI (agy) готов: %s (модель=%s) — нажмите 'c' для теста или 'r' для запуска ИИ", bin, model)
		}
		return fmt.Sprintf("[OK] Antigravity CLI (agy) ready at %s (model=%s) — press 'c' to test or 'r' to run AI", bin, model)
	}
	if cfg.IsRussian() {
		return "[!] Бинарный файл Antigravity CLI (agy) не найден — нажмите 'c' для настройки пути"
	}
	return "[!] Antigravity CLI (agy) binary not found — press 'c' to configure agy path"
}

func initialAIQuotaStatus(cfg *config.AppConfig) string {
	if cfg.AgyCLIBin() == "" {
		if cfg.IsRussian() {
			return "[ВЫКЛ] agy CLI не найден — нажмите 'c'"
		}
		return "[OFF] agy CLI not found — press 'c'"
	}
	if cfg.IsRussian() {
		return "[OK] Доступна (нажмите 'c' для теста или запустите ИИ)"
	}
	return "[OK] Available (press 'c' to test or run AI)"
}

func (m *Model) updateAIQuotaStatus(err error) {
	ts := time.Now().Format("15:04:05")
	ru := m.cfg.IsRussian()
	if err == nil {
		if ru {
			m.aiQuotaStatus = fmt.Sprintf("[OK] Доступна (обновлено %s)", ts)
		} else {
			m.aiQuotaStatus = fmt.Sprintf("[OK] Available (updated %s)", ts)
		}
		return
	}
	msg := err.Error()
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "quota") || strings.Contains(lower, "exit status 3") || strings.Contains(lower, "rate limit") || strings.Contains(lower, "resource exhausted"):
		if ru {
			m.aiQuotaStatus = fmt.Sprintf("[QUOTA REACHED] Квота исчерпана (%s) — смените модель ('c')", ts)
		} else {
			m.aiQuotaStatus = fmt.Sprintf("[QUOTA REACHED] Individual quota reached (%s) — switch model ('c')", ts)
		}
	case strings.Contains(lower, "signal: killed") || strings.Contains(lower, "deadline exceeded"):
		if ru {
			m.aiQuotaStatus = fmt.Sprintf("[TIMEOUT] Превышен таймаут agy CLI (%s)", ts)
		} else {
			m.aiQuotaStatus = fmt.Sprintf("[TIMEOUT] agy CLI timed out (%s)", ts)
		}
	case strings.Contains(lower, "authentication"):
		if ru {
			m.aiQuotaStatus = fmt.Sprintf("[AUTH REQUIRED] Запустите 'agy' в терминале (%s)", ts)
		} else {
			m.aiQuotaStatus = fmt.Sprintf("[AUTH REQUIRED] Run 'agy' in terminal (%s)", ts)
		}
	default:
		if ru {
			m.aiQuotaStatus = fmt.Sprintf("[ERROR] %s (%s)", truncateSingleLine(msg, 48), ts)
		} else {
			m.aiQuotaStatus = fmt.Sprintf("[ERROR] %s (%s)", truncateSingleLine(msg, 48), ts)
		}
	}
}

func (m *Model) addTrace(msg string) {
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s] %s", ts, msg)
	m.traceLogs = append(m.traceLogs, line)
	if len(m.traceLogs) > 500 {
		m.traceLogs = m.traceLogs[len(m.traceLogs)-500:]
	}
	logPath := m.traceLogFilePath()
	if f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		_, _ = f.WriteString(line + "\n")
		_ = f.Close()
	}
}

func (m *Model) traceLogFilePath() string {
	_ = os.MkdirAll(m.cfg.DataDir, 0o755)
	return filepath.Join(m.cfg.DataDir, "trace.log")
}

func isPureConfirmation(s string) bool {
	lower := strings.ToLower(strings.TrimSpace(s))
	prefixes := []string{
		"понял", "договорились", "хорошо", "буду отправлять", "буду напоминать", "отправка", "серия",
		"привет! серия", "привет! цикл", "пятиминутный цикл", "все 5 сообщений", "все 5 приветствий",
		"цикл отправки", "таймер остановлен", "останавливаю отправку",
		"i will", "understood", "sure", "alright", "got it", "i'll remind", "i'll send",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}
