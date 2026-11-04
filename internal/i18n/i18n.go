package i18n

import (
	"fmt"
	"strings"
)

const (
	En = "en"
	Ru = "ru"
)

// Normalize returns either "ru" or "en" (default).
func Normalize(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	if l == "ru" || strings.HasPrefix(l, "ru_") || strings.HasPrefix(l, "ru-") {
		return Ru
	}
	return En
}

// IsRussian checks if the language code is Russian.
func IsRussian(lang string) bool {
	return Normalize(lang) == Ru
}

var messages = map[string]map[string]string{
	En: {
		// General / Top / Bottom
		"status_ready":          "Ready",
		"status_busy":           "Busy",
		"period_label":          "Period",
		"selected_chats":        "Selected: %d chats",
		"parsed_chats":          "Parsed %d chats",
		"key_quit":              "Quit",
		"key_vim_nav":           "Vim Nav",
		"key_login":             "QR Login & Conn Test",
		"key_start_parse":       "Start Parse",
		"key_select_chat":       "Select Chat",
		"key_delete_period":     "Delete Period",
		"key_period":            "Period",
		"key_ai_config":         "AI Config",
		"key_global_config":     "Global Config",
		"key_quicklook":         "QuickLook",
		"key_preview":           "Preview",
		"key_edit_skill":        "Edit Skill",
		"key_new_skill":         "New Skill",
		"key_notes":             "Notes",
		"key_rerun_ai":          "Re-Run AI",
		"key_views":             "Views",
		"key_finder":            "Finder",
		"key_help":              "Help",

		// Sidebar
		"sidebar_chats_hdr":     "─Chats (Space:Select) [%d/%d]─",
		"sidebar_periods_hdr":   "─Period Folders [%d/%d]─",

		// Main Panel Tabs
		"tab_details":           "─Details",
		"tab_1":                 "1:Summary",
		"tab_2":                 "2:Media Vault",
		"tab_3":                 "3:Transcript & Voice",
		"tab_4":                 "4:Skills & Re-Run",
		"tab_5":                 "5:Trace: full",

		// Tab 1 Summary
		"sum_hdr":               " ▸ %s — Period Summary",
		"sum_status_label":      "Status: %s",
		"sum_period_range":      "Period: %s -> %s (%s)",
		"sum_accounting":        "Accounting: %d messages  |  [VOICE] %d transcribed  |  [IMAGE] %d images  |  [DOC] %d documents",
		"sum_archive":           "Archive: %s/{images,documents,voice}",
		"sum_card_skill":        "Skill & Instructions Used",
		"sum_card_executive":    "Executive Summary",
		"sum_card_decisions":    "Key Decisions & Highlights",
		"sum_card_media":        "Voice Messages, Images & Documents Accounting",
		"sum_card_actions":      "Action Items & Follow-ups",
		"sum_legacy_prompt":     "legacy prompt",
		"sum_extra_notes":       "Extra Notes: ",
		"sum_empty":             "  No period folders found yet. Select chats with Space and press 's' to parse.",

		// Tab 2 Media Vault
		"media_hdr":             " ▸ Media Vault [%d/%d] (%s) — j/k:Scroll  v:QuickLook/Play  o:Finder",
		"media_empty":           "  No downloaded voice, image, or document files in this period folder.",
		"media_file_lbl":        "File: %s\nAI Capture: %s",
		"media_requires_ai":     "Requires AI transcription",

		// Tab 3 Transcript
		"transcript_hdr":        " ▸ Chronological Message & Voice Timeline (%d messages)",
		"transcript_empty":      "  No messages cached in this period archive.",

		// Tab 4 Skills & Re-Run
		"skills_tab_hdr":        " ▸ AI Skills & Period Re-Run — j/k:Navigate  v:Preview  Enter:Apply  e:Edit  n:New  a/m:Notes  r:Re-Run",
		"skills_card_ai_config": "Current AI Configuration & Active Skill",
		"skills_card_catalog":   "Skills Library (.agents/skills/) [%d/%d] — v:Preview, Enter:Activate, e:Edit, n:New",
		"skills_card_run":       "Selected Period & Run Settings",
		"skills_card_actions":   "AI Actions & Shortcuts",
		"skills_lbl_engine":     "Engine: %s  |  Model: %s  (--model %s)",
		"skills_lbl_skill":      "Active Summary Skill: %s  |  Skills Dir: %s",
		"skills_lbl_diag":       "Last Diagnostic: %s",
		"skills_empty_lib":      "No skills found. Press 'n' to create one.",
		"skills_badge_active":   "[ACTIVE]",
		"skills_badge_summary":  "[SUMMARY]",
		"skills_period_sel":     "Selected Period : %s  |  %s -> %s (%s)",
		"skills_period_sum":     "Last Summarized : %s  |  Model: %s  |  Skill: %s",
		"skills_period_media":   "Media AI Status : %d media items (%d AI-analyzed, %d pending)",
		"skills_period_notes":   "Extra Run Notes : %s",
		"skills_period_none":    "No period folder selected yet.\n\nExtra Run Notes:\n%s",
		"skills_notes_none":     "(none — using raw skill instructions)",

		// Tab 5 Trace
		"trace_hdr":             " ▸ Real-Time Execution & MTProto Diagnostic Trace",

		// Global Config Modal
		"cfg_modal_title":       "─Global Configuration (TelePilot)─",
		"cfg_modal_banner":      "Manage application preferences, preferred language, and system paths.",
		"cfg_modal_lang":        "Interface Language:",
		"cfg_modal_btn_toggle":  "Toggle Language (F2 / Space / h / l)",
		"cfg_modal_btn_save":    "Save & Close (Enter / Ctrl+S)",
		"cfg_modal_btn_cancel":  "Cancel / Close (Esc)",
		"cfg_modal_hint":        "Hotkey tip: Capital 'C' opens this Global Config window anytime from main view.",
		"cfg_modal_info_model":  "Active AI Model:      %s",
		"cfg_modal_info_period": "Default Period:       %s",
		"cfg_modal_info_skill":  "Default Skill:        %s",
		"cfg_modal_info_skills": "Skills Directory:     %s",
		"cfg_modal_info_export": "Exports Directory:    %s",
		"cfg_modal_info_server": "Telegram Server:      %s",

		// AI Config Modal
		"ai_modal_title":        "─Connect & Configure AI Engine (Antigravity CLI — agy)─",
		"ai_modal_path":         "AGY CLI Path:  ",
		"ai_modal_model":        "AGY Model:     ",
		"ai_modal_btn_test":     "Save & Test AGY (Enter / Ctrl+T)",
		"ai_modal_btn_cycle":    "Cycle Model Preset (F2)",
		"ai_modal_btn_save":     "Save & Close (Ctrl+S)",
		"ai_modal_btn_cancel":   "Cancel / Close (Esc)",

		// Help Modal
		"help_modal_title":      "─TelePilot (tgp) Vim Keyboard Shortcuts─",

		// Confirm Delete Modal
		"del_modal_title":       "─[!] Confirm Period Folder Deletion─",
		"del_modal_warning":     "WARNING: All downloaded files, subfolders (images/, documents/, voice/), transcripts, summary.md, and metadata.json in the selected period folder will be permanently deleted from disk.",
		"del_modal_chat":        "Chat:     %s",
		"del_modal_period":      "Period:   %s",
		"del_modal_contents":    "Contents: %s",
		"del_modal_folder":      "Folder:   %s",
		"del_modal_confirm":     "Confirm Delete (y / d / Enter)",
		"del_modal_cancel":      "Cancel (n / q / Esc)",
		"del_modal_hint":        "Vim keys: h/l or Tab to switch button  •  y/d to Delete  •  n/q/Esc to Cancel",

		// Input Modals
		"input_period_title":    "─Configure Period Window─",
		"input_period_sub":      "Enter relative period (e.g. 7d, 2w, 1m, 3m) or date range (YYYY-MM-DD..YYYY-MM-DD):",
		"input_period_foot":     "Enter: Apply Period   Esc: Cancel",

		"input_notes_title":     "─Extra Run Notes for AI Summarization─",
		"input_notes_sub":       "Enter optional extra instructions to append to the active skill for this run:",
		"input_notes_foot":      "Enter: Save Notes   Ctrl+R: Save & Re-Run   Ctrl+U: Clear   Esc: Cancel",

		"input_new_skill_title": "─Create New AI Skill─",
		"input_new_skill_sub":   "Enter name for the new skill (e.g. security-audit, incident-digest):",
		"input_new_skill_foot":  "Enter: Create & Open in $EDITOR   Esc: Cancel",

		"skill_preview_title":   "─Skill Instructions Preview: %s─",
	},
	Ru: {
		// General / Top / Bottom
		"status_ready":          "Готово",
		"status_busy":           "В работе",
		"period_label":          "Период",
		"selected_chats":        "Выбрано: %d чатов",
		"parsed_chats":          "Обработано чатов: %d",
		"key_quit":              "Выход",
		"key_vim_nav":           "Навигация",
		"key_login":             "Вход по QR и тест",
		"key_start_parse":       "Запуск анализа",
		"key_select_chat":       "Выбор чата",
		"key_delete_period":     "Удалить период",
		"key_period":            "Период",
		"key_ai_config":         "ИИ-движок",
		"key_global_config":     "Настройки",
		"key_quicklook":         "Просмотр",
		"key_preview":           "Просмотр",
		"key_edit_skill":        "Редакт. навыка",
		"key_new_skill":         "Новый навык",
		"key_notes":             "Заметки",
		"key_rerun_ai":          "Перезапуск ИИ",
		"key_views":             "Вкладки",
		"key_finder":            "В папке",
		"key_help":              "Справка",

		// Sidebar
		"sidebar_chats_hdr":     "─Чаты (Space:Выбор) [%d/%d]─",
		"sidebar_periods_hdr":   "─Папки периодов [%d/%d]─",

		// Main Panel Tabs
		"tab_details":           "─Детали",
		"tab_1":                 "1:Сводка",
		"tab_2":                 "2:Медиафайлы",
		"tab_3":                 "3:Стенограмма и голос",
		"tab_4":                 "4:Навыки и запуск",
		"tab_5":                 "5:Логи: полные",

		// Tab 1 Summary
		"sum_hdr":               " ▸ %s — Сводка за период",
		"sum_status_label":      "Статус: %s",
		"sum_period_range":      "Период: %s -> %s (%s)",
		"sum_accounting":        "Учет: %d сообщений  |  [ГОЛОС] %d транскрибировано  |  [ФОТО] %d изображений  |  [ДОК] %d документов",
		"sum_archive":           "Архив: %s/{images,documents,voice}",
		"sum_card_skill":        "Использованный навык и инструкции",
		"sum_card_executive":    "Краткое содержание",
		"sum_card_decisions":    "Ключевые решения и главное",
		"sum_card_media":        "Учет голосовых сообщений, изображений и документов",
		"sum_card_actions":      "Задачи и следующие шаги",
		"sum_legacy_prompt":     "базовый промпт",
		"sum_extra_notes":       "Доп. заметки: ",
		"sum_empty":             "  Папки периодов пока не найдены. Выберите чаты клавишей Space и нажмите 's' для запуска анализа.",

		// Tab 2 Media Vault
		"media_hdr":             " ▸ Медиафайлы [%d/%d] (%s) — j/k:Прокрутка  v:Просмотр  o:Открыть в Finder",
		"media_empty":           "  В этой папке периода нет загруженных голосовых, изображений или документов.",
		"media_file_lbl":        "Файл: %s\nИИ-анализ: %s",
		"media_requires_ai":     "Требуется ИИ-транскрипция",

		// Tab 3 Transcript
		"transcript_hdr":        " ▸ Хронологическая лента сообщений и голоса (%d сообщений)",
		"transcript_empty":      "  В этом архиве периода нет сохраненных сообщений.",

		// Tab 4 Skills & Re-Run
		"skills_tab_hdr":        " ▸ ИИ-навыки и перезапуск — j/k:Навигация  v:Просмотр  Enter:Применить  e:Редакт.  n:Создать  a/m:Заметки  r:Перезапуск",
		"skills_card_ai_config": "Текущая конфигурация ИИ и активный навык",
		"skills_card_catalog":   "Библиотека навыков (.agents/skills/) [%d/%d] — v:Просмотр, Enter:Активировать, e:Редакт., n:Создать",
		"skills_card_run":       "Выбранный период и параметры запуска",
		"skills_card_actions":   "Действия ИИ и горячие клавиши",
		"skills_lbl_engine":     "Движок: %s  |  Модель: %s  (--model %s)",
		"skills_lbl_skill":      "Активный навык: %s  |  Папка навыков: %s",
		"skills_lbl_diag":       "Диагностика: %s",
		"skills_empty_lib":      "Навыки не найдены. Нажмите 'n', чтобы создать новый.",
		"skills_badge_active":   "[АКТИВНО]",
		"skills_badge_summary":  "[СВОДКА]",
		"skills_period_sel":     "Выбранный период : %s  |  %s -> %s (%s)",
		"skills_period_sum":     "Дата анализа     : %s  |  Модель: %s  |  Навык: %s",
		"skills_period_media":   "Статус медиа     : %d файлов (%d обработано ИИ, %d в очереди)",
		"skills_period_notes":   "Доп. заметки     : %s",
		"skills_period_none":    "Папка периода пока не выбрана.\n\nДополнительные заметки:\n%s",
		"skills_notes_none":     "(нет — используются стандартные инструкции навыка)",

		// Tab 5 Trace
		"trace_hdr":             " ▸ Трассировка выполнения и диагностики MTProto в реальном времени",

		// Global Config Modal
		"cfg_modal_title":       "─Глобальные настройки (TelePilot)─",
		"cfg_modal_banner":      "Управление параметрами приложения, языком интерфейса и системными путями.",
		"cfg_modal_lang":        "Язык интерфейса:",
		"cfg_modal_btn_toggle":  "Переключить язык (F2 / Space / h / l)",
		"cfg_modal_btn_save":    "Сохранить и закрыть (Enter / Ctrl+S)",
		"cfg_modal_btn_cancel":  "Закрыть (Esc)",
		"cfg_modal_hint":        "Подсказка: Клавиша 'C' (заглавная C) открывает это окно настроек в любое время.",
		"cfg_modal_info_model":  "Активная модель ИИ:   %s",
		"cfg_modal_info_period": "Период по умолчанию:  %s",
		"cfg_modal_info_skill":  "Навык по умолчанию:   %s",
		"cfg_modal_info_skills": "Каталог навыков:      %s",
		"cfg_modal_info_export": "Каталог экспорта:     %s",
		"cfg_modal_info_server": "Сервер Telegram:      %s",

		// AI Config Modal
		"ai_modal_title":        "─Подключение и настройка движка ИИ (Antigravity CLI — agy)─",
		"ai_modal_path":         "Путь к AGY CLI:",
		"ai_modal_model":        "Модель AGY:    ",
		"ai_modal_btn_test":     "Тест и сохранить AGY (Enter / Ctrl+T)",
		"ai_modal_btn_cycle":    "Сменить модель (F2)",
		"ai_modal_btn_save":     "Сохранить (Ctrl+S)",
		"ai_modal_btn_cancel":   "Закрыть (Esc)",

		// Help Modal
		"help_modal_title":      "─Горячие клавиши TelePilot (tgp)─",

		// Confirm Delete Modal
		"del_modal_title":       "─[!] Подтверждение удаления папки периода─",
		"del_modal_warning":     "ВНИМАНИЕ: Все загруженные файлы, вложенные папки (images/, documents/, voice/), стенограммы, summary.md и metadata.json в выбранной папке периода будут безвозвратно удалены с диска.",
		"del_modal_chat":        "Чат:       %s",
		"del_modal_period":      "Период:    %s",
		"del_modal_contents":    "Содержимое: %s",
		"del_modal_folder":      "Папка:     %s",
		"del_modal_confirm":     "Подтвердить удаление (y / d / Enter)",
		"del_modal_cancel":      "Отмена (n / q / Esc)",
		"del_modal_hint":        "Клавиши: h/l или Tab для переключения  •  y/d Удалить  •  n/q/Esc Отмена",

		// Input Modals
		"input_period_title":    "─Настройка окна периода─",
		"input_period_sub":      "Введите относительный период (напр. 7d, 2w, 1m, 3m) или диапазон дат (ГГГГ-ММ-ДД..ГГГГ-ММ-ДД):",
		"input_period_foot":     "Enter: Применить   Esc: Отмена",

		"input_notes_title":     "─Дополнительные заметки для ИИ-сводки─",
		"input_notes_sub":       "Введите дополнительные инструкции, которые дополнят активный навык при запуске:",
		"input_notes_foot":      "Enter: Сохранить   Ctrl+R: Сохранить и запустить   Ctrl+U: Очистить   Esc: Отмена",

		"input_new_skill_title": "─Создание нового ИИ-навыка─",
		"input_new_skill_sub":   "Введите имя нового навыка (напр. security-audit, incident-digest):",
		"input_new_skill_foot":  "Enter: Создать и открыть в редакторе   Esc: Отмена",

		"skill_preview_title":   "─Просмотр инструкций навыка: %s─",
	},
}

// T translates a key for the given language.
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

