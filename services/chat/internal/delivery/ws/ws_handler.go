package ws

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
	"social-network-system/pkg/jwtutil"
	"social-network-system/services/chat/config"
	"social-network-system/services/chat/internal/domain"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow all origins for testing
	},
}

// WSHandler manages WebSocket connections, authentication, and real-time routing.
type WSHandler struct {
	nodeID       string
	cfg          *config.Config
	chatUseCase  domain.ChatUseCase
	presenceRepo domain.PresenceRepository
	tokenManager jwtutil.TokenManager
	redisClient  *redis.Client
	conns        sync.Map // maps userID (string) -> *websocket.Conn
}

// NewWSHandler creates a new WSHandler and registers its Redis Pub/Sub consumer.
func NewWSHandler(
	cfg *config.Config,
	chatUseCase domain.ChatUseCase,
	presenceRepo domain.PresenceRepository,
	tokenManager jwtutil.TokenManager,
	redisClient *redis.Client,
) *WSHandler {
	nodeID := "node_" + uuid.New().String()
	h := &WSHandler{
		nodeID:       nodeID,
		cfg:          cfg,
		chatUseCase:  chatUseCase,
		presenceRepo: presenceRepo,
		tokenManager: tokenManager,
		redisClient:  redisClient,
	}

	// Start Redis Pub/Sub subscriber loop for this node in the background
	go h.subscribeToNodeChannel()

	return h
}

// NodeID returns the UUID of this WebSocket node.
func (h *WSHandler) NodeID() string {
	return h.nodeID
}

// HandleWS handles WebSocket handshake and read loop.
func (h *WSHandler) HandleWS(c *gin.Context) {
	// 1. Authenticate token (Check query param first, then fallback to Authorization header)
	tokenStr := c.Query("token")
	if tokenStr == "" {
		// Fallback to Bearer token in Header
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenStr = authHeader[7:]
		}
	}

	if tokenStr == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication token is required"})
		return
	}

	claims, err := h.tokenManager.VerifyAccessToken(tokenStr)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid or expired token"})
		return
	}

	userID := claims.Subject

	// 2. Upgrade to WebSocket connection
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("Failed to upgrade websocket: %v", err)
		return
	}

	// Store connection
	h.conns.Store(userID, conn)
	log.Printf("User %s connected to node %s", userID, h.nodeID)

	// Set presence to online
	ctx := context.Background()
	if err := h.presenceRepo.SetOnline(ctx, userID, h.nodeID, 60*time.Second); err != nil {
		log.Printf("Failed to set presence for user %s: %v", userID, err)
	}

	// Make sure we clean up on disconnect
	defer func() {
		conn.Close()
		h.conns.Delete(userID)
		log.Printf("User %s disconnected from node %s", userID, h.nodeID)
		_ = h.presenceRepo.SetOffline(ctx, userID, h.nodeID)
	}()

	// 3. Read loop
	for {
		_, msgBytes, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("Websocket read error for user %s: %v", userID, err)
			}
			break
		}

		var frame domain.WSFrame
		if err := json.Unmarshal(msgBytes, &frame); err != nil {
			log.Printf("Invalid frame format from user %s: %v", userID, err)
			continue
		}

		switch frame.Type {
		case "PING":
			// Update presence TTL
			_ = h.presenceRepo.SetOnline(ctx, userID, h.nodeID, 60*time.Second)
			_ = conn.WriteJSON(domain.WSFrame{Type: "PONG"})

		case "CHAT_SEND":
			payloadBytes, err := json.Marshal(frame.Payload)
			if err != nil {
				continue
			}

			var payload domain.ChatSendPayload
			if err := json.Unmarshal(payloadBytes, &payload); err != nil {
				log.Printf("Invalid CHAT_SEND payload: %v", err)
				continue
			}

			// Publish message to Kafka
			msgID, err := h.chatUseCase.PublishMessage(ctx, userID, &payload)
			status := "sent"
			if err != nil {
				log.Printf("Failed to publish message: %v", err)
				status = "failed"
			}

			// Send ACK back to client
			ackFrame := domain.WSFrame{
				Type: "CHAT_ACK",
				Payload: domain.ChatAckPayload{
					ClientMsgID: payload.ClientMsgID,
					MsgID:       msgID,
					Status:      status,
				},
			}
			_ = conn.WriteJSON(ackFrame)
		}
	}
}

// subscribeToNodeChannel listens to the Redis channel specific to this node
func (h *WSHandler) subscribeToNodeChannel() {
	ctx := context.Background()
	channel := fmt.Sprintf("chat_node:%s", h.nodeID)
	pubsub := h.redisClient.Subscribe(ctx, channel)
	defer pubsub.Close()

	log.Printf("Node %s is subscribing to Redis channel: %s", h.nodeID, channel)

	ch := pubsub.Channel()
	for msg := range ch {
		var payload domain.ChatReceivePayload
		if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
			log.Printf("Failed to unmarshal Redis message payload: %v", err)
			continue
		}

		// Find local connection of the recipient and write message to websocket
		if val, ok := h.conns.Load(payload.RecipientID); ok {
			conn, ok := val.(*websocket.Conn)
			if ok {
				frame := domain.WSFrame{
					Type:    "CHAT_RECEIVE",
					Payload: payload,
				}
				if err := conn.WriteJSON(frame); err != nil {
					log.Printf("Failed to write CHAT_RECEIVE to client: %v", err)
					conn.Close()
					h.conns.Delete(payload.RecipientID)
				}
			}
		}
	}
}
