package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"telepilot/internal/domain/config"
	"telepilot/internal/domain/storage"
)

type Engine struct {
	cfg *config.AppConfig
}

func NewEngine(cfg *config.AppConfig) *Engine {
	return &Engine{cfg: cfg}
}

type summarySections struct {
	ExecutiveSummary   string `json:"executive_summary"`
	KeyDecisions       string `json:"key_decisions"`
	VoiceMediaInsights string `json:"voice_media_insights"`
	ActionItems        string `json:"action_items"`
}

func (e *Engine) callAgyCLI(ctx context.Context, prompt string, mediaPath string) (string, error) {
	bin := e.cfg.AgyCLIBin()
	if bin == "" {
		return "", fmt.Errorf("antigravity CLI (agy) not found")
	}

	cctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()

	var args []string
	finalPrompt := prompt
	if strings.TrimSpace(mediaPath) != "" {
		if st, err := os.Stat(mediaPath); err == nil && !st.IsDir() {
			args = append(args, "--add-dir", filepath.Dir(mediaPath))
			finalPrompt = fmt.Sprintf("File: @%s (%s)\n\n%s", mediaPath, mediaPath, prompt)
		}
	}

	model := strings.TrimSpace(e.cfg.AgyModel)
	if model != "" && model != "auto" && model != "default" {
		args = append(args, "--model", model)
	}

	args = append(args, "-p", finalPrompt)

	cmd := exec.CommandContext(cctx, bin, args...)
	if e.cfg.ProjectRoot != "" {
		cmd.Dir = e.cfg.ProjectRoot
	}
	out, err := cmd.CombinedOutput()
	text := strings.TrimSpace(string(out))
	if err != nil {
		if text != "" {
			return "", fmt.Errorf("%v (%s)", err, firstLine(text))
		}
		return "", err
	}
	if text == "" {
		return "", fmt.Errorf("empty output from agy CLI")
	}
	if strings.Contains(text, "Opening authentication page") || strings.Contains(text, "Do you want to continue?") {
		return "", fmt.Errorf("agy CLI requires interactive authentication (run 'agy' in terminal once)")
	}
	return text, nil
}

func (e *Engine) TestConnection(ctx context.Context) (bool, string) {
	t0 := time.Now()
	bin := e.cfg.AgyCLIBin()
	if bin == "" {
		return false, "[!] Antigravity CLI (agy) binary not found. Install agy or set path with 'c'."
	}
	res, err := e.callAgyCLI(ctx, "Reply with the exact string: OK", "")
	if err != nil {
		return false, fmt.Sprintf("[FAIL] Antigravity CLI (%s) error: %v", bin, err)
	}
	ms := time.Since(t0).Milliseconds()
	model := strings.TrimSpace(e.cfg.AgyModel)
	if model == "" {
		model = "auto"
	}
	return true, fmt.Sprintf("[OK] Connected to Antigravity CLI (%s, model=%s) in %dms -> %s", bin, model, ms, firstLine(res))
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		s = strings.TrimSpace(s[:idx])
	}
	if len(s) > 50 {
		return s[:50] + "..."
	}
	return s
}

func formatMediaAccounting(items []storage.MediaItemRecord, lang string) string {
	isRu := strings.ToLower(lang) == "ru"
	if len(items) == 0 {
		if isRu {
			return "• В этом периоде нет голосовых сообщений, изображений или документов."
		}
		return "• No voice messages, images, or documents in this period."
	}
	var lines []string
	for _, it := range items {
		tag := strings.ToUpper(it.MediaType)
		if tag == "DOCUMENT" {
			tag = "DOC"
		}
		lines = append(lines, fmt.Sprintf("• [%s] %s (%s): %s", tag, it.FileName, it.Sender, it.AIText))
	}
	return strings.Join(lines, "\n")
}

// GenerateChatReply generates a conversational reply for a live Telegram chat using recent messages,
// the chat's assigned skill, and per-chat custom AI instructions.
func (e *Engine) GenerateChatReply(ctx context.Context, chatTitle string, recent []storage.LiveMessage, skillName string, customNotes string) (string, error) {
	if len(recent) > 15 {
		recent = recent[len(recent)-15:]
	}
	var historyLines []string
	for _, m := range recent {
		sender := strings.TrimSpace(m.Sender)
		if sender == "" {
			sender = "User"
		}
		text := strings.TrimSpace(m.Text)
		if text == "" && m.MediaType != "" {
			text = fmt.Sprintf("[%s: %s]", strings.ToUpper(m.MediaType), m.MediaFileName)
		}
		if text != "" {
			historyLines = append(historyLines, fmt.Sprintf("[%s] %s: %s", m.Timestamp, sender, text))
		}
	}

	var skillInstructions string
	if strings.TrimSpace(skillName) != "" {
		sk := e.SummarySkill(skillName)
		if strings.TrimSpace(sk.Body) != "" {
			skillInstructions = sk.Body
		}
	}

	langHint := "Reply naturally in the same language as the recent messages in the conversation."
	if e.cfg.IsRussian() {
		langHint = "Отвечай естественно на языке последних сообщений чата (по умолчанию на русском)."
	}

	var promptBuilder strings.Builder
	promptBuilder.WriteString(fmt.Sprintf("You are participating on behalf of the user ('You') in the Telegram chat %q.\n", chatTitle))
	promptBuilder.WriteString(langHint + "\n")
	if strings.TrimSpace(customNotes) != "" {
		promptBuilder.WriteString(fmt.Sprintf("\nPer-Chat AI Instructions (highest priority):\n%s\n", strings.TrimSpace(customNotes)))
	} else if strings.TrimSpace(skillInstructions) != "" {
		promptBuilder.WriteString(fmt.Sprintf("\nActive Skill Context (%s):\n%s\n", skillName, strings.TrimSpace(skillInstructions)))
	}
	promptBuilder.WriteString("\nRecent conversation history:\n")
	promptBuilder.WriteString(strings.Join(historyLines, "\n"))
	promptBuilder.WriteString("\n\nWrite ONLY the exact message text to send next as 'You'. Do not include quotes, sender prefixes, markdown code blocks, or explanations.")

	res, err := e.callAgyCLI(ctx, promptBuilder.String(), "")
	if err != nil {
		return "", err
	}
	reply := strings.TrimSpace(res)
	reply = strings.TrimPrefix(reply, "You:")
	reply = strings.TrimSpace(strings.Trim(reply, "\"`"))
	return reply, nil
}

// SingleShotAction represents one outgoing Telegram message planned by a single-shot AI instruction.
type SingleShotAction struct {
	Text         string `json:"text"`
	DelaySeconds int    `json:"delay_seconds"`
}

// PlanSingleShotInstruction executes a one-off, standalone AI instruction for the selected chat
// (unrelated to summarization or skills) and returns one or more messages (with optional delays) to send.
func (e *Engine) PlanSingleShotInstruction(ctx context.Context, chatTitle string, recent []storage.LiveMessage, instruction string) ([]SingleShotAction, error) {
	if len(recent) > 15 {
		recent = recent[len(recent)-15:]
	}
	var historyLines []string
	for _, m := range recent {
		sender := strings.TrimSpace(m.Sender)
		if sender == "" {
			sender = "User"
		}
		text := strings.TrimSpace(m.Text)
		if text == "" && m.MediaType != "" {
			text = fmt.Sprintf("[%s: %s]", strings.ToUpper(m.MediaType), m.MediaFileName)
		}
		if text != "" {
			historyLines = append(historyLines, fmt.Sprintf("[%s] %s: %s", m.Timestamp, sender, text))
		}
	}

	var promptBuilder strings.Builder
	promptBuilder.WriteString(fmt.Sprintf("You are executing a standalone single-shot instruction on behalf of the user ('You') in the Telegram chat %q.\n", chatTitle))
	promptBuilder.WriteString("This is a direct chat action and is NOT a period summarization task.\n\n")
	if len(historyLines) > 0 {
		promptBuilder.WriteString("Recent chat context:\n")
		promptBuilder.WriteString(strings.Join(historyLines, "\n") + "\n\n")
	}
	promptBuilder.WriteString(fmt.Sprintf("User Single-Shot Instruction:\n%s\n\n", strings.TrimSpace(instruction)))
	promptBuilder.WriteString(`Return ONLY a valid JSON array of message objects to send to the chat in order:
[
  {"text": "exact message text to send", "delay_seconds": 0}
]
Rules:
- "text" must be the exact message string to send to Telegram (no extra commentary).
- NEVER send conversational confirmations like "Понял, буду отправлять...", "Привет! Цикл начат...", "Серия завершена" or "Отправляю..." into the chat.
- If the instruction is to schedule future or repeating messages (e.g. "every minute for 5 minutes", "in 30 minutes", "every day at 9"):
  Return EXACTLY ONE item containing ONLY the <schedule> directive and NO chat text, like:
  [{"text": "<schedule in=\"1m\" repeat=\"1m\" count=\"5\" until=\"5m\">exact message or prompt to send</schedule>", "delay_seconds": 0}]
  Attributes: in, at, repeat, count, until.
  Do NOT include any extra items, delays, or confirmation messages.
- Output ONLY the raw JSON array without markdown fences or explanations.`)

	res, err := e.callAgyCLI(ctx, promptBuilder.String(), "")
	if err != nil {
		return nil, err
	}

	actions := parseSingleShotActions(res)
	if len(actions) == 0 {
		return nil, fmt.Errorf("AI returned empty message plan")
	}
	return actions, nil
}

func parseSingleShotActions(raw string) []SingleShotAction {
	trimmed := strings.TrimSpace(raw)
	if start := strings.IndexByte(trimmed, '['); start >= 0 {
		if end := strings.LastIndexByte(trimmed, ']'); end > start {
			jsonSlice := trimmed[start : end+1]
			var parsed []SingleShotAction
			if err := json.Unmarshal([]byte(jsonSlice), &parsed); err == nil {
				var valid []SingleShotAction
				for _, a := range parsed {
					t := strings.TrimSpace(a.Text)
					if t != "" {
						if a.DelaySeconds < 0 {
							a.DelaySeconds = 0
						}
						if a.DelaySeconds > 300 {
							a.DelaySeconds = 300
						}
						valid = append(valid, SingleShotAction{
							Text:         t,
							DelaySeconds: a.DelaySeconds,
						})
					}
				}
				if len(valid) > 0 {
					return valid
				}
			}
		}
	}
	clean := strings.TrimSpace(strings.Trim(trimmed, "\"`"))
	if clean == "" {
		return nil
	}
	return []SingleShotAction{{Text: clean, DelaySeconds: 0}}
}

// GenerateScheduledTaskMessage generates a context-aware message to send to a Telegram chat
// when a background scheduled task or reminder is triggered.
func (e *Engine) GenerateScheduledTaskMessage(ctx context.Context, chatTitle string, recent []storage.LiveMessage, taskPrompt string) (string, error) {
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}
	var historyLines []string
	for _, m := range recent {
		sender := strings.TrimSpace(m.Sender)
		if sender == "" {
			sender = "User"
		}
		text := strings.TrimSpace(m.Text)
		if text != "" {
			historyLines = append(historyLines, fmt.Sprintf("[%s] %s: %s", m.Timestamp, sender, text))
		}
	}

	var promptBuilder strings.Builder
	promptBuilder.WriteString(fmt.Sprintf("You are participating on behalf of the user ('You') in Telegram chat %q.\n", chatTitle))
	promptBuilder.WriteString("A scheduled background reminder / recurring task has just triggered now:\n")
	promptBuilder.WriteString(fmt.Sprintf("Scheduled Task / Reminder: %q\n\n", strings.TrimSpace(taskPrompt)))
	if len(historyLines) > 0 {
		promptBuilder.WriteString("Recent chat conversation context:\n")
		promptBuilder.WriteString(strings.Join(historyLines, "\n") + "\n\n")
	}
	promptBuilder.WriteString("Write ONLY the exact message text to send to the chat now as 'You'. Do NOT include quotes, markdown code fences, prefixes like 'You:', or explanations. Respond naturally in the language of the task or chat.")

	res, err := e.callAgyCLI(ctx, promptBuilder.String(), "")
	if err != nil {
		// Fallback to sending the task prompt directly if CLI is unreachable
		return strings.TrimSpace(taskPrompt), nil
	}
	clean := strings.TrimSpace(res)
	clean = strings.TrimPrefix(clean, "You:")
	clean = strings.TrimSpace(strings.Trim(clean, "\"`"))
	if clean == "" {
		return strings.TrimSpace(taskPrompt), nil
	}
	return clean, nil
}

