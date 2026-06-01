package database

import (
	"context"
	"fmt"
)

// ConfigRepo implements ConfigRepository against SQLite (Executor).
type ConfigRepo struct {
	db Executor
}

func NewConfigRepo(db Executor) *ConfigRepo {
	return &ConfigRepo{db: db}
}

// GetSection returns the JSON data for a config section.
func (r *ConfigRepo) GetSection(ctx context.Context, section string) (string, error) {
	var data string
	err := r.db.QueryRowContext(ctx,
		"SELECT data FROM config WHERE section = ?", section,
	).Scan(&data)
	if err != nil {
		// sql.ErrNoRows is fine - just return empty
		return "", nil
	}
	return data, nil
}

// GetAll returns all config sections.
func (r *ConfigRepo) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT section, data FROM config ORDER BY section")
	if err != nil {
		return nil, fmt.Errorf("query config: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var section, data string
		if err := rows.Scan(&section, &data); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		result[section] = data
	}
	return result, rows.Err()
}

// SetSection upserts a config section.
func (r *ConfigRepo) SetSection(ctx context.Context, section, data string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO config (section, data, updated_at) VALUES (?, ?, datetime('now')) ON CONFLICT(section) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at",
		section, data,
	)
	if err != nil {
		return fmt.Errorf("upsert config section %s: %w", section, err)
	}
	return nil
}

// InitDefaults seeds default config sections only if the config table is empty.
func (r *ConfigRepo) InitDefaults(ctx context.Context, defaults map[string]string) error {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM config").Scan(&count); err != nil {
		return fmt.Errorf("count config: %w", err)
	}
	if count > 0 {
		return nil // already seeded
	}
	for section, data := range defaults {
		if err := r.SetSection(ctx, section, data); err != nil {
			return err
		}
	}
	return nil
}


// MySQLConfigRepo implements ConfigRepository against MySQL.
type MySQLConfigRepo struct {
	db Executor
}

func NewMySQLConfigRepo(db Executor) *MySQLConfigRepo {
	return &MySQLConfigRepo{db: db}
}

func (r *MySQLConfigRepo) GetSection(ctx context.Context, section string) (string, error) {
	var data string
	err := r.db.QueryRowContext(ctx,
		"SELECT data FROM config WHERE section = ?", section,
	).Scan(&data)
	if err != nil {
		return "", nil
	}
	return data, nil
}

func (r *MySQLConfigRepo) GetAll(ctx context.Context) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx, "SELECT section, data FROM config ORDER BY section")
	if err != nil {
		return nil, fmt.Errorf("query config: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var section, data string
		if err := rows.Scan(&section, &data); err != nil {
			return nil, fmt.Errorf("scan config: %w", err)
		}
		result[section] = data
	}
	return result, rows.Err()
}

func (r *MySQLConfigRepo) SetSection(ctx context.Context, section, data string) error {
	_, err := r.db.ExecContext(ctx,
		"INSERT INTO config (section, data, updated_at) VALUES (?, ?, NOW()) ON DUPLICATE KEY UPDATE data = VALUES(data), updated_at = NOW()",
		section, data,
	)
	if err != nil {
		return fmt.Errorf("upsert config section %s: %w", section, err)
	}
	return nil
}

func (r *MySQLConfigRepo) InitDefaults(ctx context.Context, defaults map[string]string) error {
	var count int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM config").Scan(&count); err != nil {
		return fmt.Errorf("count config: %w", err)
	}
	if count > 0 {
		return nil
	}
	for section, data := range defaults {
		if err := r.SetSection(ctx, section, data); err != nil {
			return err
		}
	}
	return nil
}
