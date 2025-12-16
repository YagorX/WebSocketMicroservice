package http

import (
	models "MicroserviceWebsocket/internal/domain"
	"encoding/json"
	"net/http"
	"strconv"
)

func (a *API) createChat(w http.ResponseWriter, r *http.Request) {
	var req models.CreateChatReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", "invalid json body")
		return
	}
	if req.UserID <= 0 || req.ModelName == "" || req.ModelVersion == "" || req.FirstMessage == "" {
		writeErr(w, http.StatusBadRequest, "validation_error", "required fields: user_id, model_name, model_version, first_message")
		return
	}

	resp, err := a.svc.CreateChat(r.Context(), req)
	if err != nil {
		// тут ты маппишь доменные ошибки на http
		switch err {
		case ErrModelNotFound:
			writeErr(w, http.StatusNotFound, "model_not_found", "model not found")
		default:
			writeErr(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	writeJSON(w, http.StatusCreated, resp)
}

func (a *API) listChats(w http.ResponseWriter, r *http.Request) {
	userIDStr := r.URL.Query().Get("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || userID <= 0 {
		writeErr(w, http.StatusBadRequest, "validation_error", "query user_id is required and must be int")
		return
	}

	resp, err := a.svc.ListChats(r.Context(), userID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal", "internal error")
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (a *API) listMessages(w http.ResponseWriter, r *http.Request, chatID string) {
	userIDStr := r.URL.Query().Get("user_id") // лучше из токена, но пока так
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || userID <= 0 {
		writeErr(w, http.StatusBadRequest, "validation_error", "query user_id is required and must be int")
		return
	}

	resp, err := a.svc.ListMessages(r.Context(), userID, chatID)
	if err != nil {
		switch err {
		case ErrChatNotFound:
			writeErr(w, http.StatusNotFound, "chat_not_found", "chat not found")
		case ErrForbidden:
			writeErr(w, http.StatusForbidden, "forbidden", "chat does not belong to user")
		default:
			writeErr(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

func (a *API) deleteChat(w http.ResponseWriter, r *http.Request, chatID string) {
	userIDStr := r.URL.Query().Get("user_id")
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil || userID <= 0 {
		writeErr(w, http.StatusBadRequest, "validation_error", "query user_id is required and must be int")
		return
	}

	err = a.svc.DeleteChat(r.Context(), userID, chatID)
	if err != nil {
		switch err {
		case ErrChatNotFound:
			writeErr(w, http.StatusNotFound, "chat_not_found", "chat not found")
		case ErrForbidden:
			writeErr(w, http.StatusForbidden, "forbidden", "chat does not belong to user")
		default:
			writeErr(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (a *API) feedback(w http.ResponseWriter, r *http.Request, messageID string) {
	var req models.FeedbackReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "bad_json", "invalid json body")
		return
	}
	if req.UserID <= 0 {
		writeErr(w, http.StatusBadRequest, "validation_error", "user_id is required")
		return
	}

	resp, err := a.svc.SetFeedback(r.Context(), messageID, req.UserID, req.IsPositive)
	if err != nil {
		switch err {
		case ErrMessageNotFound:
			writeErr(w, http.StatusNotFound, "message_not_found", "message not found")
		case ErrNotBotMessage:
			writeErr(w, http.StatusBadRequest, "not_bot_message", "feedback allowed only for bot messages")
		case ErrForbidden:
			writeErr(w, http.StatusForbidden, "forbidden", "message does not belong to user")
		default:
			writeErr(w, http.StatusInternalServerError, "internal", "internal error")
		}
		return
	}

	writeJSON(w, http.StatusOK, resp)
}
