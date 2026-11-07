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
		"active_chat_hdr":       "Chat: %s",
		"parsed_chats":          "Parsed %d chats",
		"key_quit":              "Quit",
		"key_vim_nav":           "Vim Nav",
		"key_focus":             "Focus",
		"key_login":             "QR Login",
		"key_start_parse":       "Start Parse",
		"key_select_chat":       "Open Chat",
		"key_compose":           "Compose",
		"key_send":              "Send",
		"key_cancel":            "Cancel",
		"key_refresh":           "Refresh",
		"key_run_ai":            "Run AI",
		"key_disable_ai":        "Disable AI",
		"key_activate_skill":    "Activate Skill",
		"key_prev_next_period":  "Prev/Next Period",
		"key_delete_period":     "Delete Archive",
		"key_period":            "Period",
		"key_ai_config":         "AI Engine",
		"key_global_config":     "Settings",
		"key_quicklook":         "QuickLook/Play",
		"key_preview":           "Preview Skill",
		"key_edit_skill":        "Edit Skill",
		"key_new_skill":         "New Skill",
		"key_notes":             "Single-Shot AI",
		"key_rerun_ai":          "Run AI",
		"key_ai_reply":          "AI Reply",
		"key_auto_reply":        "Auto-Reply",
		"key_send_mode":         "Send Mode",
		"key_views":             "Tabs",
		"key_finder":            "Finder",
		"key_help":              "Help",

		// Sidebar
		"sidebar_chats_hdr":     "─Chats (Enter/Space:Open) [%d/%d]─",
		"sidebar_periods_hdr":   "─Period Folders [%d/%d]─",

		// Main Panel Tabs (4 Tabs)
		"tab_details":           "─TelePilot",
		"tab_1":                 "1:Live Chat",
		"tab_2":                 "2:AI & Skills",
		"tab_3":                 "3:Period Archive",
		"tab_4":                 "4:Trace: full",

		// Tab 1 Live Chat
		"chat_hdr":              " ▸ %s [%s] — Live Chat (%d msgs)  │  Sync: Push + 15s",
		"chat_empty":            "  No messages in this chat yet. Press 'i' or Enter to send a message.",
		"chat_input_idle":       " Press 'i' or Enter to message %s...",
		"chat_input_active":     " Compose to %s (Enter: Send  │  Esc: Cancel):",
		"chat_media_hint":       "[Press 'v' to download & preview/play]",
		"chat_media_cached":     "[Cached: %s — press 'v' to open]",

		// Tab 3 Unified Period Archive (Summary + Media Vault + Transcript)
		"period_tab_hdr":        " ▸ %s — Unified Period Archive [%d/%d]  ([ / ]:Switch Archive  p:Period(%s)  r:Run AI  v:Preview/Play  o:Finder  d:Delete)",
		"period_selector_card":  "Period Archives for %s ([ / ] to switch)",
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
		"sum_empty":             "  No period archives for this chat yet. Press 'r' (in Tab 2 or Tab 3) to parse & summarize this chat.",

		// Media Vault Section in Tab 3
		"media_hdr":             " ▸ Media Vault [%d/%d] (%s) — j/k:Select  v:QuickLook/Play  o:Finder",
		"media_empty":           "  No downloaded voice, image, or document files in this period archive.",
		"media_file_lbl":        "File: %s\nAI Capture: %s",
		"media_requires_ai":     "Requires AI transcription",

		// Transcript Section in Tab 3
		"transcript_hdr":        " ▸ Chronological Message & Voice Timeline (%d messages)",
		"transcript_empty":      "  No messages cached in this period archive.",

		// Tab 2 Per-Chat AI & Skills
		"skills_tab_hdr":        " ▸ Per-Chat AI & Skills — Enter:Skill  a:Auto-Reply  s:Send Mode  A:AI Reply  m:Notes  x:Disable  r:Parse",
		"skills_card_chat_ai":   "Per-Chat AI Instructions: %s",
		"skills_chat_enabled":   "AI Status:\n    [ON] [ACTIVE] (Highlighted in Chats sidebar)\nActive Skill:\n    %s | Period window: %s\nMessage Control:\n    Auto-Reply: %s (press 'a')\n    Send Mode: %s (press 's') | Press 'a' in Tab 1 to reply\nSingle-Shot:\n    %s (press 'm' to run/append/stop)",
		"skills_chat_disabled":  "AI Status:\n    [OFF] [NOT CONFIGURED] (Default — no AI instructions active for this chat)\nActive Skill:\n    [OFF] (%s | Period window: %s)\nMessage Control:\n    Auto-Reply: [OFF] (manual via 'a' in Tab 1) (press 'a')\n    Send Mode: Draft to Compose Box (Enter to confirm) (press 's') | Press 'a' in Tab 1 to reply\nSingle-Shot:\n    [OFF] (none) (press 'm' to run/append/stop)",
		"skills_autoreply_on":   "[ON] (AI replies to incoming msgs)",
		"skills_autoreply_off":  "[OFF] (manual via 'a' in Tab 1)",
		"skills_sendmode_auto":  "Auto-Send Immediately",
		"skills_sendmode_draft": "Draft to Compose Box (Enter to confirm)",
		"skills_card_ai_config": "Common AI Engine Configuration (Hotkey 'c')",
		"skills_card_catalog":   "Skills Library (.agents/skills/) [%d/%d] — Enter:Assign to Chat, x:Disable Chat AI, v:Preview, e:Edit, n:New",
		"skills_card_run":       "Selected Chat Latest Archive & Run Settings",
		"skills_card_actions":   "AI Actions & Shortcuts",
		"skills_lbl_engine":     "Engine & Model:\n    %s | Model: %s (--model %s)",
		"skills_lbl_skill":      "Chat Skill & Directory:\n    %s | %s",
		"skills_lbl_diag":       "Last Diagnostic:\n    %s",
		"skills_empty_lib":      "No skills found. Press 'n' to create one.",
		"skills_badge_active":   "[ACTIVE]",
		"skills_badge_summary":  "[SUMMARY]",
		"skills_period_sel":     "Latest Archive:\n    %s | %s -> %s (%s)",
		"skills_period_sum":     "Last Summarized:\n    %s | Model: %s | Skill: %s",
		"skills_period_media":   "Media AI Status:\n    %d media items (%d AI-analyzed, %d pending)",
		"skills_period_notes":   "Single-Shot:\n    %s",
		"skills_period_none":    "Archive Status:\n    [OFF] No period archive generated for this chat yet (press 'r' to run AI)\nSingle-Shot:\n    %s",
		"skills_notes_none":     "[OFF] (none)",

		// Tab 4 Trace
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

		"input_notes_title":     "─Single-Shot AI Instruction for Selected Chat─",
		"input_notes_sub":       "Enter a new AI instruction for this chat (Enter cancels previous; Ctrl+A appends to current):",
		"input_notes_foot":      "Enter: Run Single-Shot (Cancel Prev)   Ctrl+A: Append   Ctrl+X: Stop Running   Esc: Close",

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
		"active_chat_hdr":       "Чат: %s",
		"parsed_chats":          "Обработано чатов: %d",
		"key_quit":              "Выход",
		"key_vim_nav":           "Навигация",
		"key_focus":             "Фокус",
		"key_login":             "Вход по QR",
		"key_start_parse":       "Запуск анализа",
		"key_select_chat":       "Открыть чат",
		"key_compose":           "Написать",
		"key_send":              "Отправить",
		"key_cancel":            "Отмена",
		"key_refresh":           "Обновить",
		"key_run_ai":            "Запуск ИИ",
		"key_disable_ai":        "Откл. ИИ",
		"key_activate_skill":    "Выбрать навык",
		"key_prev_next_period":  "Пред/След период",
		"key_delete_period":     "Удалить архив",
		"key_period":            "Период",
		"key_ai_config":         "ИИ-движок",
		"key_global_config":     "Настройки",
		"key_quicklook":         "Просмотр",
		"key_preview":           "Просмотр",
		"key_edit_skill":        "Редакт. навыка",
		"key_new_skill":         "Новый навык",
		"key_notes":             "Разовая ИИ-команда",
		"key_rerun_ai":          "Запуск ИИ",
		"key_ai_reply":          "Ответ ИИ",
		"key_auto_reply":        "Автоответ ИИ",
		"key_send_mode":         "Режим отпр.",
		"key_views":             "Вкладки",
		"key_finder":            "В папке",
		"key_help":              "Справка",

		// Sidebar
		"sidebar_chats_hdr":     "─Чаты (Enter/Space:Открыть) [%d/%d]─",
		"sidebar_periods_hdr":   "─Папки периодов [%d/%d]─",

		// Main Panel Tabs (4 Tabs)
		"tab_details":           "─TelePilot",
		"tab_1":                 "1:Чат",
		"tab_2":                 "2:ИИ и навыки",
		"tab_3":                 "3:Архив периода",
		"tab_4":                 "4:Логи: полные",

		// Tab 1 Live Chat
		"chat_hdr":              " ▸ %s [%s] — Живой чат (%d сообщ.)  │  Синхр: Push + 15с",
		"chat_empty":            "  В этом чате пока нет сообщений. Нажмите 'i' или Enter, чтобы написать.",
		"chat_input_idle":       " Нажмите 'i' или Enter, чтобы написать в %s...",
		"chat_input_active":     " Сообщение в %s (Enter: Отправить  │  Esc: Отмена):",
		"chat_media_hint":       "[Нажмите 'v' для загрузки и просмотра]",
		"chat_media_cached":     "[В кэше: %s — нажмите 'v' для открытия]",

		// Tab 3 Unified Period Archive (Summary + Media Vault + Transcript)
		"period_tab_hdr":        " ▸ %s — Архив периода [%d/%d]  ([ / ]:Смена архива  p:Период(%s)  r:Запуск ИИ  v:Просмотр  o:Finder  d:Удалить)",
		"period_selector_card":  "Архивы периодов для чата %s ([ / ] для переключения)",
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
		"sum_empty":             "  Для этого чата пока нет архивов периода. Нажмите 'r' (во вкл. 2 или 3) для запуска ИИ-анализа.",

		// Media Vault Section in Tab 3
		"media_hdr":             " ▸ Медиафайлы [%d/%d] (%s) — j/k:Выбор  v:Просмотр  o:Открыть в Finder",
		"media_empty":           "  В этой папке периода нет загруженных голосовых, изображений или документов.",
		"media_file_lbl":        "Файл: %s\nИИ-анализ: %s",
		"media_requires_ai":     "Требуется ИИ-транскрипция",

		// Transcript Section in Tab 3
		"transcript_hdr":        " ▸ Хронологическая лента сообщений и голоса (%d сообщений)",
		"transcript_empty":      "  В этом архиве периода нет сохраненных сообщений.",

		// Tab 2 Per-Chat AI & Skills
		"skills_tab_hdr":        " ▸ ИИ-навыки для чата — Enter:Навык  a:Автоответ  s:Режим отпр.  A:Ответ ИИ  m:Заметки  x:Откл.  r:Анализ",
		"skills_card_chat_ai":   "Инструкции ИИ для выбранного чата: %s",
		"skills_chat_enabled":   "Статус ИИ:\n    [ВКЛ] [АКТИВЕН] (Выделен цветом в списке чатов)\nАктивный навык:\n    %s | Период: %s\nУправление сообщениями:\n    Автоответ: %s (нажмите 'a')\n    Режим отправки: %s (нажмите 's') | 'a' во вкл. 1 для ответа\nРазовая команда:\n    %s (нажмите 'm' для запуска/дополнения/стопа)",
		"skills_chat_disabled":  "Статус ИИ:\n    [ВЫКЛ] [НЕ НАСТРОЕН] (По умолчанию — у чата нет активных ИИ-инструкций)\nАктивный навык:\n    [ВЫКЛ] (%s | Период: %s)\nУправление сообщениями:\n    Автоответ: [ВЫКЛ] (вручную по 'a' во вкл. 1) (нажмите 'a')\n    Режим отправки: В черновик поля ввода (Enter для отправки) (нажмите 's') | 'a' во вкл. 1 для ответа\nРазовая команда:\n    [ВЫКЛ] (нет) (нажмите 'm' для запуска/дополнения/стопа)",
		"skills_autoreply_on":   "[ВКЛ] (ИИ отвечает на входящие)",
		"skills_autoreply_off":  "[ВЫКЛ] (вручную по 'a' во вкл. 1)",
		"skills_sendmode_auto":  "Сразу отправлять в чат",
		"skills_sendmode_draft": "В черновик поля ввода (Enter для отправки)",
		"skills_card_ai_config": "Текущая конфигурация ИИ и активный навык",
		"skills_card_catalog":   "Библиотека навыков (.agents/skills/) [%d/%d] — Enter:Назначить чату, x:Откл. ИИ чата, v:Просмотр, e:Редакт., n:Создать",
		"skills_card_run":       "Выбранный период и параметры запуска",
		"skills_card_actions":   "Действия ИИ и горячие клавиши",
		"skills_lbl_engine":     "Движок и модель:\n    %s | Модель: %s (--model %s)",
		"skills_lbl_skill":      "Активный навык и папка:\n    %s | %s",
		"skills_lbl_diag":       "Диагностика:\n    %s",
		"skills_empty_lib":      "Навыки не найдены. Нажмите 'n', чтобы создать новый.",
		"skills_badge_active":   "[АКТИВНО]",
		"skills_badge_summary":  "[СВОДКА]",
		"skills_period_sel":     "Последний архив:\n    %s | %s -> %s (%s)",
		"skills_period_sum":     "Дата анализа:\n    %s | Модель: %s | Навык: %s",
		"skills_period_media":   "Статус медиа:\n    %d файлов (%d обработано ИИ, %d в очереди)",
		"skills_period_notes":   "Разовая команда:\n    %s",
		"skills_period_none":    "Статус архива:\n    [ВЫКЛ] Для этого чата еще не создан архив периода (нажмите 'r' для запуска ИИ)\nРазовая команда:\n    %s",
		"skills_notes_none":     "[ВЫКЛ] (нет)",

		// Tab 4 Trace
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

		"input_notes_title":     "─Разовая ИИ-инструкция для выбранного чата─",
		"input_notes_sub":       "Введите инструкцию для ИИ в чате (Enter отменяет предыдущую; Ctrl+A дополняет текущую):",
		"input_notes_foot":      "Enter: Запустить заново (отмена пред.)   Ctrl+A: Дополнить   Ctrl+X: Стоп   Esc: Закрыть",

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

