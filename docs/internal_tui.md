# Package `tui`

```go
import "telepilot/internal/tui"
```

## Types

### `type AIEngine`

AIEngine defines the AI operations required by the TUI, decoupling it from the concrete CLI engine.

```go
type AIEngine interface {
	GenerateChatReply(ctx context.Context, chatTitle string, recent []storage.LiveMessage, skillName string, customNotes string) (string, error)
	TestConnection(ctx context.Context) (bool, string)
	PlanSingleShotInstruction(ctx context.Context, chatTitle string, recent []storage.LiveMessage, instruction string) ([]ai.SingleShotAction, error)
	GenerateScheduledTaskMessage(ctx context.Context, chatTitle string, recent []storage.LiveMessage, taskPrompt string) (string, error)
	ExecuteChatSkill(ctx context.Context, chat storage.ChatInfo, skillName string, customNotes string, recent []storage.LiveMessage) (storage.SkillOutputRecord, error)
}
```

### `type Model`

Model holds the complete Bubble Tea application state for TelePilot's terminal UI.

```go
type Model struct {
	// contains filtered or unexported fields
}
```

#### `func NewModel`

```go
func NewModel(cfg *config.AppConfig, aiEngine AIEngine, mtSvc TelegramClient, forceDemo bool) *Model
```

NewModel initializes the TUI Model with loaded configs, skills, chats, and runtime state.

#### `func (*Model) Init`

```go
func (m *Model) Init() tea.Cmd
```

Init starts the Bubble Tea background ticks, UI event listener, auth check, and live Telegram stream.

#### `func (*Model) SetGlobalConfigModalForCapture`

```go
func (m *Model) SetGlobalConfigModalForCapture(langIdx int)
```

SetGlobalConfigModalForCapture configures the model to display the Global Configuration modal.

#### `func (*Model) SetQRModalForCapture`

```go
func (m *Model) SetQRModalForCapture(status, qrArt, qrPNGPath string)
```

SetQRModalForCapture sets the model state to show the QR modal for captures and previews.

#### `func (*Model) SetSkillPreviewModalForCapture`

```go
func (m *Model) SetSkillPreviewModalForCapture(skillIdx int)
```

SetSkillPreviewModalForCapture configures the model to display the Skill Preview modal.

#### `func (*Model) T`

```go
func (m *Model) T(key string, args ...any) string
```

T translates a localization key into the active UI language with optional formatting arguments.

#### `func (*Model) Update`

```go
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)
```

Update handles all incoming Bubble Tea messages, timers, async results, and keyboard events.

#### `func (*Model) View`

```go
func (m *Model) View() string
```

View renders the entire responsive terminal layout.

### `type TelegramClient`

TelegramClient defines the Telegram operations required by the TUI, decoupling it from MTProto details.

```go
type TelegramClient interface {
	StartLiveStream(ctx context.Context, onNewMessage func(chatID int64, msg storage.LiveMessage))
	CheckAuthorized(ctx context.Context) (bool, string)
	ListDialogs(ctx context.Context, limit int) ([]storage.ChatInfo, error)
	FetchRecentMessages(ctx context.Context, chat storage.ChatInfo, limit int) ([]storage.LiveMessage, error)
	SendMessage(ctx context.Context, chat storage.ChatInfo, text string) (storage.LiveMessage, error)
	DownloadMessageMedia(ctx context.Context, chat storage.ChatInfo, msg storage.LiveMessage) (string, error)
	TestConnection(ctx context.Context, onUpdate func(lines []string)) (bool, []string)
	RunQRLogin(ctx context.Context, onQR func(qrArt, loginURL, pngPath string), onStatus func(status string)) (string, error)
}
```

