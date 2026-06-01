package database

import (
	"context"
	"database/sql"
	"time"

	"github.com/yxd/yunxi-home/internal/nas"
)

type shareRepo struct {
	db *sql.DB
}

// NewShareRepo 创建分享仓库 (SQLite)
func NewShareRepo(db *sql.DB) ShareRepository {
	return &shareRepo{db: db}
}

func (r *shareRepo) Create(ctx context.Context, share *nas.Share) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO shares (token, file_path, password, expires_at) VALUES (?, ?, ?, ?)`,
		share.Token, share.FilePath, share.Password, nullTime(share.ExpiresAt),
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *shareRepo) GetByToken(ctx context.Context, token string) (*nas.Share, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, token, file_path, password, expires_at, created_at, downloads FROM shares WHERE token = ?`, token,
	)
	var s nas.Share
	var expiresAt sql.NullTime
	err := row.Scan(&s.ID, &s.Token, &s.FilePath, &s.Password, &expiresAt, &s.CreatedAt, &s.Downloads)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	if expiresAt.Valid {
		s.ExpiresAt = expiresAt.Time
	}
	s.HasPass = s.Password != ""
	return &s, nil
}

func (r *shareRepo) List(ctx context.Context, limit, offset int) ([]nas.Share, int64, error) {
	var total int64
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM shares`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := r.db.QueryContext(ctx,
		`SELECT id, token, file_path, password, expires_at, created_at, downloads FROM shares ORDER BY created_at DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var shares []nas.Share
	for rows.Next() {
		var s nas.Share
		var expiresAt sql.NullTime
		if err := rows.Scan(&s.ID, &s.Token, &s.FilePath, &s.Password, &expiresAt, &s.CreatedAt, &s.Downloads); err != nil {
			return nil, 0, err
		}
		if expiresAt.Valid {
			s.ExpiresAt = expiresAt.Time
		}
		s.HasPass = s.Password != ""
		shares = append(shares, s)
	}
	return shares, total, nil
}

func (r *shareRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM shares WHERE id = ?`, id)
	return err
}

func (r *shareRepo) IncrementDownloads(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE shares SET downloads = downloads + 1 WHERE id = ?`, id)
	return err
}

func (r *shareRepo) CleanExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, `DELETE FROM shares WHERE expires_at IS NOT NULL AND expires_at < datetime('now')`)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func nullTime(t time.Time) interface{} {
	if t.IsZero() {
		return nil
	}
	return t
}
