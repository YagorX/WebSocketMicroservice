-- chats
CREATE TABLE IF NOT EXISTS chats (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id     BIGINT NOT NULL,
  model_id    UUID NOT NULL REFERENCES bot_models(id),
  title       TEXT NOT NULL,
  is_deleted  BOOLEAN NOT NULL DEFAULT FALSE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at  TIMESTAMPTZ NULL
);

CREATE INDEX IF NOT EXISTS idx_chats_user_updated
  ON chats (user_id, updated_at DESC);

-- messages
CREATE TABLE IF NOT EXISTS messages (
  id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  chat_id              UUID NOT NULL REFERENCES chats(id),
  role                 TEXT NOT NULL,
  content              TEXT NOT NULL,
  model_id             UUID NULL REFERENCES bot_models(id),
  reply_to_message_id  UUID NULL REFERENCES messages(id),
  is_deleted           BOOLEAN NOT NULL DEFAULT FALSE,
  created_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  deleted_at           TIMESTAMPTZ NULL,
  CONSTRAINT role_chk CHECK (role IN ('user', 'bot'))
);

CREATE INDEX IF NOT EXISTS idx_messages_chat_created
  ON messages (chat_id, created_at);

CREATE INDEX IF NOT EXISTS idx_messages_reply_to
  ON messages (reply_to_message_id);

-- message_feedbacks
CREATE TABLE IF NOT EXISTS message_feedbacks (
  id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  message_id  UUID NOT NULL REFERENCES messages(id),
  user_id     BIGINT NOT NULL,
  model_id    UUID NOT NULL REFERENCES bot_models(id),
  is_positive BOOLEAN NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, message_id)
);

CREATE INDEX IF NOT EXISTS idx_feedback_message
  ON message_feedbacks (message_id);
