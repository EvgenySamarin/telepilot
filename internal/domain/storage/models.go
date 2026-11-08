package storage

type ChatInfo struct {
	ChatID      int64  `json:"chat_id"`
	AccessHash  int64  `json:"access_hash"`
	Title       string `json:"title"`
	ChatType    string `json:"chat_type"` // "group" | "channel" | "dm"
	UnreadCount int    `json:"unread_count"`
	Username    string `json:"username"`
}

// FavoriteChatRecord represents a user-pinned / favourite chat persisted across sessions.
type FavoriteChatRecord struct {
	ChatID   int64  `json:"chat_id"`
	Title    string `json:"title"`
	Username string `json:"username,omitempty"`
	ChatType string `json:"chat_type,omitempty"`
	PinnedAt string `json:"pinned_at,omitempty"`
}

// HiddenChatRecord represents a user-hidden chat excluded from the chat list.
type HiddenChatRecord struct {
	ChatID   int64  `json:"chat_id"`
	Title    string `json:"title"`
	Username string `json:"username,omitempty"`
	ChatType string `json:"chat_type,omitempty"`
	HiddenAt string `json:"hidden_at,omitempty"`
}

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

// ChatAIConfig stores per-chat AI skill, extra notes, auto-reply, and send mode configuration in data/chat_ai_config.json.
// By default no chat has AI enabled unless explicitly configured in Tab 2.
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

// LiveMessage represents a single message in Tab 1 (Live Chat).
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

// SkillOutputRecord stores the free-form result of running an AI skill on a chat (displayed in Tab 3: AI Output).
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
