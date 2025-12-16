package http

import (
	models "MicroserviceWebsocket/internal/domain"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"golang.org/x/exp/slog"
)

var (
	ErrModelNotFound   = errors.New("model not found")
	ErrChatNotFound    = errors.New("chat not found")
	ErrMessageNotFound = errors.New("message not found")
	ErrForbidden       = errors.New("forbidden")
	ErrNotBotMessage   = errors.New("not bot message")
)

type Storage interface {
	CreateChat(ctx context.Context, req models.CreateChatReq) (models.CreateChatResp, error)
	ListChats(ctx context.Context, userID int64) (models.ListChatsResp, error)
	ListMessages(ctx context.Context, userID int64, chatID string) (models.ListMessagesResp, error)
	DeleteChat(ctx context.Context, userID int64, chatID string) error
	SetFeedback(ctx context.Context, messageID string, userID int64, isPositive bool) (models.FeedbackResp, error)
}

type API struct {
	log *slog.Logger
	svc Storage
}

func NewAPI(log *slog.Logger, svc Storage) *API {
	return &API{log: log, svc: svc}
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type apiErrorResp struct {
	Error apiError `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, code, msg string) {
	writeJSON(w, status, apiErrorResp{Error: apiError{Code: code, Message: msg}})
}

// /chats -> POST create chat, GET list chats
func (a *API) Chats(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		a.createChat(w, r)
	case http.MethodGet:
		a.listChats(w, r)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// /chats/{chat_id}/messages or /chats/{chat_id}
func (a *API) ChatByID(w http.ResponseWriter, r *http.Request) {
	// path: /chats/{id}/...
	path := strings.TrimPrefix(r.URL.Path, "/chats/")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) == 0 || parts[0] == "" {
		writeErr(w, http.StatusNotFound, "not_found", "not found")
		return
	}

	chatID := parts[0]

	// /chats/{id}/messages
	if len(parts) == 2 && parts[1] == "messages" {
		if r.Method != http.MethodGet {
			writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
			return
		}
		a.listMessages(w, r, chatID)
		return
	}

	// /chats/{id} (DELETE)
	if len(parts) == 1 && r.Method == http.MethodDelete {
		a.deleteChat(w, r, chatID)
		return
	}

	writeErr(w, http.StatusNotFound, "not_found", "not found")
}

// /messages/{message_id}/feedback
func (a *API) MessageByID(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/messages/")
	parts := strings.Split(strings.Trim(path, "/"), "/")

	if len(parts) != 2 || parts[1] != "feedback" {
		writeErr(w, http.StatusNotFound, "not_found", "not found")
		return
	}
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}

	messageID := parts[0]
	a.feedback(w, r, messageID)
}
