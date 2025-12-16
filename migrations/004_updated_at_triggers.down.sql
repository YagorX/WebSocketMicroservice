DROP TRIGGER IF EXISTS trg_message_feedbacks_updated_at ON message_feedbacks;
DROP TRIGGER IF EXISTS trg_messages_updated_at ON messages;
DROP TRIGGER IF EXISTS trg_chats_updated_at ON chats;
DROP TRIGGER IF EXISTS trg_bot_models_updated_at ON bot_models;

DROP FUNCTION IF EXISTS set_updated_at();
