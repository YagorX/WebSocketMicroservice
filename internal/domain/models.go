package domain

type BatchItem struct {
	Request Request
	Future  chan *Response
}

type Request struct {
	UUID      string `json:"uuid"` // сообщения
	ModelName string `json:"model_name"`
	Message   string `json:"message"`
	ChatUUID  string `json:"chat_uuid"`
}

type Response struct {
	UUID      string `json:"uuid"`
	Response  string `json:"response"`
	CreatedAt string `json:"created_at"`
}

type WSPing struct {
	Type string `json:"type"` // "ping"
}
type WSPong struct {
	Type string `json:"type"` // "pong"
}

type WSBotMessage struct {
	Type            string `json:"type"`
	ChatUUID        string `json:"chat_uuid"`
	UserMessageUUID string `json:"user_message_uuid"`
	BotMessageUUID  string `json:"bot_message_uuid"`
	Response        string `json:"response"`
	CreatedAt       string `json:"created_at"`
}

// -------------------- HTTP models --------------------

// ---------- POST /chats ----------
type CreateChatReq struct {
	ChatUUID     string `json:"chat_uuid"`
	UserID       int64  `json:"user_id"`
	ModelName    string `json:"model_name"`
	ModelVersion string `json:"model_version"`
	Title        string `json:"title"`
}
type CreateChatResp struct {
	ChatUUID string `json:"chat_uuid"`
}

// ---------- GET /chats?user_id=123 ----------
type ChatItem struct {
	ID        string `json:"id"` // chat_uuid
	Title     string `json:"title"`
	ModelID   int64  `json:"model_id"` // bot_models.id (BIGINT)
	UpdatedAt string `json:"updated_at"`
}

type ListChatsResp struct {
	Items []ChatItem `json:"items"`
}

// ---------- GET /chats/{chat_id}/messages ----------
type MessageItem struct {
	ID               string `json:"id"`   // message_uuid
	Role             string `json:"role"` // user|bot
	Content          string `json:"content"`
	CreatedAt        string `json:"created_at"`
	ReplyToMessageID string `json:"reply_to_message_id,omitempty"`
}

type ListMessagesResp struct {
	ChatID string        `json:"chat_id"` // chat_uuid
	Items  []MessageItem `json:"items"`
}

// ---------- POST /messages/{message_id}/feedback ----------
type FeedbackReq struct {
	UserID     int64 `json:"user_id"`
	IsPositive bool  `json:"is_positive"`
}

type FeedbackResp struct {
	MessageID  string `json:"message_id"` // message_uuid
	IsPositive bool   `json:"is_positive"`
}
