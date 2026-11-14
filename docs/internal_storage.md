# Package `storage`

```go
import "telepilot/internal/storage"
```

## Variables

DemoChats provides built-in sample Telegram dialogs used in offline demo mode.

```go
var DemoChats = []ChatInfo{
	{ChatID: -1001849201, Title: "Core Backend & Infra", ChatType: "group", UnreadCount: 14},
	{ChatID: -1001928374, Title: "Product Design & Specs", ChatType: "group", UnreadCount: 6},
	{ChatID: -1001554321, Title: "Founders Weekly Sync", ChatType: "group", UnreadCount: 3},
	{ChatID: -1001778899, Title: "AI Research & Papers", ChatType: "channel", UnreadCount: 28},
	{ChatID: 48291042, Title: "Alexey (Tech Lead)", ChatType: "dm", UnreadCount: 2},
}
```

## Functions

### `func BuildSummaryMarkdown`

```go
func BuildSummaryMarkdown(rec PeriodSummaryRecord) string
```

BuildSummaryMarkdown formats a PeriodSummaryRecord into a Markdown summary report.

### `func ChatAIConfigFilePath`

```go
func ChatAIConfigFilePath(dataDir string) string
```

ChatAIConfigFilePath returns the JSON file path for per-chat AI configuration.

### `func ChatSkillOutputsFilePath`

```go
func ChatSkillOutputsFilePath(dataDir string) string
```

ChatSkillOutputsFilePath returns the JSON file path for per-chat skill output records.

### `func DeletePeriodRun`

```go
func DeletePeriodRun(folderPath string, exportsDir string) error
```

DeletePeriodRun safely removes a period export folder after verifying it resides inside exportsDir.

### `func FavoriteChatsFilePath`

```go
func FavoriteChatsFilePath(dataDir string) string
```

FavoriteChatsFilePath returns the JSON file path for pinned favourite chats.

### `func HiddenChatsFilePath`

```go
func HiddenChatsFilePath(dataDir string) string
```

HiddenChatsFilePath returns the JSON file path for hidden chats.

### `func LoadChatAIConfigs`

```go
func LoadChatAIConfigs(dataDir string) (map[int64]ChatAIConfig, error)
```

LoadChatAIConfigs reads per-chat AI configurations from disk keyed by chat ID.

### `func LoadChatSkillOutputs`

```go
func LoadChatSkillOutputs(dataDir string) (map[int64]SkillOutputRecord, error)
```

LoadChatSkillOutputs reads per-chat skill output records from disk keyed by chat ID.

### `func SaveChatAIConfigs`

```go
func SaveChatAIConfigs(dataDir string, cfgs map[int64]ChatAIConfig) error
```

SaveChatAIConfigs persists the per-chat AI configuration map as indented JSON.

### `func SaveChatSkillOutputs`

```go
func SaveChatSkillOutputs(dataDir string, outputs map[int64]SkillOutputRecord) error
```

SaveChatSkillOutputs persists the per-chat skill output records map as indented JSON.

### `func SaveFavoriteChats`

```go
func SaveFavoriteChats(dataDir string, favs []FavoriteChatRecord) error
```

SaveFavoriteChats persists the list of favourite chat records as indented JSON.

### `func SaveHiddenChats`

```go
func SaveHiddenChats(dataDir string, hidden []HiddenChatRecord) error
```

SaveHiddenChats persists the list of hidden chat records as indented JSON.

### `func SavePeriodRun`

```go
func SavePeriodRun(rec PeriodSummaryRecord) error
```

SavePeriodRun writes a period summary record's metadata JSON, summary markdown, and media index to disk.

### `func SaveScheduledTasks`

```go
func SaveScheduledTasks(dataDir string, tasks []ScheduledTask) error
```

SaveScheduledTasks persists the slice of scheduled tasks as indented JSON.

### `func ScheduledTasksFilePath`

```go
func ScheduledTasksFilePath(dataDir string) string
```

ScheduledTasksFilePath returns the JSON file path for persisted scheduled tasks.

## Types

### `type ChatAIConfig`

ChatAIConfig stores per-chat AI skill, extra notes, auto-reply, and send mode configuration in data/chat_ai_config.json.
By default no chat has AI enabled unless explicitly configured in Tab 2.

```go
type ChatAIConfig struct {
	ChatID               int64  `json:"chat_id"`
	ChatTitle            string `json:"chat_title"`
	Enabled              bool   `json:"enabled"`
	Skill                string `json:"skill"`
	Notes                string `json:"notes"`
	PeriodSpec           string `json:"period_spec"`
	AutoReply            bool   `json:"auto_reply"` // when true, AI automatically replies to incoming messages in this chat
	SendMode             string `json:"send_mode"`  // "draft" (confirm with Enter in compose box) or "auto_send" (send immediately)
	LastAutoRepliedMsgID int    `json:"last_auto_replied_msg_id,omitempty"`
	UpdatedAt            string `json:"updated_at"`
}
```

### `type ChatInfo`

ChatInfo represents metadata for a Telegram dialog (group, channel, or direct message).

```go
type ChatInfo struct {
	ChatID      int64  `json:"chat_id"`
	AccessHash  int64  `json:"access_hash"`
	Title       string `json:"title"`
	ChatType    string `json:"chat_type"` // "group" | "channel" | "dm"
	UnreadCount int    `json:"unread_count"`
	Username    string `json:"username"`
}
```

### `type FavoriteChatRecord`

FavoriteChatRecord represents a user-pinned / favourite chat persisted across sessions.

```go
type FavoriteChatRecord struct {
	ChatID   int64  `json:"chat_id"`
	Title    string `json:"title"`
	Username string `json:"username,omitempty"`
	ChatType string `json:"chat_type,omitempty"`
	PinnedAt string `json:"pinned_at,omitempty"`
}
```

#### `func LoadFavoriteChats`

```go
func LoadFavoriteChats(dataDir string) ([]FavoriteChatRecord, error)
```

LoadFavoriteChats reads persisted favourite chat records from disk.

### `type HiddenChatRecord`

HiddenChatRecord represents a user-hidden chat excluded from the chat list.

```go
type HiddenChatRecord struct {
	ChatID   int64  `json:"chat_id"`
	Title    string `json:"title"`
	Username string `json:"username,omitempty"`
	ChatType string `json:"chat_type,omitempty"`
	HiddenAt string `json:"hidden_at,omitempty"`
}
```

#### `func LoadHiddenChats`

```go
func LoadHiddenChats(dataDir string) ([]HiddenChatRecord, error)
```

LoadHiddenChats reads persisted hidden chat records from disk.

### `type LiveMessage`

LiveMessage represents a single message in Tab 1 (Live Chat).

```go
type LiveMessage struct {
	MessageID     int    `json:"message_id"`
	ChatID        int64  `json:"chat_id"`
	Sender        string `json:"sender"`
	IsOutgoing    bool   `json:"is_outgoing"`
	Timestamp     string `json:"timestamp"`
	Text          string `json:"text"`
	MediaType     string `json:"media_type,omitempty"`      // "", "voice", "image", "document"
	MediaFileName string `json:"media_file_name,omitempty"` // display file name
	MediaSize     int64  `json:"media_size,omitempty"`
	DurationSec   int    `json:"duration_sec,omitempty"`
	CachedPath    string `json:"cached_path,omitempty"` // populated once downloaded on-demand via 'v'
}
```

#### `func DemoLiveMessages`

```go
func DemoLiveMessages(chat ChatInfo) []LiveMessage
```

DemoLiveMessages returns built-in demo messages for a chat when running in offline demo mode.

### `type MediaItemRecord`

MediaItemRecord represents a downloaded voice note, image, or document extracted during a period export.

```go
type MediaItemRecord struct {
	MessageID     int    `json:"message_id"`
	MediaType     string `json:"media_type"` // "voice" | "image" | "document"
	FileName      string `json:"file_name"`
	FilePath      string `json:"file_path"`
	Sender        string `json:"sender"`
	Timestamp     string `json:"timestamp"`
	DurationSec   int    `json:"duration_sec"`
	FileSizeBytes int64  `json:"file_size_bytes"`
	Caption       string `json:"caption"`
	AIText        string `json:"ai_text"`
	DeepLink      string `json:"deep_link"`
	Skill         string `json:"skill,omitempty"`      // media skill that produced AIText
	SkillHash     string `json:"skill_hash,omitempty"` // hash of that skill's SKILL.md at analysis time
}
```

### `type MessageRecord`

MessageRecord represents a single archived Telegram message with optional media analysis and deep link.

```go
type MessageRecord struct {
	MessageID       int    `json:"message_id"`
	Sender          string `json:"sender"`
	Timestamp       string `json:"timestamp"`
	Text            string `json:"text"`
	ReplyToMsgID    int    `json:"reply_to_msg_id,omitempty"`
	MediaType       string `json:"media_type,omitempty"`
	MediaFile       string `json:"media_file,omitempty"`
	VoiceTranscript string `json:"voice_transcript,omitempty"`
	MediaAnalysis   string `json:"media_analysis,omitempty"`
	DeepLink        string `json:"deep_link"`
}
```

### `type PeriodSummaryRecord`

PeriodSummaryRecord holds metadata, counts, media items, and generated markdown for a chat period summary run.

```go
type PeriodSummaryRecord struct {
	FolderPath         string            `json:"folder_path"`
	ChatID             int64             `json:"chat_id"`
	ChatTitle          string            `json:"chat_title"`
	PeriodSpec         string            `json:"period_spec"`
	PeriodStart        string            `json:"period_start"`
	PeriodEnd          string            `json:"period_end"`
	Prompt             string            `json:"prompt"` // extra notes applied on top of the skill
	Skill              string            `json:"skill,omitempty"`
	SkillHash          string            `json:"skill_hash,omitempty"`
	Status             string            `json:"status"`
	ModelUsed          string            `json:"model_used"`
	TotalMessages      int               `json:"total_messages"`
	VoiceCount         int               `json:"voice_count"`
	ImageCount         int               `json:"image_count"`
	DocumentCount      int               `json:"document_count"`
	ExecutiveSummary   string            `json:"executive_summary"`
	KeyDecisions       string            `json:"key_decisions"`
	VoiceMediaInsights string            `json:"voice_media_insights"`
	ActionItems        string            `json:"action_items"`
	CreatedAt          string            `json:"created_at"`
	MediaItems         []MediaItemRecord `json:"media_items"`
	Messages           []MessageRecord   `json:"messages"`
}
```

#### `func DiscoverAllPeriodFolders`

```go
func DiscoverAllPeriodFolders(exportsDir string) []PeriodSummaryRecord
```

DiscoverAllPeriodFolders scans exportsDir and returns all valid period summary records sorted newest first.

#### `func DiscoverChatPeriodFolders`

```go
func DiscoverChatPeriodFolders(exportsDir string, chat ChatInfo) []PeriodSummaryRecord
```

DiscoverChatPeriodFolders returns all saved period summary records belonging to the given chat.

#### `func EnsureDemoPeriodFolders`

```go
func EnsureDemoPeriodFolders(cfg *config.AppConfig) []PeriodSummaryRecord
```

EnsureDemoPeriodFolders seeds sample period export folders if exportsDir is empty and returns all discovered records.

#### `func LoadPeriodRun`

```go
func LoadPeriodRun(folderPath string) (*PeriodSummaryRecord, error)
```

LoadPeriodRun reads a PeriodSummaryRecord from a period export folder's metadata.json.

### `type ScheduledTask`

ScheduledTask represents a delayed or repeating AI task scheduled for a Telegram chat.

```go
type ScheduledTask struct {
	ID          string     `json:"id"`
	ChatID      int64      `json:"chat_id"`
	ChatTitle   string     `json:"chat_title"`
	TaskPrompt  string     `json:"task_prompt"`
	DueAt       time.Time  `json:"due_at"`
	IntervalStr string     `json:"interval,omitempty"`   // e.g. "30m", "2h", "24h", "daily"
	MaxRuns     int        `json:"max_runs,omitempty"`   // maximum executions (0 = unlimited until cancelled)
	RunsCount   int        `json:"runs_count,omitempty"` // number of times executed
	Until       *time.Time `json:"until,omitempty"`      // stop after this wall-clock timestamp
	CreatedAt   time.Time  `json:"created_at"`
}
```

#### `func LoadScheduledTasks`

```go
func LoadScheduledTasks(dataDir string) ([]ScheduledTask, error)
```

LoadScheduledTasks reads persisted scheduled tasks from disk, returning an empty slice if none exist.

### `type SkillOutputRecord`

SkillOutputRecord stores the free-form result of running an AI skill on a chat (displayed in Tab 3: AI Output).

```go
type SkillOutputRecord struct {
	ChatID       int64  `json:"chat_id"`
	ChatTitle    string `json:"chat_title"`
	Skill        string `json:"skill"`
	SkillHash    string `json:"skill_hash,omitempty"`
	SkillDesc    string `json:"skill_desc,omitempty"`
	Notes        string `json:"notes,omitempty"`
	ModelUsed    string `json:"model_used"`
	MessageCount int    `json:"message_count"`
	Output       string `json:"output"`
	OutputPath   string `json:"output_path,omitempty"`
	CreatedAt    string `json:"created_at"`
}
```

#### `func DemoSkillOutput`

```go
func DemoSkillOutput(chat ChatInfo, lang string) SkillOutputRecord
```

DemoSkillOutput constructs a sample SkillOutputRecord for a chat in the requested language.

