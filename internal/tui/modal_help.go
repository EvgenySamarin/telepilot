/*
File: internal/tui/modal_help.go
Description: Renderer for the keyboard shortcuts and workflow Help modal.
*/

package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func (m *Model) renderHelpModal() string {
	modalW := 80
	if m.width-6 < modalW {
		modalW = m.width - 6
	}
	if modalW < 50 {
		modalW = 50
	}
	innerW := modalW - 6

	var lines []string
	lines = append(lines, lipgloss.NewStyle().Foreground(colCyan).Bold(true).Render(m.T("help_modal_title")), "")

	type shortcutItem struct {
		key  string
		desc string
	}

	var items []shortcutItem
	if m.cfg.IsRussian() {
		items = []shortcutItem{
			{"1 - 4", "Вкладки (1:Чат, 2:ИИ и навыки, 3:Результат ИИ, 4:Логи)"},
			{"Tab / h / l", "Переключение фокуса между списком чатов и главным окном"},
			{"j / k или Вверх/Вниз", "Перемещение курсора / прокрутка активного окна"},
			{"g / G, Ctrl+d/Ctrl+u", "Быстрая навигация по списку сообщений или результату ИИ"},
			{"Enter / Space (Чаты)", "Открыть выбранный чат (только 1 активный чат)"},
			{"p / f (Чаты)", "Закрепить/открепить чат наверху списка"},
			{"H / x (Чаты)", "Скрыть чат из списка (управление в Настройках 'C')"},
			{"\\ или /", "Быстрый Vim-поиск по окну чатов (Enter выбрать, Esc сброс)"},
			{"i / Enter (Вкл. 1)", "Написать сообщение в чат (Enter отправить, Esc отмена)"},
			{"v (Вкл. 1 / 2 / 3)", "Прослушать голос / открыть медиа (QuickLook) / навык / отчет"},
			{"Enter (Вкл. 2)", "Включить выбранный ИИ-навык для текущего чата ([AI])"},
			{"x (Вкл. 2)", "Отключить ИИ-инструкции для текущего чата"},
			{"m (Вкл. 1 / 2 / 3)", "Разовая команда ИИ для текущего чата (запуск / стоп)"},
			{"e / O / o (Вкл. 2)", "Редакт. в $EDITOR (e), macOS редакторе (O), открыть в Finder (o)"},
			{"n / I (Вкл. 2)", "Создать новый навык (n) или импортировать через Finder (I)"},
			{"r (Вкл. 1 / 2 / 3)", "Обновить чат (Вкл. 1) или запустить ИИ-навык (Вкл. 2/3)"},
			{"o (Вкл. 1 / 2 / 3)", "Показать кэш-медиа (Вкл. 1), навык (Вкл. 2) или отчет (Вкл. 3) в Finder"},
			{"c / C / L", "ИИ-движок (c), Настройки языка (C), Вход по QR (L)"},
			{"q / Esc", "Выход или закрытие модального окна"},
		}
	} else {
		items = []shortcutItem{
			{"1 - 4", "Switch tabs (1:Live Chat, 2:AI & Skills, 3:AI Output, 4:Trace)"},
			{"Tab / h / l", "Switch focus between Chats sidebar and Main window"},
			{"j / k or Up/Dn", "Move cursor down / up or scroll active view"},
			{"g / G, Ctrl+d/Ctrl+u", "Jump to top/bottom or half-page scroll"},
			{"Enter / Space (Chat)", "Select & open chat in Tab 1 (single-chat selection)"},
			{"p / f (Chat)", "Pin / unpin favourite chat on top of list"},
			{"H / x (Chat)", "Hide chat from list (manage in Settings 'C')"},
			{"\\ or /", "Vim-style search in Chats window (Enter open, Esc clear)"},
			{"i / Enter (Tab 1)", "Compose message in live chat (Enter to send, Esc to cancel)"},
			{"v (Tab 1 / 2 / 3)", "Play voice / QuickLook media / Preview skill / QuickLook output"},
			{"Enter (Tab 2)", "Enable highlighted AI skill for active chat ([AI])"},
			{"x (Tab 2)", "Disable/clear AI instructions for active chat"},
			{"m (Tab 1 / 2 / 3)", "Run or stop single-shot AI instruction for active chat"},
			{"e / O / o (Tab 2)", "Edit skill in $EDITOR (e), macOS editor (O), or Finder (o)"},
			{"n / I (Tab 2)", "Create new skill in terminal (n) or import via Finder (I)"},
			{"r (Tab 1 / 2 / 3)", "Refresh live chat (Tab 1) or Run active AI skill (Tab 2/3)"},
			{"o (Tab 1 / 2 / 3)", "Reveal cached media (Tab 1), skill (Tab 2), or output (Tab 3) in Finder"},
			{"c / C / L", "AI Engine config (c), Global settings (C), QR Login (L)"},
			{"q / Esc", "Quit or close modal"},
		}
	}

	keyColW := 0
	for _, it := range items {
		w := lipgloss.Width(it.key)
		if w > keyColW {
			keyColW = w
		}
	}

	prefixW := 2 + keyColW + 3
	descW := innerW - prefixW
	if descW < 20 {
		descW = 20
	}
	indent := strings.Repeat(" ", prefixW)

	for _, it := range items {
		prefix := "  " + padRightVisual(it.key, keyColW) + " : "
		wrapped := wrapTextBlock(it.desc, descW)
		if len(wrapped) == 0 {
			lines = append(lines, prefix)
			continue
		}
		lines = append(lines, prefix+wrapped[0])
		for _, wl := range wrapped[1:] {
			lines = append(lines, indent+wl)
		}
	}

	return lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(colBorderFoc).
		Padding(1, 2).
		Width(modalW).
		Render(strings.Join(lines, "\n"))
}
