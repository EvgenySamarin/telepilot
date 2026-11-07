package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// Tokyo-Night Color Palette
var (
	colBg        = lipgloss.Color("#1a1b26")
	colPanelBg   = lipgloss.Color("#16161e")
	colBorder    = lipgloss.Color("#3b4261")
	colBorderFoc = lipgloss.Color("#7aa2f7")
	colBlue      = lipgloss.Color("#7aa2f7")
	colCyan      = lipgloss.Color("#7dcfff")
	colGreen     = lipgloss.Color("#9ece6a")
	colYellow    = lipgloss.Color("#e0af68")
	colRed       = lipgloss.Color("#f7768e")
	colPurple    = lipgloss.Color("#bb9af7")
	colText      = lipgloss.Color("#c0caf5")
	colMuted     = lipgloss.Color("#565f89")
	colHighlight = lipgloss.Color("#283457")
)

// clampLinesToWidth ensures every string element is a single line whose visual width never exceeds maxW,
// preventing Lipgloss from wrapping any line and overflowing the terminal height.
func clampLinesToWidth(lines []string, maxW int) []string {
	if maxW < 1 {
		maxW = 1
	}
	out := make([]string, 0, len(lines))
	for _, raw := range lines {
		for _, sub := range strings.Split(raw, "\n") {
			if lipgloss.Width(sub) > maxW {
				sub = ansi.Truncate(sub, maxW, "")
			}
			out = append(out, sub)
		}
	}
	return out
}

// splitDateAndTime splits "2026-11-03 15:18" into ("2026-11-03", "15:18").
func splitDateAndTime(ts string) (string, string) {
	parts := strings.Fields(strings.TrimSpace(ts))
	if len(parts) >= 2 {
		tPart := parts[1]
		if len(tPart) > 5 {
			tPart = tPart[:5]
		}
		return parts[0], tPart
	}
	if len(parts) == 1 {
		return "", parts[0]
	}
	return "", "--:--"
}

// renderDateSeparator renders a centered muted date divider like " ─────────── 2026-11-03 ─────────── ".
func renderDateSeparator(dateStr string, innerW int) string {
	label := " " + dateStr + " "
	avail := innerW - 4 - lipgloss.Width(label)
	if avail < 6 {
		return lipgloss.NewStyle().Foreground(colMuted).Render("  " + dateStr)
	}
	left := avail / 2
	right := avail - left
	line := "  " + strings.Repeat("─", left) + label + strings.Repeat("─", right)
	return lipgloss.NewStyle().Foreground(colMuted).Render(line)
}

func renderInteriorCardWithBorder(title string, body string, cardWidth int, titleCol, borderCol lipgloss.Color) []string {
	innerTextW := cardWidth - 4
	if innerTextW < 10 {
		innerTextW = 10
	}

	wrappedLines := wrapTextBlock(body, innerTextW)
	var paddedLines []string
	for _, l := range wrappedLines {
		padded := padOrTruncate(l, innerTextW)
		paddedLines = append(paddedLines, renderColorizedCardLine(padded))
	}

	box := lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(borderCol).
		Padding(0, 1).
		Width(cardWidth - 2).
		Render(strings.Join(paddedLines, "\n"))

	var out []string
	if strings.TrimSpace(title) != "" {
		tLine := " " + lipgloss.NewStyle().Foreground(titleCol).Bold(true).Render(truncateSingleLine(title, cardWidth))
		out = append(out, tLine)
	}
	for _, bl := range strings.Split(box, "\n") {
		out = append(out, " "+bl)
	}
	return out
}

// renderColorizedCardLine highlights ON/ACTIVE status tokens in green and OFF/NOT CONFIGURED status tokens in red
// while preserving the exact visual width of the padded card line.
func renderColorizedCardLine(padded string) string {
	type statusTok struct {
		token string
		col   lipgloss.Color
	}
	tokens := []statusTok{
		{"[NOT CONFIGURED]", colRed},
		{"[НЕ НАСТРОЕН]", colRed},
		{"[NOT CONNECTED]", colRed},
		{"[НЕ ПОДКЛЮЧЕН]", colRed},
		{"[CONNECTED]", colGreen},
		{"[ПОДКЛЮЧЕН]", colGreen},
		{"[ACTIVE]", colGreen},
		{"[АКТИВЕН]", colGreen},
		{"[RUNNING]", colGreen},
		{"[ВЫПОЛНЯЕТСЯ]", colGreen},
		{"[OK]", colGreen},
		{"[FAIL]", colRed},
		{"[!]", colRed},
		{"[OFF]", colRed},
		{"[ВЫКЛ]", colRed},
		{"[ON]", colGreen},
		{"[ВКЛ]", colGreen},
		{" OFF ", colRed},
		{" ВЫКЛ ", colRed},
		{" ON ", colGreen},
		{" ВКЛ ", colGreen},
	}

	baseStyle := lipgloss.NewStyle().Foreground(colText)
	var out strings.Builder
	rem := padded
	for len(rem) > 0 {
		bestIdx := -1
		var bestTok statusTok
		for _, st := range tokens {
			if idx := strings.Index(rem, st.token); idx >= 0 {
				if bestIdx == -1 || idx < bestIdx {
					bestIdx = idx
					bestTok = st
				}
			}
		}
		if bestIdx < 0 {
			out.WriteString(baseStyle.Render(rem))
			break
		}
		if bestIdx > 0 {
			out.WriteString(baseStyle.Render(rem[:bestIdx]))
		}
		out.WriteString(lipgloss.NewStyle().Foreground(bestTok.col).Bold(true).Render(bestTok.token))
		rem = rem[bestIdx+len(bestTok.token):]
	}
	return out.String()
}

func formatAICaptureText(raw string, lang string) string {
	s := strings.TrimSpace(raw)
	if s == "" ||
		strings.HasPrefix(s, "Image ") ||
		strings.HasPrefix(s, "Downloaded image ") ||
		strings.HasPrefix(s, "Document ") ||
		strings.HasPrefix(s, "[Voice note ") ||
		s == "Requires AI transcription" ||
		s == "Требуется ИИ-транскрипция" {
		if strings.ToLower(lang) == "ru" {
			return "Требуется ИИ-транскрипция"
		}
		return "Requires AI transcription"
	}
	return s
}

func highlightBox1EnteredOption(body string, optIdx int) string {
	// Indented option lines in Box 1:
	// line 3: Active Skill value (optIdx 0)
	// line 5: Auto-Reply (optIdx 1)
	// line 6: Send Mode (optIdx 2)
	// line 8: Single-Shot value (optIdx 3)
	targetLineMap := map[int]int{
		0: 3,
		1: 5,
		2: 6,
		3: 8,
	}
	targetLine, ok := targetLineMap[optIdx]
	if !ok {
		return body
	}
	lines := strings.Split(body, "\n")
	if targetLine >= 0 && targetLine < len(lines) {
		trimmed := strings.TrimPrefix(lines[targetLine], "    ")
		lines[targetLine] = "  ▸ " + trimmed
	}
	return strings.Join(lines, "\n")
}

func renderInteriorCard(title string, body string, cardWidth int, titleCol lipgloss.Color) []string {
	return renderInteriorCardWithBorder(title, body, cardWidth, titleCol, colBorder)
}

func padRightVisual(s string, width int) string {
	w := lipgloss.Width(s)
	if w < width {
		return s + strings.Repeat(" ", width-w)
	}
	return s
}

func sanitizeTerminalText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\uFE0F' || r == '\uFE0E' || r == '\u200D' || (r >= 0x1F1E6 && r <= 0x1F1FF) {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}

func marqueeSingleLine(text string, width int, tick int) string {
	clean := strings.ReplaceAll(text, "\n", " ")
	if lipgloss.Width(clean) <= width {
		return clean + strings.Repeat(" ", width-lipgloss.Width(clean))
	}
	runes := []rune(clean)
	sep := []rune("   •   ")
	loop := append(append([]rune{}, runes...), sep...)
	offset := (tick / 2) % len(loop)
	doubled := append(loop, loop...)

	var out []rune
	curW := 0
	for _, r := range doubled[offset:] {
		rw := lipgloss.Width(string(r))
		if curW+rw > width {
			break
		}
		out = append(out, r)
		curW += rw
	}
	if curW < width {
		return string(out) + strings.Repeat(" ", width-curW)
	}
	return string(out)
}

func truncateSingleLine(text string, maxW int) string {
	clean := strings.ReplaceAll(text, "\n", " ")
	if lipgloss.Width(clean) <= maxW {
		return clean
	}
	runes := []rune(clean)
	if maxW <= 2 {
		return string(runes[:maxInt(0, maxW)])
	}
	for len(runes) > 0 && lipgloss.Width(string(runes))+2 > maxW {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + ".."
}

func padOrTruncate(text string, width int) string {
	t := truncateSingleLine(text, width)
	w := lipgloss.Width(t)
	if w < width {
		return t + strings.Repeat(" ", width-w)
	}
	return t
}

func wrapTextBlock(text string, width int) []string {
	if width < 8 {
		width = 8
	}
	var result []string
	paragraphs := strings.Split(text, "\n")
	for _, para := range paragraphs {
		para = strings.TrimRight(para, "\r")
		if strings.TrimSpace(para) == "" {
			result = append(result, "")
			continue
		}
		// Preserve leading indentation (e.g. "    " 4-space indented list items)
		leadSpaces := 0
		for _, ch := range para {
			if ch == ' ' {
				leadSpaces++
			} else {
				break
			}
		}
		if leadSpaces > width/2 {
			leadSpaces = 0
		}
		indentPrefix := strings.Repeat(" ", leadSpaces)
		contIndent := indentPrefix

		words := strings.Fields(para)
		if len(words) == 0 {
			result = append(result, "")
			continue
		}
		cur := indentPrefix + words[0]
		for _, w := range words[1:] {
			if lipgloss.Width(cur)+1+lipgloss.Width(w) <= width {
				cur += " " + w
			} else {
				for lipgloss.Width(cur) > width {
					r := []rune(cur)
					cut := minInt(len(r), width)
					result = append(result, string(r[:cut]))
					cur = contIndent + string(r[cut:])
				}
				result = append(result, cur)
				cur = contIndent + w
			}
		}
		for lipgloss.Width(cur) > width {
			r := []rune(cur)
			cut := minInt(len(r), width)
			result = append(result, string(r[:cut]))
			cur = contIndent + string(r[cut:])
		}
		result = append(result, cur)
	}
	if len(result) == 0 {
		return []string{""}
	}
	return result
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
