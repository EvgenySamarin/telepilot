# Package `ai`

```go
import "telepilot/internal/ai"
```

## Constants

PendingText marks media that still needs AI analysis.

```go
const PendingText = "Requires AI transcription"
```

## Functions

### `func IsPending`

```go
func IsPending(text string) bool
```

IsPending reports whether an AI text is a placeholder rather than a real result.

### `func ParseScheduleDuration`

```go
func ParseScheduleDuration(s string) (time.Duration, error)
```

ParseScheduleDuration parses user- and agent-friendly duration strings:
e.g. "30s", "15m", "2h", "1d", "3d", "hourly", "daily", "weekly".

### `func ParseScheduleWallClock`

```go
func ParseScheduleWallClock(atStr string, now time.Time) (time.Time, error)
```

ParseScheduleWallClock parses wall-clock time strings (e.g. "09:00", "9:00", "14:30")
and returns the target time. If the time has already passed today, it targets tomorrow.

## Types

### `type Engine`

Engine wraps the Antigravity (agy) CLI to execute AI skill prompts, auto-replies,
single-shot instruction planning, and scheduled task generation.

```go
type Engine struct {
	// contains filtered or unexported fields
}
```

#### `func NewEngine`

```go
func NewEngine(cfg *config.AppConfig) *Engine
```

NewEngine constructs an AI Engine configured with the given AppConfig.

#### `func (*Engine) AnalyzeMedia`

```go
func (e *Engine) AnalyzeMedia(ctx context.Context, mediaType, filePath, caption string, force bool) MediaResult
```

AnalyzeMedia analyzes any media file using the media skill routed by its kind.
When force is false, a valid cached voice transcript sidecar is reused.

#### `func (*Engine) AnalyzeMediaItem`

```go
func (e *Engine) AnalyzeMediaItem(ctx context.Context, it storage.MediaItemRecord, caption string, force bool) (MediaResult, bool)
```

AnalyzeMediaItem re-analyzes a stored media item, skipping it when it was already
analyzed successfully by the exact same skill version (unless force is set).

#### `func (*Engine) ExecuteChatSkill`

```go
func (e *Engine) ExecuteChatSkill(
	ctx context.Context,
	chat storage.ChatInfo,
	skillName string,
	notes string,
	messages []storage.LiveMessage,
) (storage.SkillOutputRecord, error)
```

ExecuteChatSkill runs the selected skill directly on the chat's messages without any rigid 4-card JSON schema,
returning the free-form AI output to display in Tab 3 (AI Output).

#### `func (*Engine) GenerateChatReply`

```go
func (e *Engine) GenerateChatReply(ctx context.Context, chatTitle string, recent []storage.LiveMessage, skillName string, customNotes string) (string, error)
```

GenerateChatReply generates a conversational reply for a live Telegram chat using recent messages,
the chat's assigned skill, and per-chat custom AI instructions.

#### `func (*Engine) GeneratePeriodSummary`

```go
func (e *Engine) GeneratePeriodSummary(
	ctx context.Context,
	chatTitle string,
	periodLabel string,
	skillName string,
	notes string,
	messages []storage.MessageRecord,
	mediaItems []storage.MediaItemRecord,
) SummaryResult
```

GeneratePeriodSummary summarizes a period using the named summary skill plus optional notes.
The JSON output contract is enforced here, so skills only control the content.

#### `func (*Engine) GenerateScheduledTaskMessage`

```go
func (e *Engine) GenerateScheduledTaskMessage(ctx context.Context, chatTitle string, recent []storage.LiveMessage, taskPrompt string) (string, error)
```

GenerateScheduledTaskMessage generates a context-aware message to send to a Telegram chat
when a background scheduled task or reminder is triggered.

#### `func (*Engine) MediaSkillFor`

```go
func (e *Engine) MediaSkillFor(mediaType, filePath string) (skills.Skill, string)
```

MediaSkillFor returns the media skill that handles the given stored media item.

#### `func (*Engine) PlanSingleShotInstruction`

```go
func (e *Engine) PlanSingleShotInstruction(ctx context.Context, chatTitle string, recent []storage.LiveMessage, instruction string) ([]SingleShotAction, error)
```

PlanSingleShotInstruction executes a one-off, standalone AI instruction for the selected chat
(unrelated to summarization or skills) and returns one or more messages (with optional delays) to send.

#### `func (*Engine) Skills`

```go
func (e *Engine) Skills() ([]skills.Skill, []error)
```

Skills loads all skills from disk. It reads fresh every time so edits made in the
user's editor are picked up without restarting the app.

#### `func (*Engine) SummarySkill`

```go
func (e *Engine) SummarySkill(name string) skills.Skill
```

SummarySkill resolves a summary skill by name, falling back to DEFAULT_SKILL,
then to the first summary skill on disk, then to a built-in one.

#### `func (*Engine) TestConnection`

```go
func (e *Engine) TestConnection(ctx context.Context) (bool, string)
```

TestConnection verifies that the agy CLI binary is available and responds to a minimal health prompt.

### `type MediaResult`

MediaResult is the outcome of analyzing one media file with a media skill.

```go
type MediaResult struct {
	Text      string
	MediaKind string
	Skill     string
	SkillHash string
}
```

### `type ScheduleDirective`

ScheduleDirective represents an extracted schedule instruction from an AI agent's response.

```go
type ScheduleDirective struct {
	InStr      string        `json:"in,omitempty"`
	AtStr      string        `json:"at,omitempty"`
	RepeatStr  string        `json:"repeat,omitempty"`
	MaxRuns    int           `json:"max_runs,omitempty"`
	Until      *time.Time    `json:"until,omitempty"`
	TaskPrompt string        `json:"task_prompt"`
	DueAt      time.Time     `json:"due_at"`
	Interval   time.Duration `json:"interval,omitempty"`
}
```

#### `func ExtractScheduleDirective`

```go
func ExtractScheduleDirective(rawText string, now time.Time) (string, *ScheduleDirective, bool)
```

ExtractScheduleDirective inspects the raw text produced by an AI agent for a <schedule> tag.
If found, it parses the delay, interval, and inner task prompt, and returns clean text
(with the schedule tag stripped) ready for user display or chat delivery.

### `type SingleShotAction`

SingleShotAction represents one outgoing Telegram message planned by a single-shot AI instruction.

```go
type SingleShotAction struct {
	Text         string `json:"text"`
	DelaySeconds int    `json:"delay_seconds"`
}
```

### `type SummaryResult`

SummaryResult is the outcome of summarizing a period with a summary skill.

```go
type SummaryResult struct {
	ExecutiveSummary   string
	KeyDecisions       string
	VoiceMediaInsights string
	ActionItems        string
	Skill              string
	SkillHash          string
}
```

