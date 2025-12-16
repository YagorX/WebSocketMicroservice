package storage

import "errors"

var (
	ErrModelNotFound   = errors.New("model not found")
	ErrChatNotFound    = errors.New("chat not found")
	ErrMessageNotFound = errors.New("message not found")
	ErrForbidden       = errors.New("forbidden")
	ErrNotBotMessage   = errors.New("not bot message")
)
