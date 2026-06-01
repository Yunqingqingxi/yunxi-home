package database

import (
    "context"
    "fmt"
    "time"

    "github.com/Yunqingqingxi/yunxi-home/internal/models"
)

// ChatSessionRepo implements ChatSessionRepository against SQLite.
type ChatSessionRepo struct {
    db Executor
}

func NewChatSessionRepo(db Executor) *ChatSessionRepo {
    return &ChatSessionRepo{db: db}
}

func (r *ChatSessionRepo) List(ctx context.Context) ([]models.ChatSession, error) {
    rows, err := r.db.QueryContext(ctx,
        "SELECT id, type, title, COALESCE(messages, '[]') as messages, created_at, updated_at FROM chat_sessions ORDER BY updated_at DESC")
    if err != nil {
        return nil, fmt.Errorf("list chat_sessions: %w", err)
    }
    defer rows.Close()

    var result []models.ChatSession
    for rows.Next() {
        var s models.ChatSession
        if err := rows.Scan(&s.ID, &s.Type, &s.Title, &s.MessagesJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
            return nil, fmt.Errorf("scan chat_session: %w", err)
        }
        result = append(result, s)
    }
    return result, rows.Err()
}

func (r *ChatSessionRepo) ListByType(ctx context.Context, sessionType string) ([]models.ChatSession, error) {
    rows, err := r.db.QueryContext(ctx,
        "SELECT id, type, title, COALESCE(messages, '[]') as messages, created_at, updated_at FROM chat_sessions WHERE type=? ORDER BY updated_at DESC", sessionType)
    if err != nil {
        return nil, fmt.Errorf("list chat_sessions by type: %w", err)
    }
    defer rows.Close()

    var result []models.ChatSession
    for rows.Next() {
        var s models.ChatSession
        if err := rows.Scan(&s.ID, &s.Type, &s.Title, &s.MessagesJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
            return nil, fmt.Errorf("scan chat_session: %w", err)
        }
        result = append(result, s)
    }
    return result, rows.Err()
}

func (r *ChatSessionRepo) Upsert(ctx context.Context, s *models.ChatSession) error {
    now := time.Now()
    s.UpdatedAt = now
    if s.CreatedAt.IsZero() {
        s.CreatedAt = now
    }
    if s.Type == "" {
        s.Type = "chat"
    }
    _, err := r.db.ExecContext(ctx,
        `INSERT INTO chat_sessions (id, type, title, messages, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?)
         ON CONFLICT(id) DO UPDATE SET type=excluded.type, title=excluded.title, messages=excluded.messages, updated_at=excluded.updated_at`,
        s.ID, s.Type, s.Title, s.MessagesJSON, s.CreatedAt, s.UpdatedAt,
    )
    if err != nil {
        return fmt.Errorf("upsert chat_session: %w", err)
    }
    return nil
}

func (r *ChatSessionRepo) Delete(ctx context.Context, id string) error {
    _, err := r.db.ExecContext(ctx, "DELETE FROM chat_sessions WHERE id=?", id)
    return err
}

func (r *ChatSessionRepo) DeleteByType(ctx context.Context, sessionType string) (int64, error) {
    res, err := r.db.ExecContext(ctx, "DELETE FROM chat_sessions WHERE type=?", sessionType)
    if err != nil {
        return 0, err
    }
    return res.RowsAffected()
}

func (r *ChatSessionRepo) DeleteStale(ctx context.Context, sessionType string, olderThan time.Duration) (int64, error) {
    cutoff := time.Now().Add(-olderThan)
    res, err := r.db.ExecContext(ctx,
        "DELETE FROM chat_sessions WHERE type=? AND updated_at < ?", sessionType, cutoff)
    if err != nil {
        return 0, err
    }
    return res.RowsAffected()
}

func (r *ChatSessionRepo) DeleteAll(ctx context.Context) error {
    _, err := r.db.ExecContext(ctx, "DELETE FROM chat_sessions")
    return err
}

// MySQLChatSessionRepo implements ChatSessionRepository against MySQL.
type MySQLChatSessionRepo struct {
    db Executor
}

func NewMySQLChatSessionRepo(db Executor) *MySQLChatSessionRepo {
    return &MySQLChatSessionRepo{db: db}
}

func (r *MySQLChatSessionRepo) List(ctx context.Context) ([]models.ChatSession, error) {
    rows, err := r.db.QueryContext(ctx,
        "SELECT id, type, title, COALESCE(messages, '[]') as messages, created_at, updated_at FROM chat_sessions ORDER BY updated_at DESC")
    if err != nil {
        return nil, fmt.Errorf("list chat_sessions: %w", err)
    }
    defer rows.Close()

    var result []models.ChatSession
    for rows.Next() {
        var s models.ChatSession
        if err := rows.Scan(&s.ID, &s.Type, &s.Title, &s.MessagesJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
            return nil, fmt.Errorf("scan chat_session: %w", err)
        }
        result = append(result, s)
    }
    return result, rows.Err()
}

func (r *MySQLChatSessionRepo) ListByType(ctx context.Context, sessionType string) ([]models.ChatSession, error) {
    rows, err := r.db.QueryContext(ctx,
        "SELECT id, type, title, COALESCE(messages, '[]') as messages, created_at, updated_at FROM chat_sessions WHERE type=? ORDER BY updated_at DESC", sessionType)
    if err != nil {
        return nil, fmt.Errorf("list chat_sessions by type: %w", err)
    }
    defer rows.Close()

    var result []models.ChatSession
    for rows.Next() {
        var s models.ChatSession
        if err := rows.Scan(&s.ID, &s.Type, &s.Title, &s.MessagesJSON, &s.CreatedAt, &s.UpdatedAt); err != nil {
            return nil, fmt.Errorf("scan chat_session: %w", err)
        }
        result = append(result, s)
    }
    return result, rows.Err()
}

func (r *MySQLChatSessionRepo) Upsert(ctx context.Context, s *models.ChatSession) error {
    now := time.Now()
    s.UpdatedAt = now
    if s.CreatedAt.IsZero() {
        s.CreatedAt = now
    }
    if s.Type == "" {
        s.Type = "chat"
    }
    _, err := r.db.ExecContext(ctx,
        `INSERT INTO chat_sessions (id, type, title, messages, created_at, updated_at)
         VALUES (?, ?, ?, ?, ?, ?)
         ON DUPLICATE KEY UPDATE type=VALUES(type), title=VALUES(title), messages=VALUES(messages), updated_at=VALUES(updated_at)`,
        s.ID, s.Type, s.Title, s.MessagesJSON, s.CreatedAt, s.UpdatedAt,
    )
    if err != nil {
        return fmt.Errorf("upsert chat_session: %w", err)
    }
    return nil
}

func (r *MySQLChatSessionRepo) Delete(ctx context.Context, id string) error {
    _, err := r.db.ExecContext(ctx, "DELETE FROM chat_sessions WHERE id=?", id)
    return err
}

func (r *MySQLChatSessionRepo) DeleteByType(ctx context.Context, sessionType string) (int64, error) {
    res, err := r.db.ExecContext(ctx, "DELETE FROM chat_sessions WHERE type=?", sessionType)
    if err != nil {
        return 0, err
    }
    return res.RowsAffected()
}

func (r *MySQLChatSessionRepo) DeleteStale(ctx context.Context, sessionType string, olderThan time.Duration) (int64, error) {
    cutoff := time.Now().Add(-olderThan)
    res, err := r.db.ExecContext(ctx,
        "DELETE FROM chat_sessions WHERE type=? AND updated_at < ?", sessionType, cutoff)
    if err != nil {
        return 0, err
    }
    return res.RowsAffected()
}

func (r *MySQLChatSessionRepo) DeleteAll(ctx context.Context) error {
    _, err := r.db.ExecContext(ctx, "DELETE FROM chat_sessions")
    return err
}
