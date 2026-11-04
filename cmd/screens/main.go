package main

import (
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

func hexColor(c ansi.Color, defaultHex string) string {
	if c == nil {
		return defaultHex
	}
	r, g, b, _ := c.RGBA()
	r8, g8, b8 := r>>8, g>>8, b>>8
	// Boost very dark muted text for high contrast and readability on dark background
	if r8 < 95 && g8 < 105 && b8 < 145 && r8 > 30 {
		return "#7aa2f7" // Tokyo Night readable accent blue
	}
	return fmt.Sprintf("#%02x%02x%02x", r8, g8, b8)
}

func RenderTerminalSVG(viewStr string, widthCols, heightRows int, title string) string {
	buf := cellbuf.NewBuffer(widthCols, heightRows)
	cellbuf.SetContent(buf, viewStr)

	charW := 8.0
	charH := 18.0
	padX := 24.0
	padTop := 44.0 // title bar height
	padBottom := 20.0

	termW := float64(widthCols) * charW
	termH := float64(heightRows) * charH
	windowW := termW + padX*2
	windowH := termH + padTop + padBottom

	viewBoxW := windowW + 50
	viewBoxH := windowH + 50
	originX := 25.0
	originY := 20.0

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %.1f %.1f" width="%.1f" height="%.1f">
  <defs>
    <filter id="shadow" x="-5%%" y="-5%%" width="110%%" height="115%%">
      <feDropShadow dx="0" dy="18" stdDeviation="22" flood-color="#000000" flood-opacity="0.6"/>
    </filter>
  </defs>
  <style>
    .mono { font-family: ui-monospace, 'SF Mono', Menlo, Monaco, Consolas, monospace; font-size: 13px; font-variant-ligatures: none; }
  </style>
  <rect width="100%%" height="100%%" fill="#0d1117"/>
  <g transform="translate(%.1f, %.1f)" filter="url(#shadow)">
    <!-- Window Body -->
    <rect width="%.1f" height="%.1f" rx="10" ry="10" fill="#1a1b26" stroke="#292e42" stroke-width="1"/>
    <!-- Title Bar -->
    <path d="M 0,10 A 10,10 0 0 1 10,0 L %.1f,0 A 10,10 0 0 1 %.1f,10 L %.1f,%.1f L 0,%.1f Z" fill="#16161e"/>
    <!-- Window Controls -->
    <circle cx="20" cy="18" r="6" fill="#f7768e"/>
    <circle cx="38" cy="18" r="6" fill="#e0af68"/>
    <circle cx="56" cy="18" r="6" fill="#9ece6a"/>
    <!-- Window Title -->
    <text x="%.1f" y="22" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif" font-size="12" fill="#7aa2f7" font-weight="600" text-anchor="middle">%s</text>
`, viewBoxW, viewBoxH, viewBoxW, viewBoxH, originX, originY, windowW, windowH, windowW-10, windowW, windowW, padTop, padTop, windowW/2, html.EscapeString(title)))

	// 1. Render background rects
	for y := 0; y < heightRows; y++ {
		line := buf.Line(y)
		var bgStart int = -1
		var curBg string = ""

		flushBg := func(endX int) {
			if bgStart >= 0 && curBg != "" && curBg != "#1a1b26" && curBg != "#16161e" {
				rw := float64(endX-bgStart) * charW
				rx := padX + float64(bgStart)*charW
				ry := padTop + float64(y)*charH
				sb.WriteString(fmt.Sprintf(`    <rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`+"\n", rx, ry, rw, charH, curBg))
			}
			bgStart = -1
			curBg = ""
		}

		for x := 0; x < widthCols && x < len(line); x++ {
			c := line[x]
			if c == nil || c.Style.Bg == nil {
				flushBg(x)
				continue
			}
			hex := hexColor(c.Style.Bg, "")
			if hex == curBg {
				continue
			}
			flushBg(x)
			if hex != "" {
				bgStart = x
				curBg = hex
			}
		}
		flushBg(len(line))
	}

	// 2. Render text spans line by line with explicit column alignment and lengthAdjust
	for y := 0; y < heightRows; y++ {
		line := buf.Line(y)
		textY := padTop + float64(y)*charH + 13.5 // baseline
		sb.WriteString(fmt.Sprintf(`    <text class="mono" y="%.1f" xml:space="preserve">`, textY))

		type runSpan struct {
			startCol int
			numCols  int
			text     string
			fg       string
			bold     bool
		}
		var runs []runSpan
		var curRun runSpan

		for x := 0; x < widthCols && x < len(line); x++ {
			c := line[x]
			r := ' '
			fg := "#c0caf5" // default clear bright Tokyo Night text
			bold := false
			w := 1
			if c != nil {
				if c.Rune != 0 {
					r = c.Rune
				}
				if c.Width > 0 {
					w = c.Width
				}
				if c.Style.Fg != nil {
					fg = hexColor(c.Style.Fg, "#c0caf5")
				}
				if c.Style.Attrs&cellbuf.BoldAttr != 0 {
					bold = true
				}
			}

			if curRun.text == "" {
				curRun = runSpan{startCol: x, numCols: w, text: string(r), fg: fg, bold: bold}
			} else if curRun.fg == fg && curRun.bold == bold && (curRun.startCol+curRun.numCols == x) {
				curRun.text += string(r)
				curRun.numCols += w
			} else {
				runs = append(runs, curRun)
				curRun = runSpan{startCol: x, numCols: w, text: string(r), fg: fg, bold: bold}
			}
		}
		if curRun.text != "" {
			runs = append(runs, curRun)
		}

		for _, r := range runs {
			if strings.TrimSpace(r.text) == "" {
				continue // spaces do not need glyphs; background rects handle highlights
			}
			boldAttr := ""
			if r.bold {
				boldAttr = ` font-weight="bold"`
			}
			xCoord := padX + float64(r.startCol)*charW
			spanW := float64(r.numCols) * charW
			sb.WriteString(fmt.Sprintf(`<tspan x="%.1f" textLength="%.1f" lengthAdjust="spacing" fill="%s"%s>%s</tspan>`,
				xCoord, spanW, r.fg, boldAttr, html.EscapeString(r.text)))
		}
		sb.WriteString("</text>\n")
	}

	sb.WriteString("  </g>\n</svg>\n")
	return sb.String()
}

func main() {
	// Force TrueColor profile so Lipgloss preserves all Tokyo-Night colors in non-tty scripts
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

	for _, s := range screens {
		m := s.setup()
		svg := RenderTerminalSVG(m.View(), cols, rows, s.title)
		pngPath := filepath.Join(assetsDir, s.name+".png")

		// Render PNG directly using native macOS /usr/bin/sips
		tmpSVG := filepath.Join(tmpDir, s.name+".svg")
		if err := os.WriteFile(tmpSVG, []byte(svg), 0o644); err != nil {
			fmt.Printf("Failed to write SVG for %s: %v\n", s.name, err)
			continue
		}

		cmd := exec.Command("/usr/bin/sips", "-s", "format", "png", tmpSVG, "--out", pngPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("Error rendering %s.png via sips: %v (output: %s)\n", s.name, err, string(out))
			continue
		}

		fmt.Printf("✓ Rendered %s.png\n", s.name)
	}

	fmt.Println("\nAll PNG screenshots generated successfully in assets/!")
}
