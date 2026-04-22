package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/you/discord-backend/internal/middleware"
	"github.com/you/discord-backend/internal/model"
	"github.com/you/discord-backend/internal/store"
	"github.com/you/discord-backend/internal/ws"
	"golang.org/x/crypto/bcrypt"
)

type Handler struct {
	store     *store.Store
	hub       *ws.Hub
	jwtSecret string
}

func New(s *store.Store, hub *ws.Hub, jwtSecret string) *Handler {
	return &Handler{store: s, hub: hub, jwtSecret: jwtSecret}
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (h *Handler) makeToken(userID int64) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(7 * 24 * time.Hour).Unix(),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(h.jwtSecret))
}

// ── Auth ─────────────────────────────────────────────────────────────────────

func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Username == "" || body.Password == "" {
		writeErr(w, http.StatusBadRequest, "username and password required")
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(body.Password), bcrypt.DefaultCost)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}

	user, err := h.store.CreateUser(r.Context(), body.Username, string(hash))
	if err != nil {
		writeErr(w, http.StatusConflict, "username already taken")
		return
	}

	token, _ := h.makeToken(user.ID)
	writeJSON(w, http.StatusCreated, map[string]any{"user": user, "token": token})
}

func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "bad request")
		return
	}

	user, err := h.store.GetUserByUsername(r.Context(), body.Username)
	if err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(body.Password)); err != nil {
		writeErr(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, _ := h.makeToken(user.ID)
	writeJSON(w, http.StatusOK, map[string]any{"user": user, "token": token})
}

// ── Channels ─────────────────────────────────────────────────────────────────

func (h *Handler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Name == "" {
		writeErr(w, http.StatusBadRequest, "name required")
		return
	}

	ch, err := h.store.CreateChannel(r.Context(), body.Name, body.Description)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create channel")
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

func (h *Handler) ListChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := h.store.ListChannels(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not fetch channels")
		return
	}
	writeJSON(w, http.StatusOK, channels)
}

// ── Messages ─────────────────────────────────────────────────────────────────

func (h *Handler) ListMessages(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid channel id")
		return
	}

	limit := 50
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}

	msgs, err := h.store.ListMessages(r.Context(), channelID, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not fetch messages")
		return
	}
	writeJSON(w, http.StatusOK, msgs)
}

// ── WebSocket ─────────────────────────────────────────────────────────────────

func (h *Handler) WSConnect(w http.ResponseWriter, r *http.Request) {
	channelID, err := strconv.ParseInt(r.PathValue("channelID"), 10, 64)
	if err != nil {
		http.Error(w, "invalid channel id", http.StatusBadRequest)
		return
	}

	// token via query param for WS handshake
	tokenStr := r.URL.Query().Get("token")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (any, error) {
		return []byte(h.jwtSecret), nil
	})
	if err != nil || !token.Valid {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	claims := token.Claims.(jwt.MapClaims)
	userID := int64(claims["sub"].(float64))

	user, err := h.store.GetUserByUsername(r.Context(), "") // we only have id, fetch via store
	_ = user
	// just embed username in the token for simplicity
	username, _ := claims["username"].(string)
	if username == "" {
		username = "user"
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	client := ws.NewClient(h.hub, conn, channelID, userID, username)
	h.hub.Register(client)

	// notify others
	h.hub.BroadcastPresence(channelID, model.WSEvent{
		Type:    "presence",
		Payload: map[string]any{"user_id": userID, "username": username, "online": true},
	})

	go client.WritePump()
	client.ReadPump(func(c *ws.Client, raw []byte) {
		var body struct {
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &body); err != nil || body.Content == "" {
			return
		}

		msg, err := h.store.CreateMessage(r.Context(), channelID, userID, body.Content)
		if err != nil {
			return
		}

		h.hub.Broadcast(channelID, model.WSEvent{Type: "message", Payload: msg})
	})

	// disconnected
	h.hub.BroadcastPresence(channelID, model.WSEvent{
		Type:    "presence",
		Payload: map[string]any{"user_id": userID, "username": username, "online": false},
	})
}

// Routes returns a configured mux.
func (h *Handler) Routes(jwtSecret string) http.Handler {
	mux := http.NewServeMux()
	auth := middleware.Auth(jwtSecret)

	mux.HandleFunc("POST /api/register", h.Register)
	mux.HandleFunc("POST /api/login", h.Login)

	mux.Handle("GET /api/channels", auth(http.HandlerFunc(h.ListChannels)))
	mux.Handle("POST /api/channels", auth(http.HandlerFunc(h.CreateChannel)))
	mux.Handle("GET /api/channels/{channelID}/messages", auth(http.HandlerFunc(h.ListMessages)))

	// WS: token in query because browsers can't send headers during upgrade
	mux.HandleFunc("GET /ws/channels/{channelID}", h.WSConnect)

	return mux
}
