package ai

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"telepilot/internal/config"
	"telepilot/internal/storage"
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

