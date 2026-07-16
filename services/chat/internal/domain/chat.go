package domain

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Message represents a 1-1 chat message entity in MongoDB.
type Message struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ConversationID string             `bson:"conversation_id" json:"conversation_id"`
	SenderID       primitive.ObjectID `bson:"sender_id" json:"sender_id"`
	RecipientID    primitive.ObjectID `bson:"recipient_id" json:"recipient_id"`
	ContentType    string             `bson:"content_type" json:"content_type"`
	Content        string             `bson:"content" json:"content"`
	MediaURL       string             `bson:"media_url,omitempty" json:"media_url,omitempty"`
	CreatedAt      time.Time          `bson:"created_at" json:"created_at"`
}

// WSFrame represents the general structure of a WebSocket packet.
type WSFrame struct {
	Type    string      `json:"type"`              // e.g., "CHAT_SEND", "CHAT_ACK", "CHAT_RECEIVE", "PING", "PONG"
	Payload interface{} `json:"payload,omitempty"` // json.RawMessage or concrete types
}

// ChatSendPayload represents the payload when a client sends a message.
type ChatSendPayload struct {
	RecipientID string `json:"recipient_id"`
	ContentType string `json:"content_type"` // "text", "media"
	Content     string `json:"content"`
	MediaURL    string `json:"media_url,omitempty"`
	ClientMsgID string `json:"client_msg_id"`
}

// ChatAckPayload represents the confirmation sent back to the sender.
type ChatAckPayload struct {
	ClientMsgID string `json:"client_msg_id"`
	MsgID       string `json:"msg_id"`
	Status      string `json:"status"` // "sent", "failed"
}

// ChatReceivePayload represents the message pushed to the recipient.
type ChatReceivePayload struct {
	MsgID       string    `json:"msg_id"`
	SenderID    string    `json:"sender_id"`
	RecipientID string    `json:"recipient_id,omitempty"` // For internal routing
	ContentType string    `json:"content_type"`
	Content     string    `json:"content"`
	MediaURL    string    `json:"media_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChatIncomingEvent represents the event structure published to Kafka topic.
type ChatIncomingEvent struct {
	ID          string    `json:"id"`
	SenderID    string    `json:"sender_id"`
	RecipientID string    `json:"recipient_id"`
	ContentType string    `json:"content_type"`
	Content     string    `json:"content"`
	MediaURL    string    `json:"media_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ChatRepository defines the MongoDB contract for message operations.
type ChatRepository interface {
	Create(ctx context.Context, msg *Message) error
	GetHistory(ctx context.Context, conversationID string, cursor time.Time, limit int) ([]*Message, error)
}

// PresenceRepository defines the Redis contract for online status operations.
type PresenceRepository interface {
	SetOnline(ctx context.Context, userID string, nodeID string, ttl time.Duration) error
	SetOffline(ctx context.Context, userID string, nodeID string) error
	GetNode(ctx context.Context, userID string) (string, error)
}

// ChatUseCase defines the business logic contract for chat operations.
type ChatUseCase interface {
	GetChatHistory(ctx context.Context, userID string, recipientID string, cursor string, size int) ([]*Message, string, error)
	PublishMessage(ctx context.Context, senderID string, req *ChatSendPayload) (string, error)
}
