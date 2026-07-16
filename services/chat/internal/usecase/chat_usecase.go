package usecase

import (
	"context"
	"sort"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"social-network-system/pkg/kafka"
	"social-network-system/services/chat/internal/domain"
)

type chatUseCase struct {
	chatRepo domain.ChatRepository
	producer kafka.Producer
}

// NewChatUseCase creates a new ChatUseCase instance.
func NewChatUseCase(chatRepo domain.ChatRepository, producer kafka.Producer) domain.ChatUseCase {
	return &chatUseCase{
		chatRepo: chatRepo,
		producer: producer,
	}
}

func (u *chatUseCase) GetChatHistory(ctx context.Context, userID string, recipientID string, cursor string, size int) ([]*domain.Message, string, error) {
	// Standardize conversation ID: sorted alphabetically to be bidirectionally stable
	convID := getConversationID(userID, recipientID)

	if size <= 0 {
		size = 20 // Default size
	}
	if size > 100 {
		size = 100 // Maximum size
	}

	var cursorTime time.Time
	if cursor != "" {
		parsed, err := time.Parse(time.RFC3339Nano, cursor)
		if err != nil {
			return nil, "", err
		}
		cursorTime = parsed
	}

	messages, err := u.chatRepo.GetHistory(ctx, convID, cursorTime, size)
	if err != nil {
		return nil, "", err
	}

	nextCursor := ""
	if len(messages) > 0 {
		// MongoDB results reversed in repo to be chronological: oldest is messages[0], newest is messages[len-1]
		// The cursor needs to be the creation time of the OLDEST message in the batch for fetching older pages
		nextCursor = messages[0].CreatedAt.Format(time.RFC3339Nano)
	}

	return messages, nextCursor, nil
}

func (u *chatUseCase) PublishMessage(ctx context.Context, senderID string, req *domain.ChatSendPayload) (string, error) {
	msgID := primitive.NewObjectID()
	createdAt := time.Now()

	senderObjID, err := primitive.ObjectIDFromHex(senderID)
	if err != nil {
		return "", err
	}

	recipientObjID, err := primitive.ObjectIDFromHex(req.RecipientID)
	if err != nil {
		return "", err
	}

	// 1. Prepare event to publish to Kafka
	event := domain.ChatIncomingEvent{
		ID:          msgID.Hex(),
		SenderID:    senderObjID.Hex(),
		RecipientID: recipientObjID.Hex(),
		ContentType: req.ContentType,
		Content:     req.Content,
		MediaURL:    req.MediaURL,
		CreatedAt:   createdAt,
	}

	// 2. Publish to Kafka topic 'chat-incoming' asynchronously or synchronously.
	// The producer.Publish call is synchronous but we use it inside the websocket context.
	// We want to verify it was sent to Kafka before returning.
	err = u.producer.Publish(ctx, event.ID, event)
	if err != nil {
		return "", err
	}

	return event.ID, nil
}

// getConversationID creates a unique conversation identifier between two users
func getConversationID(userA, userB string) string {
	slice := []string{userA, userB}
	sort.Strings(slice)
	return slice[0] + "_" + slice[1]
}
