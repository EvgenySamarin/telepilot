package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"telepilot/internal/config"
)

type ChatInfo struct {
	ChatID      int64  `json:"chat_id"`
	AccessHash  int64  `json:"access_hash"`
	Title       string `json:"title"`
	ChatType    string `json:"chat_type"` // "group" | "channel" | "dm"
	UnreadCount int    `json:"unread_count"`
	Username    string `json:"username"`
}

type MediaItemRecord struct {
	MessageID     int    `json:"message_id"`
	MediaType     string `json:"media_type"` // "voice" | "image" | "document"
	FileName      string `json:"file_name"`
	FilePath      string `json:"file_path"`
	Sender        string `json:"sender"`
	Timestamp     string `json:"timestamp"`
	DurationSec   int    `json:"duration_sec"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	Caption       string `json:"caption"`
	AIText        string `json:"ai_text"`
	DeepLink      string `json:"deep_link"`
	Skill         string `json:"skill,omitempty"`      // media skill that produced AIText
	SkillHash     string `json:"skill_hash,omitempty"` // hash of that skill's SKILL.md at analysis time
}

type MessageRecord struct {
	MessageID       int    `json:"message_id"`
	Sender          string `json:"sender"`
	Timestamp       string `json:"timestamp"`
	Text            string `json:"text"`
	ReplyToMsgID    int    `json:"reply_to_msg_id,omitempty"`
	MediaType       string `json:"media_type,omitempty"`
	MediaFile       string `json:"media_file,omitempty"`
	VoiceTranscript string `json:"voice_transcript,omitempty"`
	MediaAnalysis   string `json:"media_analysis,omitempty"`
	DeepLink        string `json:"deep_link"`
}

type PeriodSummaryRecord struct {
	FolderPath         string            `json:"folder_path"`
	ChatID             int64             `json:"chat_id"`
	ChatTitle          string            `json:"chat_title"`
	PeriodSpec         string            `json:"period_spec"`
	PeriodStart        string            `json:"period_start"`
	PeriodEnd          string            `json:"period_end"`
	Prompt             string            `json:"prompt"` // extra notes applied on top of the skill
	Skill              string            `json:"skill,omitempty"`
	SkillHash          string            `json:"skill_hash,omitempty"`
	Status             string            `json:"status"`
	ModelUsed          string            `json:"model_used"`
	TotalMessages      int               `json:"total_messages"`
	VoiceCount         int               `json:"voice_count"`
	ImageCount         int               `json:"image_count"`
	DocumentCount      int               `json:"document_count"`
	ExecutiveSummary   string            `json:"executive_summary"`
	KeyDecisions       string            `json:"key_decisions"`
	VoiceMediaInsights string            `json:"voice_media_insights"`
	ActionItems        string            `json:"action_items"`
	CreatedAt          string            `json:"created_at"`
	MediaItems         []MediaItemRecord `json:"media_items"`
	Messages           []MessageRecord   `json:"messages"`
}

var DemoChats = []ChatInfo{
	{ChatID: -1001849201, Title: "Core Backend & Infra", ChatType: "group", UnreadCount: 14},
	{ChatID: -1001928374, Title: "Product Design & Specs", ChatType: "group", UnreadCount: 6},
	{ChatID: -1001554321, Title: "Founders Weekly Sync", ChatType: "group", UnreadCount: 3},
	{ChatID: -1001778899, Title: "AI Research & Papers", ChatType: "channel", UnreadCount: 28},
	{ChatID: 48291042, Title: "Alexey (Tech Lead)", ChatType: "dm", UnreadCount: 2},
}

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

