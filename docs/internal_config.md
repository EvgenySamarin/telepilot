# Package `config`

```go
import "telepilot/internal/config"
```

## Constants

DefaultProjectDir is the fallback project root directory used when TELEPILOT_HOME is unset.

```go
const DefaultProjectDir = "/Users/ey.samarin/coding/telegram-ai-client"
```

DefaultSkillName is the summary skill used when DEFAULT_SKILL is not set.

```go
const DefaultSkillName = "period-summary"
```

LegacyDefaultPrompt is the built-in DEFAULT_PROMPT used before skills existed.
Its content now lives in the "period-summary" skill, so it is ignored as notes.

```go
const LegacyDefaultPrompt = "Provide a comprehensive executive summary of this chat period. Account for all transcribed voice messages, downloaded images, and documents. Group into: 1) Executive Summary, 2) Key Decisions & Agreements, 3) Voice & Media Insights (referencing specific files and voice notes), and 4) Action Items & Deadlines."
```

## Functions

### `func NormalizeNotes`

```go
func NormalizeNotes(s string) string
```

NormalizeNotes drops the legacy built-in prompt so it is not duplicated on top of a skill.

### `func ParsePeriod`

```go
func ParsePeriod(spec string) (time.Time, time.Time, string)
```

ParsePeriod parses a relative window (e.g. 1w, 1m) or explicit date range (YYYY-MM-DD..YYYY-MM-DD) into UTC start/end bounds and a display label.

### `func ResolveProjectRoot`

```go
func ResolveProjectRoot() string
```

ResolveProjectRoot determines the project root directory from TELEPILOT_HOME, DefaultProjectDir, or the current working directory.

### `func Slugify`

```go
func Slugify(value string) string
```

Slugify converts an arbitrary title or label into a filesystem-safe lowercase underscore slug.

## Types

### `type AppConfig`

AppConfig holds resolved application settings loaded from .env and environment variables.

```go
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
```

#### `func Load`

```go
func Load() *AppConfig
```

Load reads .env and environment variables to construct and initialize an AppConfig.

#### `func (*AppConfig) ActiveModelLabel`

```go
func (c *AppConfig) ActiveModelLabel() string
```

ActiveModelLabel returns a human-readable label for the currently active AI CLI and model.

#### `func (*AppConfig) AgyCLIBin`

```go
func (c *AppConfig) AgyCLIBin() string
```

AgyCLIBin resolves the path to the agy CLI executable from configuration, PATH, or standard install locations.

#### `func (*AppConfig) GetPeriodFolder`

```go
func (c *AppConfig) GetPeriodFolder(chatTitle string, start, end time.Time) string
```

GetPeriodFolder returns (and creates) the export directory for a chat and date range, including media subdirectories.

#### `func (*AppConfig) HasTelegramCredentials`

```go
func (c *AppConfig) HasTelegramCredentials() bool
```

HasTelegramCredentials reports whether both Telegram API ID and API hash are configured.

#### `func (*AppConfig) IsRussian`

```go
func (c *AppConfig) IsRussian() bool
```

IsRussian reports whether the current configured language is Russian.

#### `func (*AppConfig) SaveAIConfig`

```go
func (c *AppConfig) SaveAIConfig(agyBinPath, agyModel string) error
```

SaveAIConfig updates the AGY CLI binary path and model preset and persists them to .env.

#### `func (*AppConfig) SaveCredentials`

```go
func (c *AppConfig) SaveCredentials(apiID int, apiHash, serverIP string, serverPort int, agyModel string) error
```

SaveCredentials updates Telegram API credentials, server endpoint, and AI model in memory and persists them to .env.

#### `func (*AppConfig) SaveLanguage`

```go
func (c *AppConfig) SaveLanguage(lang string) error
```

SaveLanguage updates and persists the preferred UI & skills language ("en" or "ru").

#### `func (*AppConfig) SaveSkillSettings`

```go
func (c *AppConfig) SaveSkillSettings(skillName, notes string) error
```

SaveSkillSettings persists the active summary skill and the extra run notes to .env.

#### `func (*AppConfig) SessionFilePath`

```go
func (c *AppConfig) SessionFilePath() string
```

SessionFilePath returns the MTProto session JSON path scoped to the configured Telegram server IP.

#### `func (*AppConfig) SkillsDir`

```go
func (c *AppConfig) SkillsDir() string
```

SkillsDir is where skills are stored (<project>/.agents/skills).

