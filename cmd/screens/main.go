package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"

	"telepilot/internal/ai"
	"telepilot/internal/config"
	"telepilot/internal/mtproto"
	"telepilot/internal/skills"
	"telepilot/internal/storage"
	"telepilot/internal/tui"
)

type CellJSON struct {
	Char string `json:"char"`
	Fg   string `json:"fg"`
	Bg   string `json:"bg"`
	Bold bool   `json:"bold"`
}

func hexColor(c ansi.Color, defaultHex string) string {
	if c == nil {
		return defaultHex
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

func extractGrid(viewStr string, widthCols, heightRows int) [][]CellJSON {
	buf := cellbuf.NewBuffer(widthCols, heightRows)
	cellbuf.SetContent(buf, viewStr)

	grid := make([][]CellJSON, heightRows)
	for y := 0; y < heightRows; y++ {
		line := buf.Line(y)
		grid[y] = make([]CellJSON, widthCols)
		for x := 0; x < widthCols; x++ {
			charStr := " "
			fg := "#c0caf5"
			bg := ""
			bold := false

			if x < len(line) && line[x] != nil {
				c := line[x]
				if c.Rune != 0 {
					charStr = string(c.Rune)
				}
				if c.Style.Fg != nil {
					fg = hexColor(c.Style.Fg, "#c0caf5")
				}
				if c.Style.Bg != nil {
					bg = hexColor(c.Style.Bg, "")
				}
				if c.Style.Attrs&cellbuf.BoldAttr != 0 {
					bold = true
				}
			}
			grid[y][x] = CellJSON{
				Char: charStr,
				Fg:   fg,
				Bg:   bg,
				Bold: bold,
			}
		}
	}
	return grid
}

func main() {
	lipgloss.SetColorProfile(termenv.TrueColor)
	tmpDir, err := os.MkdirTemp("", "tgp_capture_*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	cwd, _ := os.Getwd()
	cfg := &config.AppConfig{
		ProjectRoot:        cwd,
		TelegramAPIID:      12345,
		TelegramAPIHash:    "dummy_hash",
		TelegramDCID:       2,
		TelegramServerIP:   "149.154.167.50",
		TelegramServerPort: 443,
		AgyModel:           "gemini-2.5-pro",
		DefaultPeriod:      "1m",
		DefaultSkill:       "period-summary",
		Language:           "en",
		ExportsDir:         filepath.Join(tmpDir, "exports"),
		DataDir:            filepath.Join(tmpDir, "data"),
	}

	storage.EnsureDemoPeriodFolders(cfg)
	_ = skills.EnsureDefaults(cfg.SkillsDir())

	aiEng := ai.NewEngine(cfg)
	mtSvc := mtproto.NewService(cfg, aiEng)

	assetsDir := filepath.Join(cwd, "assets")
	_ = os.MkdirAll(assetsDir, 0o755)

	cols := 125
	rows := 36

	screens := []struct {
		name  string
		title string
		setup func() *tui.Model
	}{
		{
			name:  "demo_dashboard",
			title: "telepilot — Tokyo-Night Terminal UI (Executive Summary)",
			setup: func() *tui.Model {
				m := tui.NewModel(cfg, aiEng, mtSvc, true)
				mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
				m = mUpdated.(*tui.Model)
				mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
				mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
				mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeySpace})
				mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
				mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
				return mUpdated.(*tui.Model)
			},
		},
		{
			name:  "media_vault",
			title: "telepilot — Multimodal Media Vault (Voice Transcripts, Diagrams & Specs)",
			setup: func() *tui.Model {
				m := tui.NewModel(cfg, aiEng, mtSvc, true)
				mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
				mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
				mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
				return mUpdated.(*tui.Model)
			},
		},
		{
			name:  "transcript_view",
			title: "telepilot — Chronological Chat Timeline & Inline Voice Transcripts",
			setup: func() *tui.Model {
				m := tui.NewModel(cfg, aiEng, mtSvc, true)
				mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
				mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
				return mUpdated.(*tui.Model)
			},
		},
		{
			name:  "skills_browser",
			title: "telepilot — Modular AI Skills Catalog (.agents/skills) & Re-Prompting",
			setup: func() *tui.Model {
				m := tui.NewModel(cfg, aiEng, mtSvc, true)
				mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
				mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
				return mUpdated.(*tui.Model)
			},
		},
		{
			name:  "qr_login",
			title: "telepilot — MTProto Connection Diagnostics & QR Code Login",
			setup: func() *tui.Model {
				m := tui.NewModel(cfg, aiEng, mtSvc, true)
				mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
				m = mUpdated.(*tui.Model)
				qrPath := filepath.Join(tmpDir, "qr.png")
				qrArt, _ := mtproto.RenderQRHalfBlocks("tg://login?token=AQAAAI_DEMO_TOKEN_TELEPILOT_42x9K", qrPath)
				m.SetQRModalForCapture(
					"[✓] Ready to scan: Open Telegram on your phone → Settings → Devices → Link Desktop Device",
					qrArt,
					qrPath,
				)
				return m
			},
		},
		{
			name:  "language_config",
			title: "telepilot — Global Configuration & Language Switcher (En / Ru)",
			setup: func() *tui.Model {
				m := tui.NewModel(cfg, aiEng, mtSvc, true)
				mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
				m = mUpdated.(*tui.Model)
				m.SetGlobalConfigModalForCapture(1)
				return m
			},
		},
		{
			name:  "skill_editor_preview",
			title: "telepilot — In-TUI Skill Preview & Editor ($EDITOR Integration)",
			setup: func() *tui.Model {
				m := tui.NewModel(cfg, aiEng, mtSvc, true)
				mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
				mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
				m = mUpdated.(*tui.Model)
				m.SetSkillPreviewModalForCapture(0)
				return m
			},
		},
	}

	pyScript := filepath.Join(cwd, "cmd", "screens", "render.py")

	for _, s := range screens {
		m := s.setup()
		grid := extractGrid(m.View(), cols, rows)
		gridJSON, err := json.Marshal(grid)
		if err != nil {
			panic(err)
		}

		outPNG := filepath.Join(assetsDir, s.name+".png")
		cmd := exec.Command(".venv/bin/python3", pyScript, s.title, outPNG)
		stdin, _ := cmd.StdinPipe()
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			fmt.Printf("Error starting python renderer for %s: %v\n", s.name, err)
			continue
		}
		_, _ = stdin.Write(gridJSON)
		_ = stdin.Close()
		if err := cmd.Wait(); err != nil {
			fmt.Printf("Error running python renderer for %s: %v\n", s.name, err)
			continue
		}
		fmt.Printf("✓ Generated pixel-perfect %s.png\n", s.name)
	}
	fmt.Println("\nAll PNG screenshots rendered successfully!")
}
