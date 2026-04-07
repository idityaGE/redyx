package notification

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"go.uber.org/zap"

	"github.com/idityaGE/redyx/internal/platform/auth"
)

// Hub manages WebSocket connections for real-time notification delivery.
// Maps user_id → active WebSocket connections.
type Hub struct {
	mu     sync.RWMutex
	conns  map[string][]*websocket.Conn
	store  *Store
	jwtVal *auth.JWTValidator
	logger *zap.Logger
}

// NewHub creates a new WebSocket hub.
func NewHub(store *Store, jwtVal *auth.JWTValidator, logger *zap.Logger) *Hub {
	return &Hub{
		conns:  make(map[string][]*websocket.Conn),
		store:  store,
		jwtVal: jwtVal,
		logger: logger,
	}
}

// register adds a WebSocket connection for the given user.
func (h *Hub) register(userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.conns[userID] = append(h.conns[userID], conn)
}

// unregister removes a WebSocket connection for the given user.
func (h *Hub) unregister(userID string, conn *websocket.Conn) {
	h.mu.Lock()
	defer h.mu.Unlock()

	conns := h.conns[userID]
	for i, c := range conns {
		if c == conn {
			h.conns[userID] = append(conns[:i], conns[i+1:]...)
			break
		}
	}
	if len(h.conns[userID]) == 0 {
		delete(h.conns, userID)
	}
}

// Send pushes a notification to all active WebSocket connections for the given user.
// If the user has no active connections, the notification is already persisted
// in PostgreSQL and will be delivered on next WebSocket connect.
func (h *Hub) Send(userID string, notification *Notification) error {
	h.mu.RLock()
	conns := make([]*websocket.Conn, len(h.conns[userID]))
	copy(conns, h.conns[userID])
	h.mu.RUnlock()

	if len(conns) == 0 {
		return fmt.Errorf("no active connections for user %s", userID)
	}

	var lastErr error
	for _, conn := range conns {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := wsjson.Write(ctx, conn, notification); err != nil {
			lastErr = err
			h.logger.Debug("failed to write to websocket",
				zap.String("user_id", userID),
				zap.Error(err),
			)
		}
		cancel()
	}

	return lastErr
}

// HandleWebSocket handles WebSocket upgrade requests for notification delivery.
// JWT token is passed as query parameter ?token=... since WebSocket doesn't
// support custom headers after the initial handshake (Pitfall 4 from research).
func (h *Hub) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	// Extract JWT token from query parameter
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "missing token parameter", http.StatusUnauthorized)
		return
	}

	// Validate JWT
	claims, err := h.jwtVal.Validate(token)
	if err != nil {
		http.Error(w, "invalid token", http.StatusUnauthorized)
		return
	}

	// Accept WebSocket connection
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		InsecureSkipVerify: true, // Allow all origins for dev; tighten in production
	})
	if err != nil {
		h.logger.Error("failed to accept websocket", zap.Error(err))
		return
	}
	defer conn.Close(websocket.StatusNormalClosure, "connection closed")

	userID := claims.UserID
	h.register(userID, conn)
	defer h.unregister(userID, conn)

	h.logger.Info("websocket connected",
		zap.String("user_id", userID),
		zap.String("username", claims.Username),
	)

	// Deliver offline notifications: send unread notifications from last 24h
	since := time.Now().Add(-24 * time.Hour)
	undelivered, err := h.store.GetUndeliveredSince(r.Context(), userID, since)
	if err != nil {
		h.logger.Error("failed to get undelivered notifications",
			zap.String("user_id", userID),
			zap.Error(err),
		)
	} else {
		for i := range undelivered {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := wsjson.Write(ctx, conn, &undelivered[i]); err != nil {
				cancel()
				h.logger.Debug("failed to deliver offline notification",
					zap.String("user_id", userID),
					zap.String("notification_id", undelivered[i].ID),
					zap.Error(err),
				)
				break
			}
			cancel()
		}
		if len(undelivered) > 0 {
			h.logger.Info("delivered offline notifications",
				zap.String("user_id", userID),
				zap.Int("count", len(undelivered)),
			)
		}
	}

	// Keep-alive read loop: read messages (client pings), break on error to trigger unregister.
	// We don't expect meaningful client messages — this just keeps the connection alive.
	for {
		_, _, err := conn.Read(r.Context())
		if err != nil {
			h.logger.Debug("websocket read error (connection closing)",
				zap.String("user_id", userID),
				zap.Error(err),
			)
			return
		}
	}
}

// ServeHTTP registers the WebSocket handler on the given mux.
func (h *Hub) ServeHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/ws/notifications", h.HandleWebSocket)
}
