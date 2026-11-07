package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"telepilot/internal/domain/config"
	"telepilot/internal/domain/skills"
	"telepilot/internal/domain/storage"
)

// PendingText marks media that still needs AI analysis.
const PendingText = "Requires AI transcription"

// builtinMediaSkill is used only when no media skill (not even a "*" fallback) exists on disk.
var builtinMediaSkill = skills.Skill{
	Name: "built-in",
	Kind: skills.KindMedia,
	Body: "Inspect this file. Transcribe any speech in its original language, otherwise describe the key content, text, or visuals in 1-2 concise sentences.",
	Hash: "builtin",
}

// builtinSummarySkill is used only when no summary skill exists on disk.
var builtinSummarySkill = skills.Skill{
	Name: "built-in",
	Kind: skills.KindSummary,
	Body: "Summarize the chat period. Account for every message, voice transcript, and media file. key_decisions and voice_media_insights are bullet lists; action_items is a checklist.",
	Hash: "builtin",
}

// MediaResult is the outcome of analyzing one media file with a media skill.
type MediaResult struct {
	Text      string
	MediaKind string
	Skill     string
	SkillHash string
}

// SummaryResult is the outcome of summarizing a period with a summary skill.
type SummaryResult struct {
	ExecutiveSummary   string
	KeyDecisions       string
	VoiceMediaInsights string
	ActionItems        string
	Skill              string
	SkillHash          string
}

// IsPending reports whether an AI text is a placeholder rather than a real result.
func IsPending(text string) bool {
	s := strings.TrimSpace(text)
	return s == "" || s == PendingText || strings.HasPrefix(s, "[Voice note ")
}

// Skills loads all skills from disk. It reads fresh every time so edits made in the
// user's editor are picked up without restarting the app.
func (e *Engine) Skills() ([]skills.Skill, []error) {
	return skills.Load(e.cfg.SkillsDir())
}

// MediaSkillFor returns the media skill that handles the given stored media item.
func (e *Engine) MediaSkillFor(mediaType, filePath string) (skills.Skill, string) {
	kind := skills.ClassifyMedia(mediaType, filePath)
	list, _ := e.Skills()
	if sk, ok := skills.ForMedia(list, kind); ok {
		return sk, kind
	}
	return builtinMediaSkill, kind
}

// SummarySkill resolves a summary skill by name, falling back to DEFAULT_SKILL,
// then to the first summary skill on disk, then to a built-in one.
func (e *Engine) SummarySkill(name string) skills.Skill {
	list, _ := e.Skills()
	summaries := skills.Summaries(list)
	for _, n := range []string{name, e.cfg.DefaultSkill, config.DefaultSkillName} {
		if sk, ok := skills.Find(summaries, n); ok {
			return sk
		}
	}
	if len(summaries) > 0 {
		return summaries[0]
	}
	return builtinSummarySkill
}

// usesSidecarCache reports whether the media kind caches its result in a <file>.txt sidecar
// (voice / round video files live in voice/ where a .txt never collides with real media).
func usesSidecarCache(kind string) bool {
	return kind == skills.MediaVoice || kind == skills.MediaVideoNote
}

var inlineTextExts = map[string]bool{
	".txt": true, ".md": true, ".csv": true, ".json": true, ".log": true, ".yaml": true, ".yml": true,
}

// AnalyzeMedia analyzes any media file using the media skill routed by its kind.
// When force is false, a valid cached voice transcript sidecar is reused.
func (e *Engine) AnalyzeMedia(ctx context.Context, mediaType, filePath, caption string, force bool) MediaResult {
	sk, kind := e.MediaSkillFor(mediaType, filePath)
	res := MediaResult{Text: PendingText, MediaKind: kind, Skill: sk.Name, SkillHash: sk.Hash}

	sidecar := strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".txt"
	if usesSidecarCache(kind) && !force {
		if data, err := os.ReadFile(sidecar); err == nil {
			if existing := strings.TrimSpace(string(data)); !IsPending(existing) {
				res.Text = existing
				return res
			}
		}
	}

	attach := filePath
	inline := ""
	if kind == skills.MediaDocument && inlineTextExts[strings.ToLower(filepath.Ext(filePath))] {
		if data, err := os.ReadFile(filePath); err == nil && len(data) > 0 {
			snippet := string(data)
			if len(snippet) > 4000 {
				snippet = snippet[:4000] + "\n...[truncated]"
			}
			inline = snippet
			attach = ""
		}
	}

	if strings.TrimSpace(caption) == "" {
		caption = "(none)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "You are analyzing one media attachment from a Telegram chat.\n")
	fmt.Fprintf(&b, "Media kind: %s\nFile name: %s\nMessage caption/text: %s\n\n", kind, filepath.Base(filePath), caption)
	fmt.Fprintf(&b, "Follow these skill instructions (skill: %s):\n<skill>\n%s\n</skill>\n", sk.Name, sk.Body)
	if inline != "" {
		fmt.Fprintf(&b, "\nDocument content:\n<document>\n%s\n</document>\n", inline)
	}
	if e.cfg.IsRussian() {
		b.WriteString("\nLanguage requirement (MANDATORY): The preferred language is Russian (ru). Produce all final analysis, transcriptions, and descriptions in Russian (speech transcription in original spoken language is acceptable if accompanied by Russian notes).\n")
	} else {
		b.WriteString("\nLanguage requirement: The preferred language is English (en). Produce all output in English.\n")
	}
	b.WriteString("\nReturn ONLY the result text, without preamble.")

	if e.cfg.AgyCLIBin() != "" {
		if out, err := e.callAgyCLI(ctx, b.String(), attach); err == nil && out != "" {
			res.Text = out
		}
	}
	if usesSidecarCache(kind) {
		_ = os.WriteFile(sidecar, []byte(res.Text), 0o644)
	}
	return res
}

// AnalyzeMediaItem re-analyzes a stored media item, skipping it when it was already
// analyzed successfully by the exact same skill version (unless force is set).
func (e *Engine) AnalyzeMediaItem(ctx context.Context, it storage.MediaItemRecord, caption string, force bool) (MediaResult, bool) {
	sk, kind := e.MediaSkillFor(it.MediaType, it.FilePath)
	if !force && it.Skill == sk.Name && it.SkillHash == sk.Hash && !IsPending(it.AIText) {
		return MediaResult{Text: it.AIText, MediaKind: kind, Skill: sk.Name, SkillHash: sk.Hash}, true
	}
	return e.AnalyzeMedia(ctx, it.MediaType, it.FilePath, caption, true), false
}

// GeneratePeriodSummary summarizes a period using the named summary skill plus optional notes.
// The JSON output contract is enforced here, so skills only control the content.
func (e *Engine) GeneratePeriodSummary(
	ctx context.Context,
	chatTitle string,
	periodLabel string,
	skillName string,
	notes string,
	messages []storage.MessageRecord,
	mediaItems []storage.MediaItemRecord,
) SummaryResult {
	sk := e.SummarySkill(skillName)
	out := SummaryResult{Skill: sk.Name, SkillHash: sk.Hash}

	var transcriptLines []string
	limit := len(messages)
	if limit > 250 {
		limit = 250
	}
	for _, m := range messages[:limit] {
		line := fmt.Sprintf("[%s] %s (#%d): %s", m.Timestamp, m.Sender, m.MessageID, m.Text)
		if m.VoiceTranscript != "" {
			line += fmt.Sprintf(" [VOICE TRANSCRIPT (%s): %s]", m.MediaFile, m.VoiceTranscript)
		}
		if m.MediaAnalysis != "" {
			line += fmt.Sprintf(" [%s (%s): %s]", strings.ToUpper(m.MediaType), m.MediaFile, m.MediaAnalysis)
		}
		transcriptLines = append(transcriptLines, line)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "You are summarizing a Telegram chat period.\nChat: %s\nPeriod: %s\n\n", chatTitle, periodLabel)
	fmt.Fprintf(&b, "Follow these skill instructions (skill: %s):\n<skill>\n%s\n</skill>\n", sk.Name, sk.Body)
	if n := strings.TrimSpace(notes); n != "" {
		fmt.Fprintf(&b, "\nAdditional notes from the user for this run:\n%s\n", n)
	}
	if e.cfg.IsRussian() {
		b.WriteString(`
Language requirement (MANDATORY):
The preferred language is Russian (ru). All output values for "executive_summary", "key_decisions", "voice_media_insights", and "action_items" MUST be written in natural, fluent Russian.
(Keep the 4 JSON keys in English as specified below: executive_summary, key_decisions, voice_media_insights, action_items).
`)
	} else {
		b.WriteString(`
Language requirement (MANDATORY):
The preferred language is English (en). All output values MUST be written in English.
`)
	}
	b.WriteString(`
Output contract (mandatory; overrides any formatting guidance above):
Return ONLY valid JSON with these exact 4 string keys. Every value must be a single string
(use "\n" line breaks and "• " bullets inside the strings where needed):
{
  "executive_summary": "...",
  "key_decisions": "...",
  "voice_media_insights": "...",
  "action_items": "..."
}

Messages and Transcribed Media:
`)
	b.WriteString(strings.Join(transcriptLines, "\n"))

	var rawOut string
	if e.cfg.AgyCLIBin() != "" {
		rawOut, _ = e.callAgyCLI(ctx, b.String(), "")
	}

	if rawOut != "" {
		cleaned := strings.TrimSpace(rawOut)
		cleaned = strings.TrimPrefix(cleaned, "```json")
		cleaned = strings.TrimPrefix(cleaned, "```")
		cleaned = strings.TrimSuffix(cleaned, "```")
		cleaned = strings.TrimSpace(cleaned)

		startIdx := strings.IndexByte(cleaned, '{')
		endIdx := strings.LastIndexByte(cleaned, '}')
		if startIdx >= 0 && endIdx > startIdx {
			cleaned = cleaned[startIdx : endIdx+1]
		}

		var sec summarySections
		if err := json.Unmarshal([]byte(cleaned), &sec); err == nil && sec.ExecutiveSummary != "" {
			out.ExecutiveSummary = sec.ExecutiveSummary
			out.KeyDecisions = sec.KeyDecisions
			out.VoiceMediaInsights = sec.VoiceMediaInsights
			out.ActionItems = sec.ActionItems
			return out
		}
		out.ExecutiveSummary = rawOut
		if e.cfg.IsRussian() {
			out.KeyDecisions = "• Извлечено выше через " + e.cfg.ActiveModelLabel()
			out.VoiceMediaInsights = formatMediaAccounting(mediaItems, e.cfg.Language)
			out.ActionItems = "[ ] Проверить сформированную выше сводку"
		} else {
			out.KeyDecisions = "• Extracted above via " + e.cfg.ActiveModelLabel()
			out.VoiceMediaInsights = formatMediaAccounting(mediaItems, e.cfg.Language)
			out.ActionItems = "[ ] Review generated summary above"
		}
		return out
	}

	// Deterministic local summary synthesis (agy unavailable or failed)
	voiceCount, imgCount, docCount := 0, 0, 0
	for _, it := range mediaItems {
		switch it.MediaType {
		case "voice":
			voiceCount++
		case "image":
			imgCount++
		case "document":
			docCount++
		}
	}

	if e.cfg.IsRussian() {
		out.ExecutiveSummary = fmt.Sprintf(
			"Обработано сообщений: %d в %s за период %s (Голосовые: %d, Изображения: %d, Документы: %d).\nНавык: %s (ИИ недоступен — локальная сводка)",
			len(messages), chatTitle, periodLabel, voiceCount, imgCount, docCount, sk.Name,
		)
		var highlights []string
		for i := 0; i < len(messages) && len(highlights) < 5; i++ {
			m := messages[i]
			if strings.TrimSpace(m.Text) != "" {
				txt := m.Text
				if len(txt) > 120 {
					txt = txt[:120] + "..."
				}
				highlights = append(highlights, fmt.Sprintf("• [#%d %s]: %s", m.MessageID, m.Sender, txt))
			}
		}
		if len(highlights) == 0 {
			highlights = append(highlights, "• Нет текстовых сообщений в выбранном диапазоне.")
		}
		out.KeyDecisions = strings.Join(highlights, "\n")
		out.VoiceMediaInsights = formatMediaAccounting(mediaItems, e.cfg.Language)
		out.ActionItems = "[x] Архивировать сообщения и медиа в локальную папку периода\n[ ] Просмотреть загруженные голосовые заметки, изображения и документы"
		return out
	}

	out.ExecutiveSummary = fmt.Sprintf(
		"Parsed %d messages in %s for period %s (Voice notes: %d, Images: %d, Documents: %d).\nSkill: %s (AI unavailable — local fallback summary)",
		len(messages), chatTitle, periodLabel, voiceCount, imgCount, docCount, sk.Name,
	)

	var highlights []string
	for i := 0; i < len(messages) && len(highlights) < 5; i++ {
		m := messages[i]
		if strings.TrimSpace(m.Text) != "" {
			txt := m.Text
			if len(txt) > 120 {
				txt = txt[:120] + "..."
			}
			highlights = append(highlights, fmt.Sprintf("• [#%d %s]: %s", m.MessageID, m.Sender, txt))
		}
	}
	if len(highlights) == 0 {
		highlights = append(highlights, "• No text messages in selected range.")
	}
	out.KeyDecisions = strings.Join(highlights, "\n")
	out.VoiceMediaInsights = formatMediaAccounting(mediaItems, e.cfg.Language)
	out.ActionItems = "[x] Archive messages and media into local period folder\n[ ] Review downloaded voice notes, images, and documents"
	return out
}

// ExecuteChatSkill runs the selected skill directly on the chat's messages without any rigid 4-card JSON schema,
// returning the free-form AI output to display in Tab 3 (AI Output).
func (e *Engine) ExecuteChatSkill(
	ctx context.Context,
	chat storage.ChatInfo,
	skillName string,
	notes string,
	messages []storage.LiveMessage,
) (storage.SkillOutputRecord, error) {
	list, _ := e.Skills()
	sk, ok := skills.Find(list, skillName)
	if !ok {
		sk = e.SummarySkill(skillName)
	}

	var msgLines []string
	start := 0
	if len(messages) > 100 {
		start = len(messages) - 100
	}
	for _, m := range messages[start:] {
		sender := strings.TrimSpace(m.Sender)
		if sender == "" {
			sender = "User"
		}
		text := strings.TrimSpace(m.Text)
		if m.MediaType != "" {
			tag := strings.ToUpper(m.MediaType)
			if text != "" {
				text = fmt.Sprintf("%s [%s: %s]", text, tag, m.MediaFileName)
			} else {
				text = fmt.Sprintf("[%s: %s]", tag, m.MediaFileName)
			}
		}
		if text != "" {
			msgLines = append(msgLines, fmt.Sprintf("[%s] %s (#%d): %s", m.Timestamp, sender, m.MessageID, text))
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "You are executing the AI skill %q on the Telegram chat %q.\n\n", sk.Name, chat.Title)
	fmt.Fprintf(&b, "Follow these skill instructions (skill: %s):\n<skill>\n%s\n</skill>\n", sk.Name, sk.Body)
	if n := strings.TrimSpace(notes); n != "" {
		fmt.Fprintf(&b, "\nAdditional per-chat instructions from the user:\n%s\n", n)
	}
	if e.cfg.IsRussian() {
		b.WriteString("\nLanguage requirement: Produce the output in fluent Russian (ru), unless the skill instructions explicitly require another language.\n")
	} else {
		b.WriteString("\nLanguage requirement: Produce the output in English (en), unless the skill instructions explicitly require another language.\n")
	}
	b.WriteString("\nProvide your output directly in clean, readable plain text / Markdown bullet points following the skill instructions above (do NOT wrap the response in JSON unless the skill explicitly asks for JSON).\n\nChat Messages:\n")
	b.WriteString(strings.Join(msgLines, "\n"))

	rec := storage.SkillOutputRecord{
		ChatID:       chat.ChatID,
		ChatTitle:    chat.Title,
		Skill:        sk.Name,
		SkillHash:    sk.Hash,
		SkillDesc:    sk.Description,
		Notes:        strings.TrimSpace(notes),
		ModelUsed:    e.cfg.ActiveModelLabel(),
		MessageCount: len(messages),
		CreatedAt:    time.Now().UTC().Format("2006-01-02 15:04:05 UTC"),
	}

	if e.cfg.AgyCLIBin() != "" {
		out, err := e.callAgyCLI(ctx, b.String(), "")
		if err != nil {
			return rec, err
		}
		rec.Output = strings.TrimSpace(out)
		return rec, nil
	}

	// Deterministic local fallback when agy CLI is not configured
	var highlights []string
	for i := 0; i < len(messages) && len(highlights) < 6; i++ {
		m := messages[i]
		if strings.TrimSpace(m.Text) != "" {
			highlights = append(highlights, fmt.Sprintf("• [%s] %s: %s", m.Timestamp, m.Sender, m.Text))
		}
	}
	if e.cfg.IsRussian() {
		rec.Output = fmt.Sprintf("Выполнен навык %q для чата %q (%d сообщ.):\n\n%s",
			sk.Name, chat.Title, len(messages), strings.Join(highlights, "\n"))
	} else {
		rec.Output = fmt.Sprintf("Executed skill %q on chat %q (%d messages):\n\n%s",
			sk.Name, chat.Title, len(messages), strings.Join(highlights, "\n"))
	}
	return rec, nil
}
