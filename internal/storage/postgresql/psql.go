package postgresql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"golang.org/x/exp/slog"

	"github.com/google/uuid"
	_ "github.com/jackc/pgx/v5/stdlib"

	models "MicroserviceWebsocket/internal/domain"
	httpAPI "MicroserviceWebsocket/internal/server/http"
)

type Storage struct {
	db  *sql.DB
	log *slog.Logger
}

func New(databaseURL string, log *slog.Logger) (*Storage, error) {
	const op = "storage.postgres.New"

	log.Info("opening postgres connection", slog.String("dsn", databaseURL))

	db, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("%s: ping failed: %w", op, err)
	}

	return &Storage{
		db:  db,
		log: log,
	}, nil
}

// --- helpers ---

func titleFromFirstMessage(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "New chat"
	}

	cut := len(s)
	for _, sep := range []string{".", "!", "?", "\n"} {
		if i := strings.Index(s, sep); i >= 0 && i < cut {
			cut = i
		}
	}
	title := strings.TrimSpace(s[:cut])
	if title == "" {
		title = s
	}
	if len(title) > 80 {
		title = title[:80]
	}
	return title
}

// --- methods used by HTTP handlers ---

func (s *Storage) CreateChat(ctx context.Context, req models.CreateChatReq) (models.CreateChatResp, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return models.CreateChatResp{}, err
	}
	defer func() { _ = tx.Rollback() }()

	// 1) model_id
	var modelID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id
		FROM bot_models
		WHERE name = $1 AND version = $2 AND is_active = TRUE
	`, req.ModelName, req.ModelVersion).Scan(&modelID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.CreateChatResp{}, httpAPI.ErrModelNotFound
		}
		return models.CreateChatResp{}, err
	}

	// 2) chat_uuid (ожидаем от фронта)
	chatUUID := strings.TrimSpace(req.ChatUUID)
	if chatUUID == "" {
		return models.CreateChatResp{}, fmt.Errorf("chat_uuid is required")
	}
	if _, err := uuid.Parse(chatUUID); err != nil {
		return models.CreateChatResp{}, fmt.Errorf("invalid chat_uuid: %w", err)
	}

	// 3) title (обрезанный текст от фронта)
	title := titleFromFirstMessage(req.Title)
	if title == "" {
		title = "New chat"
	}

	// 4) insert chat only
	_, err = tx.ExecContext(ctx, `
		INSERT INTO chats (chat_uuid, user_id, model_id, title)
		VALUES ($1::uuid, $2, $3, $4)
	`, chatUUID, req.UserID, modelID, title)
	if err != nil {
		return models.CreateChatResp{}, err
	}

	if err := tx.Commit(); err != nil {
		return models.CreateChatResp{}, err
	}

	return models.CreateChatResp{ChatUUID: chatUUID}, nil
}

func (s *Storage) ListChats(ctx context.Context, userID int64) (models.ListChatsResp, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT chat_uuid, title, model_id, updated_at
		FROM chats
		WHERE user_id = $1 AND is_deleted = FALSE
		ORDER BY updated_at DESC
	`, userID)
	if err != nil {
		return models.ListChatsResp{}, err
	}
	defer rows.Close()

	resp := models.ListChatsResp{Items: make([]models.ChatItem, 0, 16)}
	for rows.Next() {
		var it models.ChatItem
		var updated time.Time
		if err := rows.Scan(&it.ID, &it.Title, &it.ModelID, &updated); err != nil {
			return models.ListChatsResp{}, err
		}
		it.UpdatedAt = updated.UTC().Format(time.RFC3339)
		resp.Items = append(resp.Items, it)
	}
	if err := rows.Err(); err != nil {
		return models.ListChatsResp{}, err
	}

	return resp, nil
}

func (s *Storage) ListMessages(ctx context.Context, userID int64, chatUUID string) (models.ListMessagesResp, error) {
	// 1) check chat exists and belongs
	var owner int64
	var isDeleted bool
	err := s.db.QueryRowContext(ctx, `
		SELECT user_id, is_deleted
		FROM chats
		WHERE chat_uuid = $1::uuid
	`, chatUUID).Scan(&owner, &isDeleted)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.ListMessagesResp{}, httpAPI.ErrChatNotFound
		}
		return models.ListMessagesResp{}, err
	}
	if isDeleted {
		return models.ListMessagesResp{}, httpAPI.ErrChatNotFound
	}
	if owner != userID {
		return models.ListMessagesResp{}, httpAPI.ErrForbidden
	}

	// 2) list messages
	rows, err := s.db.QueryContext(ctx, `
		SELECT message_uuid, role, content, created_at, reply_to_message_id
		FROM messages
		WHERE chat_uuid = $1::uuid AND is_deleted = FALSE
		ORDER BY created_at ASC
	`, chatUUID)
	if err != nil {
		return models.ListMessagesResp{}, err
	}
	defer rows.Close()

	resp := models.ListMessagesResp{ChatID: chatUUID, Items: make([]models.MessageItem, 0, 64)}
	for rows.Next() {
		var it models.MessageItem
		var created time.Time
		var reply sql.NullString
		if err := rows.Scan(&it.ID, &it.Role, &it.Content, &created, &reply); err != nil {
			return models.ListMessagesResp{}, err
		}
		it.CreatedAt = created.UTC().Format(time.RFC3339)
		if reply.Valid {
			it.ReplyToMessageID = reply.String
		}
		resp.Items = append(resp.Items, it)
	}
	if err := rows.Err(); err != nil {
		return models.ListMessagesResp{}, err
	}

	return resp, nil
}

func (s *Storage) DeleteChat(ctx context.Context, userID int64, chatUUID string) error {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// 1) mark chat deleted (only if belongs and not deleted)
	res, err := tx.ExecContext(ctx, `
		UPDATE chats
		SET is_deleted = TRUE, deleted_at = NOW(), updated_at = NOW()
		WHERE chat_uuid = $1::uuid AND user_id = $2 AND is_deleted = FALSE
	`, chatUUID, userID)
	if err != nil {
		return err
	}
	aff, _ := res.RowsAffected()
	if aff == 0 {
		var owner int64
		var isDel bool
		err := tx.QueryRowContext(ctx, `
			SELECT user_id, is_deleted
			FROM chats
			WHERE chat_uuid = $1::uuid
		`, chatUUID).Scan(&owner, &isDel)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return httpAPI.ErrChatNotFound
			}
			return err
		}
		if owner != userID {
			return httpAPI.ErrForbidden
		}
		return httpAPI.ErrChatNotFound
	}

	// 2) mark all messages deleted
	_, err = tx.ExecContext(ctx, `
		UPDATE messages
		SET is_deleted = TRUE, deleted_at = NOW(), updated_at = NOW()
		WHERE chat_uuid = $1::uuid AND is_deleted = FALSE
	`, chatUUID)
	if err != nil {
		return err
	}

	return tx.Commit()
}

func (s *Storage) SetFeedback(ctx context.Context, messageUUID string, userID int64, isPositive bool) (models.FeedbackResp, error) {
	// 1) message exists, role=bot, not deleted, and belongs to user's chat
	var role string
	var isDeleted bool
	var chatOwner int64

	err := s.db.QueryRowContext(ctx, `
		SELECT m.role, m.is_deleted, c.user_id
		FROM messages m
		JOIN chats c ON c.chat_uuid = m.chat_uuid
		WHERE m.message_uuid = $1::uuid
	`, messageUUID).Scan(&role, &isDeleted, &chatOwner)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return models.FeedbackResp{}, httpAPI.ErrMessageNotFound
		}
		return models.FeedbackResp{}, err
	}
	if isDeleted {
		return models.FeedbackResp{}, httpAPI.ErrMessageNotFound
	}
	if chatOwner != userID {
		return models.FeedbackResp{}, httpAPI.ErrForbidden
	}
	if role != "bot" {
		return models.FeedbackResp{}, httpAPI.ErrNotBotMessage
	}

	// 2) model_id берём из чата (чтобы не доверять фронту)
	var modelID int64
	err = s.db.QueryRowContext(ctx, `
		SELECT c.model_id
		FROM messages m
		JOIN chats c ON c.chat_uuid = m.chat_uuid
		WHERE m.message_uuid = $1::uuid
	`, messageUUID).Scan(&modelID)
	if err != nil {
		return models.FeedbackResp{}, err
	}

	// 3) upsert feedback
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO message_feedbacks (message_uuid, user_id, model_id, is_positive)
		VALUES ($1::uuid, $2, $3, $4)
		ON CONFLICT (user_id, message_uuid)
		DO UPDATE SET is_positive = EXCLUDED.is_positive, updated_at = NOW()
	`, messageUUID, userID, modelID, isPositive)
	if err != nil {
		return models.FeedbackResp{}, err
	}

	return models.FeedbackResp{MessageID: messageUUID, IsPositive: isPositive}, nil
}

func (s *Storage) InsertUserMessage(ctx context.Context, chatUUID, messageUUID, content string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (message_uuid, chat_uuid, role, content)
		VALUES ($1::uuid, $2::uuid, 'user', $3)
		ON CONFLICT (message_uuid) DO NOTHING
	`, messageUUID, chatUUID, content)
	return err
}

func (s *Storage) InsertBotMessage(ctx context.Context, chatUUID, messageUUID, content, replyToUUID string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO messages (message_uuid, chat_uuid, role, content, reply_to_message_id)
		VALUES ($1::uuid, $2::uuid, 'bot', $3, $4::uuid)
	`, messageUUID, chatUUID, content, replyToUUID)
	return err
}
