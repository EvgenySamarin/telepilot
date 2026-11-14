package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"telepilot/internal/domain/ai"
	"telepilot/internal/domain/storage"
)

// cancelScheduledTasksForChat removes all scheduled tasks for the specified chat.
func (m *Model) cancelScheduledTasksForChat(chatID int64, chatTitle string) int {
	var remaining []storage.ScheduledTask
	var cancelled int
	for _, t := range m.scheduledTasks {
		if t.ChatID == chatID {
			cancelled++
		} else {
			remaining = append(remaining, t)
		}
	}
	if cancelled > 0 {
		m.scheduledTasks = remaining
		_ = storage.SaveScheduledTasks(m.cfg.DataDir, m.scheduledTasks)
		m.addTrace(fmt.Sprintf("🛑 Stopped %d scheduled task(s) for '%s'", cancelled, chatTitle))
	}
	return cancelled
}

// cancelAllScheduledTasks removes all scheduled tasks across all chats.
func (m *Model) cancelAllScheduledTasks() int {
	cancelled := len(m.scheduledTasks)
	if cancelled > 0 {
		m.scheduledTasks = nil
		_ = storage.SaveScheduledTasks(m.cfg.DataDir, m.scheduledTasks)
		m.addTrace(fmt.Sprintf("🛑 Stopped ALL %d background scheduled task(s)", cancelled))
	}
	return cancelled
}

func (m *Model) checkScheduledTasksCmd() []tea.Cmd {
	if len(m.scheduledTasks) == 0 {
		return nil
	}
	now := time.Now()
	var due []storage.ScheduledTask
	var remaining []storage.ScheduledTask
	for _, t := range m.scheduledTasks {
		if !t.DueAt.After(now) {
			due = append(due, t)
		} else {
			remaining = append(remaining, t)
		}
	}
	if len(due) == 0 {
		return nil
	}

	var cmds []tea.Cmd
	for _, task := range due {
		task.RunsCount++
		shouldContinue := true

		if task.MaxRuns > 0 && task.RunsCount >= task.MaxRuns {
			shouldContinue = false
			m.addTrace(fmt.Sprintf("⏰ Scheduled task for '%s' reached target runs (%d/%d), stopped.", task.ChatTitle, task.RunsCount, task.MaxRuns))
		}
		if task.Until != nil && !task.Until.IsZero() && now.After(*task.Until) {
			shouldContinue = false
			m.addTrace(fmt.Sprintf("⏰ Scheduled task for '%s' reached expiration limit, stopped.", task.ChatTitle))
		}

		if shouldContinue && task.IntervalStr != "" && task.IntervalStr != "none" {
			if dur, err := ai.ParseScheduleDuration(task.IntervalStr); err == nil && dur > 0 {
				nextDue := task.DueAt.Add(dur)
				for !nextDue.After(now) {
					nextDue = nextDue.Add(dur)
				}
				task.DueAt = nextDue
				remaining = append(remaining, task)
			}
		}
		cmds = append(cmds, m.executeScheduledTaskCmd(task))
	}
	m.scheduledTasks = remaining
	_ = storage.SaveScheduledTasks(m.cfg.DataDir, m.scheduledTasks)
	return cmds
}
