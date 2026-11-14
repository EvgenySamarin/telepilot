/*
File: cmd/screens/main.go
Description: Headless terminal screenshot generator that renders TUI tabs and modals
into ANSI-to-SVG/PNG assets under assets/.
*/

package main

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
	"github.com/muesli/termenv"

	"telepilot/internal/domain/ai"
	"telepilot/internal/domain/config"
	"telepilot/internal/domain/skills"
	"telepilot/internal/domain/storage"
	"telepilot/internal/network/mtproto"
	"telepilot/internal/presentation/tui"
)

func hexColor(c ansi.Color, defaultHex string) string {
	if c == nil {
		return defaultHex
	}
	r, g, b, _ := c.RGBA()
	return fmt.Sprintf("#%02x%02x%02x", r>>8, g>>8, b>>8)
}

/*
RenderTerminalSVG converts an ANSI-styled terminal view string into a self-contained SVG terminal screenshot.
*/
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
    .mono { font-family: ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace; font-size: 13px; font-variant-ligatures: none; }
  </style>
  <rect width="100%%" height="100%%" fill="#0d1117"/>
  <g transform="translate(%.1f, %.1f)" filter="url(#shadow)" font-family="ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace" font-size="13">
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
		sb.WriteString(fmt.Sprintf(`    <text class="mono" x="%.1f" y="%.1f" xml:space="preserve">`, padX, textY))

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

	favs := []storage.FavoriteChatRecord{
		{ChatID: 101, Title: "Product Design & Specs", ChatType: "channel", PinnedAt: "2026-11-08 12:00:00"},
		{ChatID: 102, Title: "Founders Weekly Sync", ChatType: "chat", PinnedAt: "2026-11-08 12:05:00"},
	}
	_ = storage.SaveFavoriteChats(cfg.DataDir, favs)
	hidden := []storage.HiddenChatRecord{
		{ChatID: 999, Title: "Crypto News & Signals (Spam)", ChatType: "channel", HiddenAt: "2026-11-08 12:10:00"},
	}
	_ = storage.SaveHiddenChats(cfg.DataDir, hidden)

	aiEng := ai.NewEngine(cfg)
	mtSvc := mtproto.NewService(cfg, aiEng)

	assetsDir := filepath.Join(cwd, "assets")
	_ = os.MkdirAll(assetsDir, 0o755)

	cols := 125
	rows := 36

	// 1. Demo Dashboard (Tab 1: Live Chat)
	{
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
		// Enable AI for the first chat via Box 3 (Skills Library) so [AI] highlight is showcased in the sidebar
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
		for i := 0; i < 3; i++ {
			mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		}
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyEnter})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyEnter})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyEsc})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'1'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
		m = mUpdated.(*tui.Model)

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — Terminal UI (Live Chat & Per-Chat AI)", "demo_dashboard", assetsDir)
	}

	// 2. Per-Chat AI & Skills (Tab 2: AI & Skills with 2-Level Box Navigation)
	{
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
		// Navigate to Box 3 (Skills Library), enter it, assign 'period-summary', then exit back and select Box 1
		for i := 0; i < 3; i++ {
			mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		}
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyEnter})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyEnter})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyEsc})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyEnter})
		m = mUpdated.(*tui.Model)

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — Per-Chat AI Instructions & 2-Level Box Navigation", "skills_browser", assetsDir)
	}

	// 3. AI Skill Output (Tab 3: AI Output)
	{
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
		m = mUpdated.(*tui.Model)

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — AI Skill Output & Execution Report in Tab 3", "skill_output", assetsDir)
		saveScreen(view, cols, rows, "telepilot — AI Skill Output & Execution Report in Tab 3", "transcript_view", assetsDir)
	}

	// 4. AI Skill Output Scrolled (Tab 3)
	{
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'l'}})
		for i := 0; i < 4; i++ {
			mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'J'}})
		}
		m = mUpdated.(*tui.Model)

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — Detailed Skill Output & Natural Language Report", "media_vault", assetsDir)
	}

	// 5. QR Code Login Modal (telepilot --qr or L)
	{
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

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — MTProto Connection Diagnostics & QR Code Login", "qr_login", assetsDir)
	}

	// 6. Global Configuration & Language Selector (Press 'C')
	{
		cfgRows := 44
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: cfgRows})
		m = mUpdated.(*tui.Model)

		m.SetGlobalConfigModalForCapture(0)

		view := m.View()
		saveScreen(view, cols, cfgRows, "telepilot — Global Configuration, Favourite & Hidden Chats (En / Ru)", "language_config", assetsDir)
	}

	// 7. Embedded Skill Preview & In-TUI Editing (Press 'v' or 'e' in Tab 2)
	{
		m := tui.NewModel(cfg, aiEng, mtSvc, true)
		mUpdated, _ := m.Update(tea.WindowSizeMsg{Width: cols, Height: rows})
		mUpdated, _ = mUpdated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'2'}})
		m = mUpdated.(*tui.Model)

		m.SetSkillPreviewModalForCapture(0)

		view := m.View()
		saveScreen(view, cols, rows, "telepilot — In-TUI Skill Preview & Editor ($EDITOR Integration)", "skill_editor_preview", assetsDir)
	}

	fmt.Println("\nAll screenshots generated successfully in assets/!")
}

func saveScreen(view string, cols, rows int, title, baseName, assetsDir string) {
	svg := RenderTerminalSVG(view, cols, rows, title)
	svgPath := filepath.Join(assetsDir, baseName+".svg")

	if err := os.WriteFile(svgPath, []byte(svg), 0o644); err != nil {
		fmt.Printf("Failed to write %s: %v\n", svgPath, err)
		return
	}

	fmt.Printf("✓ Created %s\n", filepath.Base(svgPath))
}
