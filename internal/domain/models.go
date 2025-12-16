package domain

type BatchItem struct {
	Request Request
	Future  chan *Response
}

type Request struct {
	UUID      string `json:"uuid"`
	ModelName string `json:"model_name"`
	Message   string `json:"message"`
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

// for http methods

// ---------- 2.1 POST /chats ----------

type CreateChatReq struct {
	UserID       int64  `json:"user_id"`
	ModelName    string `json:"model_name"`
	ModelVersion string `json:"model_version"`
	FirstMessage string `json:"first_message"`
}

type CreateChatResp struct {
	ChatID        string `json:"chat_id"`
	UserMessageID string `json:"user_message_id"`
}

// ---------- 2.2 GET /chats?user_id=123 ----------

type ChatItem struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	ModelID   string `json:"model_id"`
	UpdatedAt string `json:"updated_at"`
}

type ListChatsResp struct {
	Items []ChatItem `json:"items"`
}

// ---------- 2.3 GET /chats/{chat_id}/messages ----------

type MessageItem struct {
	ID               string `json:"id"`
	Role             string `json:"role"` // user|bot
	Content          string `json:"content"`
	CreatedAt        string `json:"created_at"`
	ReplyToMessageID string `json:"reply_to_message_id,omitempty"`
}

type ListMessagesResp struct {
	ChatID string        `json:"chat_id"`
	Items  []MessageItem `json:"items"`
}

// ---------- 2.5 POST /messages/{message_id}/feedback ----------

type FeedbackReq struct {
	UserID     int64 `json:"user_id"`
	IsPositive bool  `json:"is_positive"`
}

type FeedbackResp struct {
	MessageID  string `json:"message_id"`
	IsPositive bool   `json:"is_positive"`
}
