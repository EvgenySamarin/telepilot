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
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

func RenderTerminalSVG(viewStr string, widthCols, heightRows int, title string) string {
	buf := cellbuf.NewBuffer(widthCols, heightRows)
	cellbuf.SetContent(buf, viewStr)

	charW := 7.82
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

	// 2. Render text spans line by line
	for y := 0; y < heightRows; y++ {
		line := buf.Line(y)
		textY := padTop + float64(y)*charH + 13.5 // baseline
		sb.WriteString(fmt.Sprintf(`    <text class="mono" xml:space="preserve">`))

		type runSpan struct {
			col  int
			text string
			fg   string
			bold bool
		}
		var runs []runSpan
		var currentRun runSpan

		for x := 0; x < widthCols && x < len(line); x++ {
			c := line[x]
			r := ' '
			fg := "#a9b1d6"
			bold := false
			if c != nil {
				if c.Rune != 0 {
					r = c.Rune
				}
				if c.Style.Fg != nil {
					fg = hexColor(c.Style.Fg, "#a9b1d6")
				}
				if c.Style.Attrs&cellbuf.BoldAttr != 0 {
					bold = true
				}
			}

			if currentRun.text == "" {
				currentRun = runSpan{col: x, text: string(r), fg: fg, bold: bold}
			} else if currentRun.fg == fg && currentRun.bold == bold {
				currentRun.text += string(r)
			} else {
				runs = append(runs, currentRun)
				currentRun = runSpan{col: x, text: string(r), fg: fg, bold: bold}
			}
		}
		if currentRun.text != "" {
			runs = append(runs, currentRun)
		}

		for _, r := range runs {
			if strings.TrimSpace(r.text) == "" {
				continue
			}
			boldAttr := ""
			if r.bold {
				boldAttr = ` font-weight="bold"`
			}
			xCoord := padX + float64(r.col)*charW
			sb.WriteString(fmt.Sprintf(`<tspan x="%.1f" y="%.1f" fill="%s"%s>%s</tspan>`, xCoord, textY, r.fg, boldAttr, html.EscapeString(r.text)))
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

	// 1. Demo Dashboard (Tab 1: Summary)
	{
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
		m = mUpdated.(*tui.Model)
		// Select another chat so the counter is 2/5
		mUpdated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeySpace}) // select 3rd chat
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		m = mUpdated.(*tui.Model)

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — Tokyo-Night Terminal UI (Executive Summary)", "demo_dashboard", assetsDir)
	}

	// 2. Media Vault (Tab 2: Media Vault)
	{
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
		// Move cursor down to highlight the architecture diagram
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		m = mUpdated.(*tui.Model)

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — Multimodal Media Vault (Voice Transcripts, Diagrams & Specs)", "media_vault", assetsDir)
	}

	// 3. Chronological Transcript & Voice (Tab 3: Transcript)
	{
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
		m = mUpdated.(*tui.Model)

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — Chronological Chat Timeline & Inline Voice Transcripts", "transcript_view", assetsDir)
	}

	// 4. Skills & Re-Run (Tab 4: Skills)
	{
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'4'}})
		m = mUpdated.(*tui.Model)

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — Modular AI Skills Catalog (.agents/skills) & Re-Prompting", "skills_browser", assetsDir)
	}

	// 5. QR Code Login Modal (telepilot --qr or L)
	{
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
		m = mUpdated.(*tui.Model)

		// Render actual QR code Unicode art
		qrPath := filepath.Join(tmpDir, "qr.png")
		qrArt, _ := mtproto.RenderQRHalfBlocks("tg://login?token=AQAAAI_DEMO_TOKEN_TELEPILOT_42x9K", qrPath)

		// Trigger modal and set state
		m.SetQRModalForCapture(
			"[✓] Ready to scan: Open Telegram on your phone → Settings → Devices → Link Desktop Device",
			qrArt,
			qrPath,
		)

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — MTProto Connection Diagnostics & QR Code Login", "qr_login", assetsDir)
	}

	fmt.Println("\nAll screenshots generated successfully in assets/!")
}

func saveScreen(view string, cols, rows int, title, baseName, assetsDir string) {
	svg := RenderTerminalSVG(view, cols, rows, title)
	svgPath := filepath.Join(assetsDir, baseName+".svg")
	pngPath := filepath.Join(assetsDir, baseName+".png")

	if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil {
		fmt.Printf("Failed to write %s: %v\n", svgPath, err)
		return
	}

	cmd := exec.Command("sips", "-s", "format", "png", svgPath, "--out", pngPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		fmt.Printf("Error converting %s to PNG: %v (output: %s)\n", svgPath, err, string(out))
		return
	}

	fmt.Printf("✓ Created %s and %s\n", filepath.Base(svgPath), filepath.Base(pngPath))
}
