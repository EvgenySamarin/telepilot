/*
File: internal/presentation/i18n/i18n.go
Description: Internationalization helpers (Normalize, IsRussian, T) providing English
and Russian UI string translations.
*/

package i18n

import (
	"fmt"
	"strings"
)

const (
	En = "en"
	Ru = "ru"
)

/*
Normalize returns either "ru" or "en" (default).
*/
func Normalize(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	if l == "ru" || strings.HasPrefix(l, "ru_") || strings.HasPrefix(l, "ru-") {
		return Ru
	}
	return En
}

/*
IsRussian checks if the language code is Russian.
*/
func IsRussian(lang string) bool {
	return Normalize(lang) == Ru
}

var messages = map[string]map[string]string{
	En: {
		// General / Top / Bottom
		"status_ready":      "Ready",
		"status_busy":       "Busy",
		"selected_chats":    "Selected: %d chats",
		"active_chat_hdr":   "Chat: %s",
		"parsed_chats":      "Skill completed (%s)",
		"key_quit":          "Quit",
		"key_vim_nav":       "Vim Nav",
		"key_focus":         "Focus",
		"key_login":         "QR Login",
		"key_start_parse":   "Run Skill",
		"key_select_chat":   "Open Chat",
		"key_pin_chat":      "Pin/Fav",
		"key_hide_chat":     "Hide",
		"key_search_chats":  "Search Chats",
		"key_compose":       "Compose",
		"key_send":          "Send",
		"key_cancel":        "Cancel",
		"key_refresh":       "Refresh",
		"key_run_ai":        "Run Skill",
		"key_disable_ai":    "Disable AI",
		"key_activate_skill": "Activate Skill",
		"key_ai_config":     "AI Engine",
		"key_global_config": "Settings",
		"key_quicklook":     "QuickLook/Play",
		"key_preview":       "Preview Skill",
		"key_edit_skill":    "Edit Skill",
		"key_mac_edit":      "Mac Editor",
		"key_new_skill":     "New Skill",
		"key_import_skill":  "Import Skill",
		"key_notes":         "Single-Shot AI",
		"key_rerun_ai":      "Run Skill",
		"key_ai_reply":      "AI Reply",
		"key_auto_reply":    "Auto-Reply",
		"key_send_mode":     "Send Mode",
		"key_views":         "Tabs",
		"key_finder":        "Finder",
		"key_help":          "Help",

		// Sidebar
		"sidebar_chats_hdr":      "─Chats (Enter/Space:Open  \\:Search) [%d/%d]─",
		"sidebar_search_active":  " \\ %s█ [%d/%d]",
		"sidebar_search_filter":  " \\ %s [%d/%d] (Esc:Clear)",
		"sidebar_search_no_match": "  No matching chats",
		"sidebar_hidden_notice":  "  👁  %d chat(s) hidden (check Settings 'C')",

		// Main Panel Tabs (4 Tabs)
		"tab_details": "─TelePilot",
		"tab_1":       "1:Live Chat",
		"tab_2":       "2:AI & Skills",
		"tab_3":       "3:AI Output",
		"tab_4":       "4:Trace: full",

		// Tab 1 Live Chat
		"chat_hdr":          " ▸ %s [%s] — Live Chat (%d msgs)  │  Sync: Push + 15s",
		"chat_empty":        "  No messages in this chat yet. Press 'i' or Enter to send a message.",
		"chat_input_idle":   " Press 'i' or Enter to message %s...",
		"chat_input_active": " Compose to %s (Enter: Send  │  Esc: Cancel):",
		"chat_media_hint":   "[Press 'v' to preview/play, 'o' in Finder]",
		"chat_media_cached": "[Cached: %s — 'v' QuickLook • 'o' Finder]",

		// Tab 3 AI Skill Output
		"output_tab_hdr":     " ▸ %s — AI Skill Output  (r:Run Skill  m:Single-Shot  v:QuickLook  o:Finder)",
		"output_card_meta":   "Skill & Execution Metadata",
		"output_meta_body":   "Chat:         %s (ID: %d)\nActive Skill: %s (%s)\nModel & Time: %s  |  %s\nMessages:     %d live chat messages analyzed\nOutput File:  %s",
		"output_card_result": "AI Skill Output (%s)",
		"output_empty":       "  No AI skill output for this chat yet. Press 'r' (in Tab 2 or Tab 3) to run the active skill on this chat.",
		"sum_extra_notes":    "Extra Notes: ",

		// Tab 2 Per-Chat AI & Skills
		"skills_tab_hdr":        " ▸ Per-Chat AI & Skills — Enter:Skill  a:Auto-Reply  s:Send Mode  A:AI Reply  m:Single-Shot  x:Disable  r:Run Skill",
		"skills_card_chat_ai":   "Per-Chat AI Instructions: %s",
		"skills_chat_enabled":   "AI Status:\n    [ON] [ACTIVE] (Highlighted in Chats sidebar)\nActive Skill:\n    %s\nMessage Control:\n    Auto-Reply: %s (press 'a')\n    Send Mode: %s (press 's') | Press 'a' in Tab 1 to reply\nSingle-Shot:\n    %s (press 'm' to run/append/stop)",
		"skills_chat_disabled":  "AI Status:\n    [OFF] [NOT CONFIGURED] (Default — no AI instructions active for this chat)\nActive Skill:\n    [OFF] (%s)\nMessage Control:\n    Auto-Reply: [OFF] (manual via 'a' in Tab 1) (press 'a')\n    Send Mode: Draft to Compose Box (Enter to confirm) (press 's') | Press 'a' in Tab 1 to reply\nSingle-Shot:\n    [OFF] (none) (press 'm' to run/append/stop)",
		"skills_autoreply_on":   "[ON] (AI replies to incoming msgs)",
		"skills_autoreply_off":  "[OFF] (manual via 'a' in Tab 1)",
		"skills_sendmode_auto":  "Auto-Send Immediately",
		"skills_sendmode_draft": "Draft to Compose Box (Enter to confirm)",
		"skills_card_ai_config": "Common AI Engine Configuration (Hotkey 'c')",
		"skills_card_catalog":   "Skills Library (.agents/skills/) [%d/%d]",
		"skills_catalog_keys":   "Keys: Enter:Assign  v:Preview  e:Edit  O:Mac Edit  o:Finder  n:New  I:Import  x:Disable",
		"skills_card_run":       "Selected Chat Latest AI Skill Output",
		"skills_card_actions":   "AI Actions & Shortcuts",
		"skills_lbl_engine":     "Engine & Model:\n    %s | Model: %s (--model %s)",
		"skills_lbl_quota":      "Quota & Health:\n    %s",
		"skills_lbl_skill":      "Chat Skill & Directory:\n    %s | %s",
		"skills_lbl_diag":       "Last Diagnostic:\n    %s",
		"skills_empty_lib":      "No skills found. Press 'n' to create one.",
		"skills_badge_active":   "[ACTIVE]",
		"skills_badge_summary":  "[SKILL]",
		"skills_output_meta":    "Last Skill Run:\n    %s | Skill: %s | Model: %s (%d msgs)",
		"skills_output_preview": "Output Preview (Tab 3):\n    %s",
		"skills_period_notes":   "Single-Shot:\n    %s",
		"skills_output_none":    "Output Status:\n    [OFF] No AI skill output generated for this chat yet (press 'r' to run skill)\nSingle-Shot:\n    %s",
		"skills_notes_none":     "[OFF] (none)",
		"skills_sched_active":   "Scheduled Tasks:\n    [ACTIVE] %d task(s) (Next in %s)",
		"skills_sched_none":     "Scheduled Tasks:\n    [OFF] (none)",

		// Tab 4 Trace
		"trace_hdr": " ▸ Real-Time Execution & MTProto Diagnostic Trace",

		// Global Config Modal
		"cfg_modal_title":             "─Global Configuration (TelePilot)─",
		"cfg_modal_banner":            "Manage application preferences, preferred language, and system paths.",
		"cfg_modal_lang":              "Interface Language:",
		"cfg_modal_favs_title":        "Favourite / Pinned Chats (%d):",
		"cfg_modal_favs_empty":        "  (No favourite chats pinned yet. Press 'p' on any chat to pin it.)",
		"cfg_modal_favs_deleted":      "[Deleted / Remote]",
		"cfg_modal_hidden_title":      "Hidden Chats (%d):",
		"cfg_modal_hidden_desc":       "  Excluded from MTProto live sync & polling (reduces network/CPU load).",
		"cfg_modal_hidden_empty":      "  (No hidden chats. Press 'H' in the sidebar to hide noisy chats & optimize sync.)",
		"cfg_modal_btn_enter":         "Go Inside Block (Enter)",
		"cfg_modal_block_nav_hint":    "j/k: Select Block │ Enter: Go Inside │ Ctrl+X: Stop Tasks │ Ctrl+R: Reset Skills │ Esc: Close",
		"cfg_modal_entered_lang_hint": "j/k or Space: Choose Language │ Enter: Confirm │ Esc: Back to Blocks",
		"cfg_modal_entered_favs_hint": "j/k: Navigate │ d/x: Delete Chat │ Esc: Back to Blocks",
		"cfg_modal_entered_hidden_hint": "j/k: Navigate │ d/x/u: Unhide Chat │ Esc: Back to Blocks",
		"cfg_modal_btn_reset_skills":  "Reset Default Skills (Ctrl+R)",
		"cfg_modal_reset_skills_success": "Default AI skills restored successfully",
		"cfg_modal_btn_save":          "Save & Close (Ctrl+S)",
		"cfg_modal_btn_cancel":        "Cancel / Close (Esc)",
		"cfg_modal_hint":              "Hotkey tip: Capital 'C' opens this Global Config window anytime from main view.",
		"cfg_modal_info_model":  "Active AI Model:      %s",
		"cfg_modal_info_skill":  "Default Skill:        %s",
		"cfg_modal_info_skills": "Skills Directory:     %s",
		"cfg_modal_info_export": "Data Directory:       %s",
		"cfg_modal_info_server": "Telegram Server:      %s",

		// AI Config Modal
		"ai_modal_title":      "─Connect & Configure AI Engine (Antigravity CLI — agy)─",
		"ai_modal_path":       "AGY CLI Path:  ",
		"ai_modal_model":      "AGY Model:     ",
		"ai_modal_btn_test":   "Save & Test AGY (Enter / Ctrl+T)",
		"ai_modal_btn_cycle":  "Cycle Model Preset (F2)",
		"ai_modal_btn_save":   "Save & Close (Ctrl+S)",
		"ai_modal_btn_cancel": "Cancel / Close (Esc)",

		// Help Modal
		"help_modal_title": "─TelePilot (tgp) Vim Keyboard Shortcuts─",

		// Input Modals
		"input_notes_title": "─Single-Shot AI Instruction for Selected Chat─",
		"input_notes_sub":   "Enter a new AI instruction for this chat (Enter cancels previous; Ctrl+A appends to current):",
		"input_notes_foot":  "Enter: Run Single-Shot (Cancel Prev)   Ctrl+A: Append   Ctrl+X: Stop Running   Esc: Close",

		"input_new_skill_title": "─Create or Import AI Skill─",
		"input_new_skill_sub":   "Enter name for a new skill (e.g. security-audit) or paste/drag a file/folder path to import:",
		"input_new_skill_foot":  "Enter: Create & Open in $EDITOR (or Import Path)   Esc: Cancel",

		"skill_preview_title": "─Skill Instructions Preview: %s─",
	},
	Ru: {
		// General / Top / Bottom
		"status_ready":      "Готово",
		"status_busy":       "В работе",
		"selected_chats":    "Выбрано: %d чатов",
		"active_chat_hdr":   "Чат: %s",
		"parsed_chats":      "Навык выполнен (%s)",
		"key_quit":          "Выход",
		"key_vim_nav":       "Навигация",
		"key_focus":         "Фокус",
		"key_login":         "Вход по QR",
		"key_start_parse":   "Запуск навыка",
		"key_select_chat":   "Открыть чат",
		"key_pin_chat":      "Закрепить",
		"key_hide_chat":     "Скрыть",
		"key_search_chats":  "Поиск чатов",
		"key_compose":       "Написать",
		"key_send":          "Отправить",
		"key_cancel":        "Отмена",
		"key_refresh":       "Обновить",
		"key_run_ai":        "Запуск навыка",
		"key_disable_ai":    "Откл. ИИ",
		"key_activate_skill": "Выбрать навык",
		"key_ai_config":     "ИИ-движок",
		"key_global_config": "Настройки",
		"key_quicklook":     "Просмотр",
		"key_preview":       "Просмотр",
		"key_edit_skill":    "Редакт. навыка",
		"key_mac_edit":      "Редакт. macOS",
		"key_new_skill":     "Новый навык",
		"key_import_skill":  "Импорт навыка",
		"key_notes":         "Разовая ИИ-команда",
		"key_rerun_ai":      "Запуск навыка",
		"key_ai_reply":      "Ответ ИИ",
		"key_auto_reply":    "Автоответ ИИ",
		"key_send_mode":     "Режим отпр.",
		"key_views":         "Вкладки",
		"key_finder":        "В папке",
		"key_help":          "Справка",

		// Sidebar
		"sidebar_chats_hdr":      "─Чаты (Enter/Space:Откр.  \\:Поиск) [%d/%d]─",
		"sidebar_search_active":  " \\ %s█ [%d/%d]",
		"sidebar_search_filter":  " \\ %s [%d/%d] (Esc:Сброс)",
		"sidebar_search_no_match": "  Чаты не найдены",
		"sidebar_hidden_notice":  "  👁  Скрыто чатов: %d (Настройки 'C')",

		// Main Panel Tabs (4 Tabs)
		"tab_details": "─TelePilot",
		"tab_1":       "1:Чат",
		"tab_2":       "2:ИИ и навыки",
		"tab_3":       "3:Результат ИИ",
		"tab_4":       "4:Логи: полные",

		// Tab 1 Live Chat
		"chat_hdr":          " ▸ %s [%s] — Живой чат (%d сообщ.)  │  Синхр: Push + 15с",
		"chat_empty":        "  В этом чате пока нет сообщений. Нажмите 'i' или Enter, чтобы написать.",
		"chat_input_idle":   " Нажмите 'i' или Enter, чтобы написать в %s...",
		"chat_input_active": " Сообщение в %s (Enter: Отправить  │  Esc: Отмена):",
		"chat_media_hint":   "[Нажмите 'v' для просмотра, 'o' в Finder]",
		"chat_media_cached": "[В кэше: %s — 'v' Просмотр • 'o' Finder]",

		// Tab 3 AI Skill Output
		"output_tab_hdr":     " ▸ %s — Результат ИИ-навыка  (r:Запуск навыка  m:Разовая команда  v:Просмотр  o:Finder)",
		"output_card_meta":   "Использованный навык и параметры запуска",
		"output_meta_body":   "Чат:          %s (ID: %d)\nНавык:        %s (%s)\nМодель/Время: %s  |  %s\nСообщения:    %d сообщений чата проанализировано\nФайл отчета:  %s",
		"output_card_result": "Результат работы ИИ-навыка (%s)",
		"output_empty":       "  Для этого чата пока нет результата ИИ-навыка. Нажмите 'r' (во вкл. 2 или 3) для запуска навыка.",
		"sum_extra_notes":    "Доп. заметки: ",

		// Tab 2 Per-Chat AI & Skills
		"skills_tab_hdr":        " ▸ ИИ-навыки для чата — Enter:Навык  a:Автоответ  s:Режим отпр.  A:Ответ ИИ  m:Команда  x:Откл.  r:Запуск навыка",
		"skills_card_chat_ai":   "Инструкции ИИ для выбранного чата: %s",
		"skills_chat_enabled":   "Статус ИИ:\n    [ВКЛ] [АКТИВЕН] (Выделен цветом в списке чатов)\nАктивный навык:\n    %s\nУправление сообщениями:\n    Автоответ: %s (нажмите 'a')\n    Режим отправки: %s (нажмите 's') | 'a' во вкл. 1 для ответа\nРазовая команда:\n    %s (нажмите 'm' для запуска/дополнения/стопа)",
		"skills_chat_disabled":  "Статус ИИ:\n    [ВЫКЛ] [НЕ НАСТРОЕН] (По умолчанию — у чата нет активных ИИ-инструкций)\nАктивный навык:\n    [ВЫКЛ] (%s)\nУправление сообщениями:\n    Автоответ: [ВЫКЛ] (вручную по 'a' во вкл. 1) (нажмите 'a')\n    Режим отправки: В черновик поля ввода (Enter для отправки) (нажмите 's') | 'a' во вкл. 1 для ответа\nРазовая команда:\n    [ВЫКЛ] (нет) (нажмите 'm' для запуска/дополнения/стопа)",
		"skills_autoreply_on":   "[ВКЛ] (ИИ отвечает на входящие)",
		"skills_autoreply_off":  "[ВЫКЛ] (вручную по 'a' во вкл. 1)",
		"skills_sendmode_auto":  "Сразу отправлять в чат",
		"skills_sendmode_draft": "В черновик поля ввода (Enter для отправки)",
		"skills_card_ai_config": "Текущая конфигурация ИИ и активный навык",
		"skills_card_catalog":   "Библиотека навыков (.agents/skills/) [%d/%d]",
		"skills_catalog_keys":   "Клавиши: Enter:Назначить  v:Просмотр  e:Редакт.  O:macOS  o:Finder  n:Создать  I:Импорт  x:Откл.",
		"skills_card_run":       "Последний результат ИИ-навыка для чата",
		"skills_card_actions":   "Действия ИИ и горячие клавиши",
		"skills_lbl_engine":     "Движок и модель:\n    %s | Модель: %s (--model %s)",
		"skills_lbl_quota":      "Квота и статус:\n    %s",
		"skills_lbl_skill":      "Активный навык и папка:\n    %s | %s",
		"skills_lbl_diag":       "Диагностика:\n    %s",
		"skills_empty_lib":      "Навыки не найдены. Нажмите 'n', чтобы создать новый.",
		"skills_badge_active":   "[АКТИВНО]",
		"skills_badge_summary":  "[НАВЫК]",
		"skills_output_meta":    "Последний запуск:\n    %s | Навык: %s | Модель: %s (%d сообщ.)",
		"skills_output_preview": "Краткий результат (Вкл. 3):\n    %s",
		"skills_period_notes":   "Разовая команда:\n    %s",
		"skills_output_none":    "Статус результата:\n    [ВЫКЛ] Для этого чата еще нет результата ИИ-навыка (нажмите 'r' для запуска)\nРазовая команда:\n    %s",
		"skills_notes_none":     "[ВЫКЛ] (нет)",
		"skills_sched_active":   "Фоновые задачи:\n    [АКТИВНО] %d задач (Следующая через %s)",
		"skills_sched_none":     "Фоновые задачи:\n    [ВЫКЛ] (нет)",

		// Tab 4 Trace
		"trace_hdr": " ▸ Трассировка выполнения и диагностики MTProto в реальном времени",

		// Global Config Modal
		"cfg_modal_title":             "─Глобальные настройки (TelePilot)─",
		"cfg_modal_banner":            "Управление параметрами приложения, языком интерфейса и системными путями.",
		"cfg_modal_lang":              "Язык интерфейса:",
		"cfg_modal_favs_title":          "Избранные / Закрепленные чаты (%d):",
		"cfg_modal_favs_empty":          "  (Нет избранных чатов. Нажмите 'p' на любом чате для закрепления.)",
		"cfg_modal_favs_deleted":        "[Удален / Нет в диалогах]",
		"cfg_modal_hidden_title":        "Скрытые чаты (%d):",
		"cfg_modal_hidden_desc":         "  Исключены из синхронизации MTProto и фонового опроса (снижает нагрузку на сеть/ЦПУ).",
		"cfg_modal_hidden_empty":        "  (Нет скрытых чатов. Нажмите 'H' в списке чатов, чтобы скрыть и оптимизировать опрос.)",
		"cfg_modal_btn_enter":           "Войти в блок (Enter)",
		"cfg_modal_block_nav_hint":      "j/k: Выбрать блок │ Enter: Войти │ Ctrl+X: Стоп задач │ Ctrl+R: Сброс навыков │ Esc: Закрыть",
		"cfg_modal_entered_lang_hint":   "j/k или Пробел: Выбор языка │ Enter: Применить │ Esc: Назад к блокам",
		"cfg_modal_entered_favs_hint":   "j/k: Навигация │ d/x: Удалить чат │ Esc: Назад к блокам",
		"cfg_modal_entered_hidden_hint": "j/k: Навигация │ d/x/u: Восстановить чат │ Esc: Назад к блокам",
		"cfg_modal_btn_reset_skills":    "Сбросить навыки ИИ (Ctrl+R)",
		"cfg_modal_reset_skills_success": "Дефолтные навыки ИИ успешно восстановлены",
		"cfg_modal_btn_save":          "Сохранить и закрыть (Ctrl+S)",
		"cfg_modal_btn_cancel":        "Закрыть (Esc)",
		"cfg_modal_hint":              "Подсказка: Клавиша 'C' (заглавная C) открывает это окно настроек в любое время.",
		"cfg_modal_info_model":  "Активная модель ИИ:   %s",
		"cfg_modal_info_skill":  "Навык по умолчанию:   %s",
		"cfg_modal_info_skills": "Каталог навыков:      %s",
		"cfg_modal_info_export": "Каталог данных:       %s",
		"cfg_modal_info_server": "Сервер Telegram:      %s",

		// AI Config Modal
		"ai_modal_title":      "─Подключение и настройка движка ИИ (Antigravity CLI — agy)─",
		"ai_modal_path":       "Путь к AGY CLI:",
		"ai_modal_model":      "Модель AGY:    ",
		"ai_modal_btn_test":   "Тест и сохранить AGY (Enter / Ctrl+T)",
		"ai_modal_btn_cycle":  "Сменить модель (F2)",
		"ai_modal_btn_save":   "Сохранить (Ctrl+S)",
		"ai_modal_btn_cancel": "Закрыть (Esc)",

		// Help Modal
		"help_modal_title": "─Горячие клавиши TelePilot (tgp)─",

		// Input Modals
		"input_notes_title": "─Разовая ИИ-инструкция для выбранного чата─",
		"input_notes_sub":   "Введите инструкцию для ИИ в чате (Enter отменяет предыдущую; Ctrl+A дополняет текущую):",
		"input_notes_foot":  "Enter: Запустить заново (отмена пред.)   Ctrl+A: Дополнить   Ctrl+X: Стоп   Esc: Закрыть",

		"input_new_skill_title": "─Создание или импорт ИИ-навыка─",
		"input_new_skill_sub":   "Введите имя нового навыка (напр. security-audit) или путь к файлу/папке для импорта:",
		"input_new_skill_foot":  "Enter: Создать и открыть в редакторе (или импортировать путь)   Esc: Отмена",

		"skill_preview_title": "─Просмотр инструкций навыка: %s─",
	},
}

/*
T translates a key for the given language.
*/
func T(lang, key string, args ...any) string {
	l := Normalize(lang)
	dict, ok := messages[l]
	if !ok {
		dict = messages[En]
	}
	pattern, found := dict[key]
	if !found {
		pattern, found = messages[En][key]
		if !found {
			return key
		}
	}
	if len(args) > 0 {
		return fmt.Sprintf(pattern, args...)
	}
	return pattern
}
