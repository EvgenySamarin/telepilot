# Package `mtproto`

```go
import "telepilot/internal/mtproto"
```

## Functions

### `func RenderQRHalfBlocks`

```go
func RenderQRHalfBlocks(url string, pngPath string) (string, error)
```

RenderQRHalfBlocks renders a URL into compact Unicode half-block lines (2 vertical modules per row)
and also writes a crisp PNG to pngPath for macOS QuickLook preview.

## Types

### `type Service`

Service manages Telegram MTProto client connections, authentication, dialog sync,
live messaging, and historical archive exports.

```go
type Service struct {
	// contains filtered or unexported fields
}
```

#### `func NewService`

```go
func NewService(cfg *config.AppConfig, aiEngine *ai.Engine) *Service
```

NewService creates a new MTProto Service bound to the given AppConfig and AI Engine.

#### `func (*Service) CheckAuthorized`

```go
func (s *Service) CheckAuthorized(ctx context.Context) (bool, string)
```

CheckAuthorized checks whether the local session is already logged in.

#### `func (*Service) DownloadMessageMedia`

```go
func (s *Service) DownloadMessageMedia(ctx context.Context, chat storage.ChatInfo, msg storage.LiveMessage) (string, error)
```

DownloadMessageMedia downloads a message's media attachment on-demand into data/live_cache/<chat_id>/.

#### `func (*Service) FetchRecentMessages`

```go
func (s *Service) FetchRecentMessages(ctx context.Context, chat storage.ChatInfo, limit int) ([]storage.LiveMessage, error)
```

FetchRecentMessages loads the last `limit` messages (default 50) for a chat in chronological order.

#### `func (*Service) ListDialogs`

```go
func (s *Service) ListDialogs(ctx context.Context, limit int) ([]storage.ChatInfo, error)
```

ListDialogs fetches the user's top chats, groups, and channels.

#### `func (*Service) ParseChatPeriod`

```go
func (s *Service) ParseChatPeriod(
	ctx context.Context,
	chat storage.ChatInfo,
	periodSpec string,
	skillName string,
	notes string,
	onLog func(msg string),
) (*storage.PeriodSummaryRecord, error)
```

ParseChatPeriod scrapes messages & downloads media for a given chat and period, then generates AI summaries.
skillName selects the summary skill; notes are optional extra instructions for this run.

#### `func (*Service) RunQRLogin`

```go
func (s *Service) RunQRLogin(
	ctx context.Context,
	onQR func(qrArt string, loginURL string, pngPath string),
	onStatus func(status string),
) (string, error)
```

RunQRLogin performs the Telegram Phone QR Code login flow using AuthExportLoginToken.

#### `func (*Service) SendMessage`

```go
func (s *Service) SendMessage(ctx context.Context, chat storage.ChatInfo, text string) (storage.LiveMessage, error)
```

SendMessage sends a text message to the specified chat and returns the resulting LiveMessage.

#### `func (*Service) StartLiveStream`

```go
func (s *Service) StartLiveStream(ctx context.Context, onNewMessage func(chatID int64, msg storage.LiveMessage))
```

StartLiveStream runs a persistent gotd/td UpdateDispatcher to deliver incoming messages in real time.

#### `func (*Service) TestConnection`

```go
func (s *Service) TestConnection(ctx context.Context, onUpdate func(lines []string)) (bool, []string)
```

TestConnection runs the 3-step live diagnostic against the configured MTProto server.

