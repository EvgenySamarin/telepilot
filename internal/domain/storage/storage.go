/*
File: internal/domain/storage/storage.go
Description: JSON and Markdown persistence helpers for per-chat AI configs, favourite
chats, hidden chats, scheduled tasks, and skill output artifacts.
*/

package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"telepilot/internal/domain/config"
)

/*
ChatAIConfigFilePath returns the JSON file path for per-chat AI configuration.
*/
func ChatAIConfigFilePath(dataDir string) string {
	return filepath.Join(dataDir, "chat_ai_config.json")
}

/*
FavoriteChatsFilePath returns the JSON file path for pinned favourite chats.
*/
func FavoriteChatsFilePath(dataDir string) string {
	return filepath.Join(dataDir, "favorite_chats.json")
}

/*
HiddenChatsFilePath returns the JSON file path for hidden chats.
*/
func HiddenChatsFilePath(dataDir string) string {
	return filepath.Join(dataDir, "hidden_chats.json")
}

/*
ScheduledTasksFilePath returns the JSON file path for persisted scheduled tasks.
*/
func ScheduledTasksFilePath(dataDir string) string {
	return filepath.Join(dataDir, "scheduled_tasks.json")
}

/*
LoadScheduledTasks reads persisted scheduled tasks from disk, returning an empty slice if none exist.
*/
func LoadScheduledTasks(dataDir string) []ScheduledTask {
	path := ScheduledTasksFilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	var list []ScheduledTask
	if err := json.Unmarshal(data, &list); err == nil {
		return list
	}
	return nil
}

/*
SaveScheduledTasks persists the slice of scheduled tasks as indented JSON.
*/
func SaveScheduledTasks(dataDir string, tasks []ScheduledTask) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	if tasks == nil {
		tasks = []ScheduledTask{}
	}
	bytes, err := json.MarshalIndent(tasks, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ScheduledTasksFilePath(dataDir), bytes, 0o644)
}

/*
LoadFavoriteChats reads persisted favourite chat records from disk.
*/
func LoadFavoriteChats(dataDir string) []FavoriteChatRecord {
	path := FavoriteChatsFilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	var list []FavoriteChatRecord
	if err := json.Unmarshal(data, &list); err == nil {
		return list
	}
	return nil
}

/*
SaveFavoriteChats persists the list of favourite chat records as indented JSON.
*/
func SaveFavoriteChats(dataDir string, favs []FavoriteChatRecord) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	if favs == nil {
		favs = []FavoriteChatRecord{}
	}
	bytes, err := json.MarshalIndent(favs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(FavoriteChatsFilePath(dataDir), bytes, 0o644)
}

/*
LoadHiddenChats reads persisted hidden chat records from disk.
*/
func LoadHiddenChats(dataDir string) []HiddenChatRecord {
	path := HiddenChatsFilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil
	}
	var list []HiddenChatRecord
	if err := json.Unmarshal(data, &list); err == nil {
		return list
	}
	return nil
}

/*
SaveHiddenChats persists the list of hidden chat records as indented JSON.
*/
func SaveHiddenChats(dataDir string, hidden []HiddenChatRecord) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	if hidden == nil {
		hidden = []HiddenChatRecord{}
	}
	bytes, err := json.MarshalIndent(hidden, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(HiddenChatsFilePath(dataDir), bytes, 0o644)
}

/*
LoadChatAIConfigs reads per-chat AI configurations from disk keyed by chat ID.
*/
func LoadChatAIConfigs(dataDir string) map[int64]ChatAIConfig {
	out := make(map[int64]ChatAIConfig)
	path := ChatAIConfigFilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return out
	}
	var list []ChatAIConfig
	if err := json.Unmarshal(data, &list); err == nil {
		for _, item := range list {
			if item.ChatID != 0 {
				out[item.ChatID] = item
			}
		}
		return out
	}
	var byID map[int64]ChatAIConfig
	if err := json.Unmarshal(data, &byID); err == nil {
		for k, v := range byID {
			out[k] = v
		}
	}
	return out
}

/*
SaveChatAIConfigs persists the per-chat AI configuration map as indented JSON.
*/
func SaveChatAIConfigs(dataDir string, cfgs map[int64]ChatAIConfig) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	var list []ChatAIConfig
	for _, v := range cfgs {
		if v.ChatID != 0 && v.Enabled {
			list = append(list, v)
		}
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ChatID < list[j].ChatID
	})
	bytes, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ChatAIConfigFilePath(dataDir), bytes, 0o644)
}

/*
ChatSkillOutputsFilePath returns the JSON file path for per-chat skill output records.
*/
func ChatSkillOutputsFilePath(dataDir string) string {
	return filepath.Join(dataDir, "chat_skill_outputs.json")
}

/*
LoadChatSkillOutputs reads per-chat skill output records from disk keyed by chat ID.
*/
func LoadChatSkillOutputs(dataDir string) map[int64]SkillOutputRecord {
	out := make(map[int64]SkillOutputRecord)
	path := ChatSkillOutputsFilePath(dataDir)
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return out
	}
	var list []SkillOutputRecord
	if err := json.Unmarshal(data, &list); err == nil {
		for _, item := range list {
			if item.ChatID != 0 {
				out[item.ChatID] = item
			}
		}
		return out
	}
	return out
}

/*
SaveChatSkillOutputs persists the per-chat skill output records map as indented JSON.
*/
func SaveChatSkillOutputs(dataDir string, outputs map[int64]SkillOutputRecord) error {
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return err
	}
	outputsDir := filepath.Join(dataDir, "outputs")
	_ = os.MkdirAll(outputsDir, 0o755)

	var list []SkillOutputRecord
	for chatID, rec := range outputs {
		if chatID == 0 || strings.TrimSpace(rec.Output) == "" {
			continue
		}
		slug := config.Slugify(rec.ChatTitle)
		mdPath := filepath.Join(outputsDir, fmt.Sprintf("%s_output.md", slug))
		mdContent := fmt.Sprintf("# AI Skill Output: %s\n\n- **Skill**: %s (%s)\n- **Model**: %s\n- **Messages Analyzed**: %d\n- **Generated At**: %s\n\n---\n\n%s\n",
			rec.ChatTitle, rec.Skill, rec.SkillHash, rec.ModelUsed, rec.MessageCount, rec.CreatedAt, rec.Output)
		if err := os.WriteFile(mdPath, []byte(mdContent), 0o644); err == nil {
			rec.OutputPath = mdPath
			outputs[chatID] = rec
		}
		list = append(list, rec)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ChatID < list[j].ChatID
	})
	bytes, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ChatSkillOutputsFilePath(dataDir), bytes, 0o644)
}

/*
DemoSkillOutput constructs a sample SkillOutputRecord for a chat in the requested language.
*/
func DemoSkillOutput(chat ChatInfo, lang string) SkillOutputRecord {
	isRu := strings.ToLower(strings.TrimSpace(lang)) == "ru"
	if isRu {
		return SkillOutputRecord{
			ChatID:       chat.ChatID,
			ChatTitle:    chat.Title,
			Skill:        "period-summary",
			SkillHash:    "ffb7ffede482",
			SkillDesc:    "Анализ и выполнение ИИ-навыка по сообщениям чата",
			ModelUsed:    "agy-cli/auto",
			MessageCount: len(DemoLiveMessages(chat)),
			CreatedAt:    "2026-11-07 23:45:00 UTC",
			Output: strings.Join([]string{
				"• График перехода на API v2 согласован: заморозка эндпоинтов v1 в пятницу 18:00 UTC.",
				"• Сессии авторизации переведены на кластер Redis 7.2 с шардированием; задержка p99 снижена на 38% (до 340 мс).",
				"• Следующий шаг: включить режим двойной записи (Phase 1 dual-write) на staging завтра утром.",
			}, "\n"),
		}
	}
	return SkillOutputRecord{
		ChatID:       chat.ChatID,
		ChatTitle:    chat.Title,
		Skill:        "period-summary",
		SkillHash:    "ffb7ffede482",
		SkillDesc:    "AI skill execution output for the selected chat",
		ModelUsed:    "agy-cli/auto",
		MessageCount: len(DemoLiveMessages(chat)),
		CreatedAt:    "2026-11-07 23:45:00 UTC",
		Output: strings.Join([]string{
			"• API v2 cutover schedule finalized: v1 endpoints freeze this Friday at 18:00 UTC.",
			"• Auth session store migrated to sharded Redis 7.2 cluster; p99 latency dropped by 38% to 340ms.",
			"• Next step: enable Phase 1 dual-write in staging first thing tomorrow morning.",
		}, "\n"),
	}
}

/*
DiscoverChatPeriodFolders returns all saved period summary records belonging to the given chat.
*/
func DiscoverChatPeriodFolders(exportsDir string, chat ChatInfo) []PeriodSummaryRecord {
	all := DiscoverAllPeriodFolders(exportsDir)
	chatSlug := config.Slugify(chat.Title)
	var filtered []PeriodSummaryRecord
	for _, rec := range all {
		if (chat.ChatID != 0 && rec.ChatID == chat.ChatID) ||
			(chat.Title != "" && config.Slugify(rec.ChatTitle) == chatSlug) {
			filtered = append(filtered, rec)
		}
	}
	return filtered
}

/*
DemoLiveMessages returns built-in demo messages for a chat when running in offline demo mode.
*/
func DemoLiveMessages(chat ChatInfo) []LiveMessage {
	switch chat.ChatID {
	case -1001849201: // Core Backend & Infra
		return []LiveMessage{
			{MessageID: 395, ChatID: chat.ChatID, Sender: "Elena (@elena_arch)", Timestamp: "10:55", Text: "Morning team, we need to finalize the API v2 cutover schedule this week."},
			{MessageID: 398, ChatID: chat.ChatID, Sender: "Alexey (@alex_lead)", Timestamp: "11:20", Text: "Voice note on v1 endpoint freeze & Redis cluster.", MediaType: "voice", MediaFileName: "voice_20261014_msg398.ogg", DurationSec: 44, MediaSize: 84200},
			{MessageID: 402, ChatID: chat.ChatID, Sender: "Elena (@elena_arch)", Timestamp: "14:05", Text: "Updated whiteboard diagram for the Redis session store", MediaType: "image", MediaFileName: "img_20261015_msg402_auth_schema.png", MediaSize: 245120},
			{MessageID: 418, ChatID: chat.ChatID, Sender: "Elena (@elena_arch)", Timestamp: "16:40", Text: "Final PDF spec for API v2 migration", MediaType: "document", MediaFileName: "msg418_API_v2_Migration_Spec.pdf", MediaSize: 512400},
			{MessageID: 455, ChatID: chat.ChatID, Sender: "Dmitry (@dmitry_sre)", Timestamp: "17:15", Text: "Benchmark numbers look solid—p99 latency dropped by 38% to 340ms.", MediaType: "document", MediaFileName: "msg455_Q4_Latency_Benchmark.csv", MediaSize: 18400},
			{MessageID: 490, ChatID: chat.ChatID, Sender: "You", IsOutgoing: true, Timestamp: "18:02", Text: "Awesome work! Let's enable Phase 1 dual-write in staging first thing tomorrow."},
		}
	case -1001928374: // Product Design & Specs
		return []LiveMessage{
			{MessageID: 205, ChatID: chat.ChatID, Sender: "Maria (@maria_ux)", Timestamp: "09:30", Text: "Shared the Tokyo-Night color contrast tokens for the terminal client."},
			{MessageID: 210, ChatID: chat.ChatID, Sender: "Maria (@maria_ux)", Timestamp: "10:12", Text: "Design critique feedback on single-chat selection & AI badges.", MediaType: "voice", MediaFileName: "voice_20261021_msg210.ogg", DurationSec: 58, MediaSize: 96000},
			{MessageID: 214, ChatID: chat.ChatID, Sender: "You", IsOutgoing: true, Timestamp: "11:05", Text: "Agreed, highlighting chats with active AI instructions right in the sidebar makes it super clear."},
		}
	default:
		return []LiveMessage{
			{MessageID: 101, ChatID: chat.ChatID, Sender: chat.Title, Timestamp: "11:00", Text: "Welcome to live chat view for " + chat.Title + "."},
			{MessageID: 104, ChatID: chat.ChatID, Sender: chat.Title, Timestamp: "15:30", Text: "Architecture overview diagram attached.", MediaType: "image", MediaFileName: "img_sample_msg104.png", MediaSize: 184000},
			{MessageID: 108, ChatID: chat.ChatID, Sender: "You", IsOutgoing: true, Timestamp: "17:15", Text: "Looks great — press 'i' or Enter to compose a message anytime."},
		}
	}
}

var DemoChats = []ChatInfo{
	{ChatID: -1001849201, Title: "Core Backend & Infra", ChatType: "group", UnreadCount: 14},
	{ChatID: -1001928374, Title: "Product Design & Specs", ChatType: "group", UnreadCount: 6},
	{ChatID: -1001554321, Title: "Founders Weekly Sync", ChatType: "group", UnreadCount: 3},
	{ChatID: -1001778899, Title: "AI Research & Papers", ChatType: "channel", UnreadCount: 28},
	{ChatID: 48291042, Title: "Alexey (Tech Lead)", ChatType: "dm", UnreadCount: 2},
}

/*
SavePeriodRun writes a period summary record's metadata JSON, summary markdown, and media index to disk.
*/
func SavePeriodRun(rec PeriodSummaryRecord) error {
	if err := os.MkdirAll(rec.FolderPath, 0o755); err != nil {
		return err
	}
	_ = os.MkdirAll(filepath.Join(rec.FolderPath, "images"), 0o755)
	_ = os.MkdirAll(filepath.Join(rec.FolderPath, "documents"), 0o755)
	_ = os.MkdirAll(filepath.Join(rec.FolderPath, "voice"), 0o755)

	metaBytes, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(rec.FolderPath, "metadata.json"), metaBytes, 0o644); err != nil {
		return err
	}

	summaryMD := BuildSummaryMarkdown(rec)
	return os.WriteFile(filepath.Join(rec.FolderPath, "summary.md"), []byte(summaryMD), 0o644)
}

/*
LoadPeriodRun reads a PeriodSummaryRecord from a period export folder's metadata.json.
*/
func LoadPeriodRun(folderPath string) (*PeriodSummaryRecord, error) {
	metaFile := filepath.Join(folderPath, "metadata.json")
	data, err := os.ReadFile(metaFile)
	if err != nil {
		return nil, err
	}
	var rec PeriodSummaryRecord
	if err := json.Unmarshal(data, &rec); err != nil {
		return nil, err
	}
	rec.FolderPath = folderPath
	return &rec, nil
}

/*
DeletePeriodRun safely removes a period export folder after verifying it resides inside exportsDir.
*/
func DeletePeriodRun(folderPath string, exportsDir string) error {
	cleanFolder := filepath.Clean(folderPath)
	cleanExports := filepath.Clean(exportsDir)
	if cleanFolder == "" || cleanFolder == "." || cleanFolder == "/" {
		return fmt.Errorf("refusing to delete invalid path: %s", folderPath)
	}
	rel, err := filepath.Rel(cleanExports, cleanFolder)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return fmt.Errorf("folder %s is outside exports directory %s", cleanFolder, cleanExports)
	}
	if err := os.RemoveAll(cleanFolder); err != nil {
		return err
	}
	// Clean up parent chat directory if no period folders remain inside it
	parentChatDir := filepath.Dir(cleanFolder)
	if parentChatDir != cleanExports {
		if entries, err := os.ReadDir(parentChatDir); err == nil {
			hasSubdirs := false
			for _, e := range entries {
				if e.Name() != ".DS_Store" {
					hasSubdirs = true
					break
				}
			}
			if !hasSubdirs {
				_ = os.RemoveAll(parentChatDir)
			}
		}
	}
	return nil
}

/*
DiscoverAllPeriodFolders scans exportsDir and returns all valid period summary records sorted newest first.
*/
func DiscoverAllPeriodFolders(exportsDir string) []PeriodSummaryRecord {
	var records []PeriodSummaryRecord
	chatDirs, err := os.ReadDir(exportsDir)
	if err != nil {
		return records
	}

	for _, cEntry := range chatDirs {
		if !cEntry.IsDir() || strings.HasPrefix(cEntry.Name(), ".") {
			continue
		}
		chatPath := filepath.Join(exportsDir, cEntry.Name())
		periodDirs, err := os.ReadDir(chatPath)
		if err != nil {
			continue
		}
		sort.Slice(periodDirs, func(i, j int) bool {
			return periodDirs[i].Name() > periodDirs[j].Name()
		})

		for _, pEntry := range periodDirs {
			if !pEntry.IsDir() || strings.HasPrefix(pEntry.Name(), ".") {
				continue
			}
			pPath := filepath.Join(chatPath, pEntry.Name())
			rec, err := LoadPeriodRun(pPath)
			if err == nil && rec != nil {
				records = append(records, *rec)
			}
		}
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].CreatedAt > records[j].CreatedAt
	})
	return records
}

/*
EnsureDemoPeriodFolders seeds sample period export folders if exportsDir is empty and returns all discovered records.
*/
func EnsureDemoPeriodFolders(cfg *config.AppConfig) []PeriodSummaryRecord {
	existing := DiscoverAllPeriodFolders(cfg.ExportsDir)
	if len(existing) > 0 {
		return existing
	}

	// Seed Demo Folder 1: Core Backend & Infra
	f1 := filepath.Join(cfg.ExportsDir, "core_backend_infra", "2026-10-04_to_2026-11-03")
	_ = os.MkdirAll(filepath.Join(f1, "images"), 0o755)
	_ = os.MkdirAll(filepath.Join(f1, "documents"), 0o755)
	_ = os.MkdirAll(filepath.Join(f1, "voice"), 0o755)

	img1 := filepath.Join(f1, "images", "img_20261015_msg402_auth_schema.png")
	doc1 := filepath.Join(f1, "documents", "msg418_API_v2_Migration_Spec.pdf")
	doc2 := filepath.Join(f1, "documents", "msg455_Q4_Latency_Benchmark.csv")
	voice1 := filepath.Join(f1, "voice", "voice_20261014_msg398.ogg")
	voice2 := filepath.Join(f1, "voice", "voice_20261028_msg490.ogg")

	_ = os.WriteFile(img1, []byte("PNG_PLACEHOLDER_AUTH_SCHEMA"), 0o644)
	_ = os.WriteFile(doc1, []byte("API v2 Migration Spec - OAuth2 + MTProto Worker Pool Architecture"), 0o644)
	_ = os.WriteFile(doc2, []byte("endpoint,p50_ms,p99_ms\n/v2/summarize,120,340\n/v2/transcribe,410,890\n"), 0o644)
	_ = os.WriteFile(voice1, []byte("OGG_VOICE_SAMPLE_1"), 0o644)
	_ = os.WriteFile(strings.TrimSuffix(voice1, ".ogg")+".txt", []byte("Guys, quick voice update: we need to freeze the v1 REST endpoints by November 15th and move all token refresh logic into the new Redis cluster."), 0o644)
	_ = os.WriteFile(voice2, []byte("OGG_VOICE_SAMPLE_2"), 0o644)
	_ = os.WriteFile(strings.TrimSuffix(voice2, ".ogg")+".txt", []byte("Checked the benchmark numbers in the CSV—p99 latency dropped by 38% after switching to async batching. Let's ship to staging tomorrow."), 0o644)

	rec1 := PeriodSummaryRecord{
		FolderPath:    f1,
		ChatID:        -1001849201,
		ChatTitle:     "Core Backend & Infra",
		PeriodSpec:    "1m",
		PeriodStart:   "2026-10-04",
		PeriodEnd:     "2026-11-03",
		Prompt:        cfg.DefaultNotes,
		Skill:         cfg.DefaultSkill,
		Status:        "completed",
		ModelUsed:     cfg.ActiveModelLabel(),
		TotalMessages: 148,
		VoiceCount:    2,
		ImageCount:    1,
		DocumentCount: 2,
		ExecutiveSummary: "Over the 1-month period (Oct 4 -> Nov 3), the Core Backend & Infra team finalized the API v2 Migration architecture and validated the new Redis cluster session store.\n" +
			"Benchmarks confirmed a 38% reduction in p99 latency (340ms on /v2/summarize), clearing the path for staging deployment and full v1 deprecation on November 15.",
		KeyDecisions: "• v1 Endpoint Freeze: Agreed to freeze legacy REST endpoints by Nov 15 (msg #398).\n" +
			"• Redis Token Refresh: All session/token refresh state moves to the Redis Cluster with 24h TTL (msg #402).\n" +
			"• 3-Phase Cutover Plan: Approved in API_v2_Migration_Spec.pdf (msg #418).",
		VoiceMediaInsights: "• [VOICE] voice_20261014_msg398.ogg (Alexey, 44s): Set the Nov 15 deadline for v1 REST freeze and mandated Redis token refresh.\n" +
			"• [VOICE] voice_20261028_msg490.ogg (Dmitry, 31s): Confirmed 38% p99 latency improvement from async batching and approved staging push.\n" +
			"• [IMAGE] images/img_20261015_msg402_auth_schema.png: Whiteboard architecture diagram mapping API Gateway -> Redis Cluster -> Postgres replica.\n" +
			"• [DOC] documents/msg418_API_v2_Migration_Spec.pdf: Detailed rollout dates (Nov 5 dual-write, Nov 12 read-cutover, Nov 15 deprecation).\n" +
			"• [DOC] documents/msg455_Q4_Latency_Benchmark.csv: Recorded /v2/summarize at 120ms p50 / 340ms p99.",
		ActionItems: "[x] Publish API_v2_Migration_Spec.pdf and Redis schema diagram (@elena_arch)\n" +
			"[x] Run Q4 p50/p99 latency benchmark and export CSV (@dmitry_sre)\n" +
			"[ ] Enable Phase 1 dual-write in production on Nov 5 (@alex_lead)\n" +
			"[ ] Execute read-cutover on Nov 12 and deprecate v1 on Nov 15 (@team)",
		CreatedAt: "2026-11-03 19:15:00 UTC",
		MediaItems: []MediaItemRecord{
			{
				MessageID:     398,
				MediaType:     "voice",
				FileName:      filepath.Base(voice1),
				FilePath:      voice1,
				Sender:        "Alexey (@alex_lead)",
				Timestamp:     "2026-10-14 11:20",
				DurationSec:   44,
				FileSizeBytes: 84200,
				AIText:        "Guys, quick voice update: we need to freeze the v1 REST endpoints by November 15th and move all token refresh logic into the new Redis cluster.",
				DeepLink:      "https://t.me/c/1849201/398",
			},
			{
				MessageID:     402,
				MediaType:     "image",
				FileName:      filepath.Base(img1),
				FilePath:      img1,
				Sender:        "Elena (@elena_arch)",
				Timestamp:     "2026-10-15 14:05",
				FileSizeBytes: 245120,
				Caption:       "Updated whiteboard diagram for the Redis session store",
				AIText:        "Architecture diagram showing Client -> API Gateway -> Redis Cluster (TTL 24h) -> Postgres replica with fallback queue.",
				DeepLink:      "https://t.me/c/1849201/402",
			},
			{
				MessageID:     418,
				MediaType:     "document",
				FileName:      filepath.Base(doc1),
				FilePath:      doc1,
				Sender:        "Elena (@elena_arch)",
				Timestamp:     "2026-10-19 16:40",
				FileSizeBytes: 512400,
				Caption:       "Final PDF spec for API v2 migration",
				AIText:        "Defines 3-phase rollout for API v2: Phase 1 dual-write (Nov 5), Phase 2 read-cutover (Nov 12), Phase 3 v1 deprecation (Nov 15).",
				DeepLink:      "https://t.me/c/1849201/418",
			},
		},
		Messages: []MessageRecord{
			{
				MessageID: 395,
				Sender:    "Elena (@elena_arch)",
				Timestamp: "2026-10-14 10:55",
				Text:      "Morning team, we need to finalize the API v2 cutover schedule this week.",
				DeepLink:  "https://t.me/c/1849201/395",
			},
			{
				MessageID:       398,
				Sender:          "Alexey (@alex_lead)",
				Timestamp:       "2026-10-14 11:20",
				MediaType:       "voice",
				MediaFile:       filepath.Base(voice1),
				VoiceTranscript: "Guys, quick voice update: we need to freeze the v1 REST endpoints by November 15th and move all token refresh logic into the new Redis cluster.",
				DeepLink:        "https://t.me/c/1849201/398",
			},
			{
				MessageID:     402,
				Sender:        "Elena (@elena_arch)",
				Timestamp:     "2026-10-15 14:05",
				Text:          "Updated whiteboard diagram for the Redis session store",
				MediaType:     "image",
				MediaFile:     filepath.Base(img1),
				MediaAnalysis: "Architecture diagram showing Client -> API Gateway -> Redis Cluster (TTL 24h) -> Postgres replica.",
				DeepLink:      "https://t.me/c/1849201/402",
			},
		},
	}
	_ = SavePeriodRun(rec1)

	// Seed Demo Folder 2: Product Design & Specs
	f2 := filepath.Join(cfg.ExportsDir, "product_design_specs", "2026-10-04_to_2026-11-03")
	rec2 := PeriodSummaryRecord{
		FolderPath:    f2,
		ChatID:        -1001928374,
		ChatTitle:     "Product Design & Specs",
		PeriodSpec:    "1m",
		PeriodStart:   "2026-10-04",
		PeriodEnd:     "2026-11-03",
		Prompt:        "Summarize UI/UX decisions, voice feedback from design reviews, and attached wireframes.",
		Status:        "completed",
		ModelUsed:     cfg.ActiveModelLabel(),
		TotalMessages: 92,
		VoiceCount:    1,
		ImageCount:    2,
		DocumentCount: 1,
		ExecutiveSummary: "The Product Design & Specs channel focused on the Q4 macOS desktop experience and onboarding flow. " +
			"Voice review notes from the Oct 21 critique led to simplifying the sidebar navigation into a single multi-select list.",
		KeyDecisions: "• Adopted Tokyo-Night dark theme contrast ratios for terminal and desktop views.\n" +
			"• Merged period archive browser and live chat selector into the left sidebar.",
		VoiceMediaInsights: "• [VOICE] voice_20261021_msg210.ogg (Maria, 58s): Requested keyboard-first navigation (Space to select chats, s to run summary).\n" +
			"• [IMAGE] 2 wireframes: High-contrast card layout with inline status bar.\n" +
			"• [DOC] Q4_Design_Checklist.pdf: Accessibility and typography sign-off criteria.",
		ActionItems: "[x] Finalize Tokyo-Night color tokens\n" +
			"[ ] Complete user testing on multi-chat batch summarization",
		CreatedAt: "2026-11-03 18:40:00 UTC",
	}
	_ = SavePeriodRun(rec2)

	return []PeriodSummaryRecord{rec1, rec2}
}

/*
BuildSummaryMarkdown formats a PeriodSummaryRecord into a Markdown summary report.
*/
func BuildSummaryMarkdown(rec PeriodSummaryRecord) string {
	skillLine := "legacy prompt (no skill)"
	if rec.Skill != "" {
		skillLine = fmt.Sprintf("%s (%s)", rec.Skill, rec.SkillHash)
	}
	notes := strings.TrimSpace(rec.Prompt)
	if notes == "" {
		notes = "none"
	}
	return fmt.Sprintf(`# Period Summary: %s

- **Period**: %s -> %s (%s)
- **Messages Parsed**: %d
- **Voice Messages Transcribed**: %d
- **Images Stored (images/)**: %d
- **Documents Stored (documents/)**: %d
- **Model**: %s
- **Local Archive**: %s

## Skill Used
- **Skill**: %s
- **Extra Notes**: %s

## Executive Summary
%s

## Key Decisions & Highlights
%s

## Voice Messages, Images & Documents Accounting
%s

## Action Items & Follow-ups
%s
`,
		rec.ChatTitle,
		rec.PeriodStart, rec.PeriodEnd, rec.PeriodSpec,
		rec.TotalMessages,
		rec.VoiceCount,
		rec.ImageCount,
		rec.DocumentCount,
		rec.ModelUsed,
		rec.FolderPath,
		skillLine,
		notes,
		rec.ExecutiveSummary,
		rec.KeyDecisions,
		rec.VoiceMediaInsights,
		rec.ActionItems,
	)
}
