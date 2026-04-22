package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/you/discord-backend/internal/model"
)

type Store struct {
	db *pgxpool.Pool
}

func New(connStr string) (*Store, error) {
	pool, err := pgxpool.New(context.Background(), connStr)
	if err != nil {
		return nil, fmt.Errorf("pgxpool.New: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping: %w", err)
	}
	return &Store{db: pool}, nil
}

func (s *Store) Close() { s.db.Close() }

// ── Users ────────────────────────────────────────────────────────────────────

func (s *Store) CreateUser(ctx context.Context, username, passwordHash string) (*model.User, error) {
	u := &model.User{}
	err := s.db.QueryRow(ctx,
		`INSERT INTO users (username, password_hash) VALUES ($1, $2)
		 RETURNING id, username, created_at`,
		username, passwordHash,
	).Scan(&u.ID, &u.Username, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) GetUserByUsername(ctx context.Context, username string) (*model.User, error) {
	u := &model.User{}
	err := s.db.QueryRow(ctx,
		`SELECT id, username, password_hash, created_at FROM users WHERE username = $1`,
		username,
	).Scan(&u.ID, &u.Username, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

// ── Channels ─────────────────────────────────────────────────────────────────

func (s *Store) CreateChannel(ctx context.Context, name, description string) (*model.Channel, error) {
	ch := &model.Channel{}
	err := s.db.QueryRow(ctx,
		`INSERT INTO channels (name, description) VALUES ($1, $2)
		 RETURNING id, name, description, created_at`,
		name, description,
	).Scan(&ch.ID, &ch.Name, &ch.Description, &ch.CreatedAt)
	if err != nil {
		return nil, err
	}
	return ch, nil
}

func (s *Store) ListChannels(ctx context.Context) ([]*model.Channel, error) {
	rows, err := s.db.Query(ctx, `SELECT id, name, description, created_at FROM channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var channels []*model.Channel
	for rows.Next() {
		ch := &model.Channel{}
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Description, &ch.CreatedAt); err != nil {
			return nil, err
		}
		channels = append(channels, ch)
	}
	return channels, rows.Err()
}

// ── Messages ─────────────────────────────────────────────────────────────────

func (s *Store) CreateMessage(ctx context.Context, channelID, userID int64, content string) (*model.Message, error) {
	msg := &model.Message{}
	err := s.db.QueryRow(ctx,
		`INSERT INTO messages (channel_id, user_id, content)
		 VALUES ($1, $2, $3)
		 RETURNING id, channel_id, user_id, content, created_at`,
		channelID, userID, content,
	).Scan(&msg.ID, &msg.ChannelID, &msg.UserID, &msg.Content, &msg.CreatedAt)
	if err != nil {
		return nil, err
	}

	// fetch username for response
	_ = s.db.QueryRow(ctx, `SELECT username FROM users WHERE id = $1`, userID).Scan(&msg.Username)
	return msg, nil
}

func (s *Store) ListMessages(ctx context.Context, channelID int64, limit int) ([]*model.Message, error) {
	rows, err := s.db.Query(ctx,
		`SELECT m.id, m.channel_id, m.user_id, u.username, m.content, m.created_at
		 FROM messages m JOIN users u ON u.id = m.user_id
		 WHERE m.channel_id = $1
		 ORDER BY m.created_at DESC
		 LIMIT $2`,
		channelID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*model.Message
	for rows.Next() {
		msg := &model.Message{}
		if err := rows.Scan(&msg.ID, &msg.ChannelID, &msg.UserID, &msg.Username, &msg.Content, &msg.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	// reverse so oldest first
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}
	return msgs, rows.Err()
}
