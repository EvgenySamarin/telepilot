/*
File: internal/domain/config/config.go
Description: Application configuration loader and persister (AppConfig), managing .env
settings, project/data/skills paths, Telegram and AGY credentials, language, and periods.
*/

package config

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const DefaultProjectDir = "/Users/ey.samarin/coding/telegram-ai-client"

type AppConfig struct {
	ProjectRoot        string
	TelegramAPIID      int
	TelegramAPIHash    string
	TelegramDCID       int
	TelegramServerIP   string
	TelegramServerPort int
	AgyCLIBinPath      string
	AgyModel           string
	DefaultPeriod      string
	DefaultSkill       string // summary skill name in .agents/skills/
	DefaultNotes       string // optional extra instructions appended to the skill on each run
	Language           string // "en" (default) or "ru"
	ExportsDir         string
	DataDir            string
}

// LegacyDefaultPrompt is the built-in DEFAULT_PROMPT used before skills existed.
// Its content now lives in the "period-summary" skill, so it is ignored as notes.
const LegacyDefaultPrompt = "Provide a comprehensive executive summary of this chat period. Account for all transcribed voice messages, downloaded images, and documents. Group into: 1) Executive Summary, 2) Key Decisions & Agreements, 3) Voice & Media Insights (referencing specific files and voice notes), and 4) Action Items & Deadlines."

// DefaultSkillName is the summary skill used when DEFAULT_SKILL is not set.
const DefaultSkillName = "period-summary"

/*
NormalizeNotes drops the legacy built-in prompt so it is not duplicated on top of a skill.
*/
func NormalizeNotes(s string) string {
	s = strings.TrimSpace(s)
	if s == LegacyDefaultPrompt {
		return ""
	}
	return s
}

/*
SkillsDir is where skills are stored (<project>/.agents/skills).
*/
func (c *AppConfig) SkillsDir() string {
	return filepath.Join(c.ProjectRoot, ".agents", "skills")
}

/*
ResolveProjectRoot determines the project root directory from TELEPILOT_HOME, DefaultProjectDir, or the current working directory.
*/
func ResolveProjectRoot() string {
	if envHome := strings.TrimSpace(os.Getenv("TELEPILOT_HOME")); envHome != "" {
		return envHome
	}
	if st, err := os.Stat(DefaultProjectDir); err == nil && st.IsDir() {
		return DefaultProjectDir
	}
	cwd, err := os.Getwd()
	if err == nil {
		return cwd
	}
	return "."
}

/*
Load reads .env and environment variables to construct and initialize an AppConfig.
*/
func Load() *AppConfig {
	root := ResolveProjectRoot()
	envMap := loadDotEnv(filepath.Join(root, ".env"))

	getVal := func(key, fallback string) string {
		if v, ok := envMap[key]; ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
		if v := strings.TrimSpace(os.Getenv(key)); v != "" {
			return v
		}
		return fallback
	}

	apiID, _ := strconv.Atoi(getVal("TELEGRAM_API_ID", ""))
	dcID, _ := strconv.Atoi(getVal("TELEGRAM_DC_ID", "2"))
	if dcID == 0 {
		dcID = 2
	}
	port, _ := strconv.Atoi(getVal("TELEGRAM_SERVER_PORT", "443"))
	if port == 0 {
		port = 443
	}

	exportsRel := getVal("EXPORTS_DIR", "exports")
	exportsDir := exportsRel
	if !filepath.IsAbs(exportsDir) {
		exportsDir = filepath.Join(root, exportsRel)
	}
	dataDir := filepath.Join(root, "data")
	_ = os.MkdirAll(exportsDir, 0o755)
	_ = os.MkdirAll(dataDir, 0o755)

	// DEFAULT_NOTES replaces the legacy DEFAULT_PROMPT (still read for backward compatibility).
	defaultNotes := NormalizeNotes(getVal("DEFAULT_NOTES", getVal("DEFAULT_PROMPT", "")))

	lang := strings.ToLower(getVal("APP_LANGUAGE", getVal("LANGUAGE", "en")))
	if lang != "ru" {
		lang = "en"
	}

	return &AppConfig{
		ProjectRoot:        root,
		TelegramAPIID:      apiID,
		TelegramAPIHash:    getVal("TELEGRAM_API_HASH", ""),
		TelegramDCID:       dcID,
		TelegramServerIP:   getVal("TELEGRAM_SERVER_IP", "149.154.167.50"),
		TelegramServerPort: port,
		AgyCLIBinPath:      getVal("AGY_CLI_BIN", ""),
		AgyModel:           getVal("AGY_MODEL", "auto"),
		DefaultPeriod:      getVal("DEFAULT_PERIOD", "1m"),
		DefaultSkill:       getVal("DEFAULT_SKILL", DefaultSkillName),
		DefaultNotes:       defaultNotes,
		Language:           lang,
		ExportsDir:         exportsDir,
		DataDir:            dataDir,
	}
}

/*
HasTelegramCredentials reports whether both Telegram API ID and API hash are configured.
*/
func (c *AppConfig) HasTelegramCredentials() bool {
	return c.TelegramAPIID > 0 && strings.TrimSpace(c.TelegramAPIHash) != ""
}

/*
AgyCLIBin resolves the path to the agy CLI executable from configuration, PATH, or standard install locations.
*/
func (c *AppConfig) AgyCLIBin() string {
	if custom := strings.TrimSpace(c.AgyCLIBinPath); custom != "" {
		if st, err := os.Stat(custom); err == nil && !st.IsDir() {
			return custom
		}
		if p, err := exec.LookPath(custom); err == nil && p != "" {
			return p
		}
	}
	if p, err := exec.LookPath("agy"); err == nil && p != "" {
		return p
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".local", "bin", "agy"),
		filepath.Join(home, ".antigravity", "bin", "agy"),
		"/opt/homebrew/bin/agy",
		"/usr/local/bin/agy",
		"/Applications/Antigravity.app/Contents/Resources/app/bin/agy",
	}
	for _, cand := range candidates {
		if st, err := os.Stat(cand); err == nil && !st.IsDir() {
			return cand
		}
	}
	return ""
}

/*
ActiveModelLabel returns a human-readable label for the currently active AI CLI and model.
*/
func (c *AppConfig) ActiveModelLabel() string {
	if c.AgyCLIBin() != "" {
		model := strings.TrimSpace(c.AgyModel)
		if model == "" {
			model = "auto"
		}
		return "agy-cli/" + model
	}
	return "agy-cli/unconfigured"
}

/*
SaveAIConfig updates the AGY CLI binary path and model preset and persists them to .env.
*/
func (c *AppConfig) SaveAIConfig(agyBinPath, agyModel string) {
	c.AgyCLIBinPath = strings.TrimSpace(agyBinPath)
	if strings.TrimSpace(agyModel) != "" {
		c.AgyModel = strings.TrimSpace(agyModel)
	} else if strings.TrimSpace(c.AgyModel) == "" {
		c.AgyModel = "auto"
	}
	c.SaveCredentials(c.TelegramAPIID, c.TelegramAPIHash, c.TelegramServerIP, c.TelegramServerPort, c.AgyModel)
}

/*
SessionFilePath returns the MTProto session JSON path scoped to the configured Telegram server IP.
*/
func (c *AppConfig) SessionFilePath() string {
	ipSlug := strings.ReplaceAll(c.TelegramServerIP, ".", "_")
	if ipSlug == "" {
		ipSlug = "149_154_167_50"
	}
	return filepath.Join(c.DataDir, fmt.Sprintf("session_%s.json", ipSlug))
}

/*
GetPeriodFolder returns (and creates) the export directory for a chat and date range, including media subdirectories.
*/
func (c *AppConfig) GetPeriodFolder(chatTitle string, start, end time.Time) string {
	chatSlug := Slugify(chatTitle)
	periodSlug := fmt.Sprintf("%s_to_%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	folder := filepath.Join(c.ExportsDir, chatSlug, periodSlug)
	_ = os.MkdirAll(filepath.Join(folder, "images"), 0o755)
	_ = os.MkdirAll(filepath.Join(folder, "documents"), 0o755)
	_ = os.MkdirAll(filepath.Join(folder, "voice"), 0o755)
	return folder
}

/*
SaveCredentials updates Telegram API credentials, server endpoint, and AI model in memory and persists them to .env.
*/
func (c *AppConfig) SaveCredentials(apiID int, apiHash, serverIP string, serverPort int, agyModel string) {
	if apiID > 0 {
		c.TelegramAPIID = apiID
	}
	if strings.TrimSpace(apiHash) != "" {
		c.TelegramAPIHash = strings.TrimSpace(apiHash)
	}
	if strings.TrimSpace(serverIP) != "" {
		c.TelegramServerIP = strings.TrimSpace(serverIP)
	}
	if serverPort > 0 {
		c.TelegramServerPort = serverPort
	}
	if strings.TrimSpace(agyModel) != "" {
		c.AgyModel = strings.TrimSpace(agyModel)
	}
	if strings.TrimSpace(c.AgyModel) == "" {
		c.AgyModel = "auto"
	}

	binToSave := strings.TrimSpace(c.AgyCLIBinPath)
	if binToSave == "" {
		binToSave = c.AgyCLIBin()
	}

	envPath := filepath.Join(c.ProjectRoot, ".env")
	langToSave := c.Language
	if langToSave != "ru" {
		langToSave = "en"
	}
	lines := []string{
		fmt.Sprintf("TELEGRAM_API_ID=%d", c.TelegramAPIID),
		fmt.Sprintf("TELEGRAM_API_HASH=%s", c.TelegramAPIHash),
		fmt.Sprintf("TELEGRAM_DC_ID=%d", c.TelegramDCID),
		fmt.Sprintf("TELEGRAM_SERVER_IP=%s", c.TelegramServerIP),
		fmt.Sprintf("TELEGRAM_SERVER_PORT=%d", c.TelegramServerPort),
		fmt.Sprintf("AGY_CLI_BIN=%s", binToSave),
		fmt.Sprintf("AGY_MODEL=%s", c.AgyModel),
		fmt.Sprintf("DEFAULT_PERIOD=%s", c.DefaultPeriod),
		fmt.Sprintf("DEFAULT_SKILL=%s", c.DefaultSkill),
		fmt.Sprintf("DEFAULT_NOTES=%s", strings.ReplaceAll(c.DefaultNotes, "\n", " ")),
		fmt.Sprintf("APP_LANGUAGE=%s", langToSave),
		"EXPORTS_DIR=exports",
	}
	_ = os.WriteFile(envPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
	_ = os.Chmod(envPath, 0o600)
}

/*
SaveLanguage updates and persists the preferred UI & skills language ("en" or "ru").
*/
func (c *AppConfig) SaveLanguage(lang string) {
	if strings.ToLower(strings.TrimSpace(lang)) == "ru" {
		c.Language = "ru"
	} else {
		c.Language = "en"
	}
	c.SaveCredentials(c.TelegramAPIID, c.TelegramAPIHash, c.TelegramServerIP, c.TelegramServerPort, c.AgyModel)
}

/*
IsRussian reports whether the current configured language is Russian.
*/
func (c *AppConfig) IsRussian() bool {
	return strings.ToLower(c.Language) == "ru"
}

/*
SaveSkillSettings persists the active summary skill and the extra run notes to .env.
*/
func (c *AppConfig) SaveSkillSettings(skillName, notes string) {
	if s := strings.TrimSpace(skillName); s != "" {
		c.DefaultSkill = s
	}
	c.DefaultNotes = strings.TrimSpace(notes)
	c.SaveCredentials(c.TelegramAPIID, c.TelegramAPIHash, c.TelegramServerIP, c.TelegramServerPort, c.AgyModel)
}

var nonWordRe = regexp.MustCompile(`[^\p{L}\p{N}\s_-]+`)
var spaceDashRe = regexp.MustCompile(`[-\s_]+`)

/*
Slugify converts an arbitrary title or label into a filesystem-safe lowercase underscore slug.
*/
func Slugify(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	v = nonWordRe.ReplaceAllString(v, "")
	v = spaceDashRe.ReplaceAllString(v, "_")
	v = strings.Trim(v, "_")
	if v == "" {
		return "chat"
	}
	return v
}

var periodRe = regexp.MustCompile(`^(\d+)\s*([dwmy])$`)

/*
ParsePeriod parses a relative window (e.g. 1w, 1m) or explicit date range (YYYY-MM-DD..YYYY-MM-DD) into UTC start/end bounds and a display label.
*/
func ParsePeriod(spec string) (time.Time, time.Time, string) {
	now := time.Now().UTC()
	s := strings.ToLower(strings.TrimSpace(spec))
	if s == "" {
		s = "1m"
	}

	if strings.Contains(s, "..") {
		parts := strings.SplitN(s, "..", 2)
		t1, err1 := time.Parse("2006-01-02", strings.TrimSpace(parts[0]))
		t2, err2 := time.Parse("2006-01-02", strings.TrimSpace(parts[1]))
		if err1 == nil && err2 == nil {
			end := t2.Add(23*time.Hour + 59*time.Minute + 59*time.Second)
			label := fmt.Sprintf("%s -> %s", t1.Format("2006-01-02"), t2.Format("2006-01-02"))
			return t1.UTC(), end.UTC(), label
		}
	}

	if m := periodRe.FindStringSubmatch(s); len(m) == 3 {
		amt, _ := strconv.Atoi(m[1])
		if amt <= 0 {
			amt = 1
		}
		unit := m[2]
		var days int
		switch unit {
		case "d":
			days = amt
		case "w":
			days = amt * 7
		case "m":
			days = amt * 30
		case "y":
			days = amt * 365
		}
		start := now.AddDate(0, 0, -days)
		label := fmt.Sprintf("%s -> %s (%d%s)", start.Format("2006-01-02"), now.Format("2006-01-02"), amt, unit)
		return start, now, label
	}

	start := now.AddDate(0, 0, -30)
	label := fmt.Sprintf("%s -> %s (1m)", start.Format("2006-01-02"), now.Format("2006-01-02"))
	return start, now, label
}

func loadDotEnv(path string) map[string]string {
	out := make(map[string]string)
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:idx])
		v := strings.TrimSpace(line[idx+1:])
		v = strings.Trim(v, `"'`)
		out[k] = v
	}
	if err := scanner.Err(); err != nil {
		// Keep whatever was parsed before the failure, but surface the problem
		// (e.g. I/O error or a line exceeding bufio.MaxScanTokenSize).
		fmt.Fprintf(os.Stderr, "warning: failed to fully read %s: %v\n", path, err)
	}
	return out
}
