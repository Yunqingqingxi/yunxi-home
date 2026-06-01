package database

import (
	"context"
	"database/sql"
	"github.com/yxd/yunxi-home/internal/nas"
)

// MySQLShareRepo implements ShareRepository for MySQL.
type MySQLShareRepo struct{ db Executor }

func NewMySQLShareRepo(db Executor) *MySQLShareRepo { return &MySQLShareRepo{db: db} }

var _ ShareRepository = (*MySQLShareRepo)(nil)

func (r *MySQLShareRepo) Create(ctx context.Context, share *nas.Share) (int64, error) {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO shares (token, file_path, password, expires_at) VALUES (?, ?, ?, ?)`,
		share.Token, share.FilePath, share.Password, nullTime(share.ExpiresAt))
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (r *MySQLShareRepo) GetByToken(ctx context.Context, token string) (*nas.Share, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT id, token, file_path, password, expires_at, created_at, downloads FROM shares WHERE token = ?`, token)
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

func (r *MySQLShareRepo) List(ctx context.Context, limit, offset int) ([]nas.Share, int64, error) {
	var total int64
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM shares").Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.QueryContext(ctx,
		`SELECT id, token, file_path, password, expires_at, created_at, downloads FROM shares ORDER BY created_at DESC LIMIT ? OFFSET ?`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var shares []nas.Share
	for rows.Next() {
		var s nas.Share
		var exp sql.NullTime
		if err := rows.Scan(&s.ID, &s.Token, &s.FilePath, &s.Password, &exp, &s.CreatedAt, &s.Downloads); err != nil {
			return nil, 0, err
		}
		if exp.Valid {
			s.ExpiresAt = exp.Time
		}
		s.HasPass = s.Password != ""
		shares = append(shares, s)
	}
	return shares, total, nil
}

func (r *MySQLShareRepo) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM shares WHERE id=?", id)
	return err
}

func (r *MySQLShareRepo) IncrementDownloads(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, "UPDATE shares SET downloads = downloads + 1 WHERE id=?", id)
	return err
}

func (r *MySQLShareRepo) CleanExpired(ctx context.Context) (int64, error) {
	result, err := r.db.ExecContext(ctx, "DELETE FROM shares WHERE expires_at IS NOT NULL AND expires_at < NOW()")
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// nullTime is defined in share_repo.go
