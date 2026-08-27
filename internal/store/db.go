// Package store 提供基于 SQLite 的持久化实现。
// 使用纯 Go 驱动 modernc.org/sqlite（CGO 无关），版本与 component-versions.json 锁定。
package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// DB 包装 sql.DB 与迁移状态。
type DB struct {
	*sql.DB
}

// Open 打开（必要时创建）SQLite 数据库并执行迁移。
func Open(path string) (*DB, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)", path)
	sqldb, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	sqldb.SetMaxOpenConns(1) // SQLite 单写者，避免锁竞争
	db := &DB{sqldb}
	if err := db.migrate(context.Background()); err != nil {
		_ = sqldb.Close()
		return nil, err
	}
	return db, nil
}

// Now 返回统一格式化的当前 UTC 时间字符串。
func Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

// NewID 生成 16 字节随机十六进制 ID（128bit 熵，冲突概率可忽略）。
func NewID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(err) // 系统熵源不可用属致命错误
	}
	return hex.EncodeToString(buf)
}

// migrate 按顺序执行建表语句；用 user_version 记录迁移版本。
func (db *DB) migrate(ctx context.Context) error {
	var ver int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&ver); err != nil {
		return err
	}
	if ver >= 1 {
		return nil
	}
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS batches (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			chamber_width REAL NOT NULL,
			chamber_height REAL NOT NULL,
			status TEXT NOT NULL,
			frame_count INTEGER NOT NULL DEFAULT 0,
			obs_count INTEGER NOT NULL DEFAULT 0,
			event_count INTEGER NOT NULL DEFAULT 0,
			version_count INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS frames (
			id TEXT PRIMARY KEY,
			batch_id TEXT NOT NULL REFERENCES batches(id),
			seq INTEGER NOT NULL,
			t_ms INTEGER NOT NULL,
			channel TEXT NOT NULL,
			image_ref TEXT NOT NULL,
			obs_count INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			UNIQUE(batch_id, seq)
		)`,
		`CREATE TABLE IF NOT EXISTS observations (
			id TEXT PRIMARY KEY,
			batch_id TEXT NOT NULL REFERENCES batches(id),
			frame_id TEXT NOT NULL REFERENCES frames(id),
			frame_seq INTEGER NOT NULL,
			droplet_key TEXT NOT NULL,
			x REAL NOT NULL,
			y REAL NOT NULL,
			radius REAL NOT NULL,
			volume REAL NOT NULL,
			intensity REAL NOT NULL,
			marker_id TEXT NOT NULL,
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			UNIQUE(batch_id, droplet_key, frame_seq)
		)`,
		`CREATE TABLE IF NOT EXISTS tracks (
			id TEXT PRIMARY KEY,
			batch_id TEXT NOT NULL REFERENCES batches(id),
			droplet_key TEXT NOT NULL,
			start_frame_seq INTEGER NOT NULL,
			end_frame_seq INTEGER NOT NULL,
			obs_count INTEGER NOT NULL DEFAULT 0,
			state TEXT NOT NULL,
			broken_at_seq INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL,
			UNIQUE(batch_id, droplet_key)
		)`,
		`CREATE TABLE IF NOT EXISTS track_links (
			id TEXT PRIMARY KEY,
			track_id TEXT NOT NULL REFERENCES tracks(id),
			from_obs_id TEXT NOT NULL,
			to_obs_id TEXT NOT NULL,
			from_seq INTEGER NOT NULL,
			to_seq INTEGER NOT NULL,
			distance REAL NOT NULL,
			match_type TEXT NOT NULL,
			extrapolation REAL NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS lineage_events (
			id TEXT PRIMARY KEY,
			batch_id TEXT NOT NULL REFERENCES batches(id),
			kind TEXT NOT NULL,
			status TEXT NOT NULL,
			tolerance REAL NOT NULL,
			reason TEXT NOT NULL,
			created_at TEXT NOT NULL,
			decided_at TEXT NOT NULL DEFAULT ''
		)`,
		`CREATE TABLE IF NOT EXISTS event_participants (
			event_id TEXT NOT NULL REFERENCES lineage_events(id),
			obs_id TEXT NOT NULL REFERENCES observations(id),
			droplet_key TEXT NOT NULL,
			role TEXT NOT NULL,
			frame_seq INTEGER NOT NULL,
			volume REAL NOT NULL,
			intensity REAL NOT NULL,
			PRIMARY KEY(event_id, obs_id)
		)`,
		`CREATE TABLE IF NOT EXISTS conservation_checks (
			id TEXT PRIMARY KEY,
			event_id TEXT NOT NULL REFERENCES lineage_events(id),
			kind TEXT NOT NULL,
			expected_value REAL NOT NULL,
			actual_value REAL NOT NULL,
			delta REAL NOT NULL,
			tolerance REAL NOT NULL,
			pass INTEGER NOT NULL,
			created_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS versions (
			id TEXT PRIMARY KEY,
			batch_id TEXT NOT NULL REFERENCES batches(id),
			rev INTEGER NOT NULL,
			status TEXT NOT NULL,
			note TEXT NOT NULL,
			event_count INTEGER NOT NULL DEFAULT 0,
			frame_count INTEGER NOT NULL DEFAULT 0,
			frozen_at TEXT NOT NULL DEFAULT '',
			superseded_by TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			published_at TEXT NOT NULL DEFAULT '',
			UNIQUE(batch_id, rev)
		)`,
		`CREATE TABLE IF NOT EXISTS version_events (
			version_id TEXT NOT NULL REFERENCES versions(id),
			event_id TEXT NOT NULL REFERENCES lineage_events(id),
			PRIMARY KEY(version_id, event_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_frames_batch ON frames(batch_id, seq)`,
		`CREATE INDEX IF NOT EXISTS idx_obs_batch ON observations(batch_id, frame_seq)`,
		`CREATE INDEX IF NOT EXISTS idx_obs_droplet ON observations(batch_id, droplet_key)`,
		`CREATE INDEX IF NOT EXISTS idx_tracks_batch ON tracks(batch_id, state)`,
		`CREATE INDEX IF NOT EXISTS idx_events_batch ON lineage_events(batch_id, status)`,
		`CREATE INDEX IF NOT EXISTS idx_checks_event ON conservation_checks(event_id)`,
		`CREATE INDEX IF NOT EXISTS idx_versions_batch ON versions(batch_id, rev)`,
		`CREATE INDEX IF NOT EXISTS idx_ve_version ON version_events(version_id)`,
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	for _, s := range stmts {
		if _, err := tx.ExecContext(ctx, s); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("migrate: %w", err)
		}
	}
	if _, err := tx.ExecContext(ctx, "PRAGMA user_version = 1"); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}
