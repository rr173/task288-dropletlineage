package store

import (
	"context"
	"database/sql"
	"errors"

	"task288-dropletlineage/internal/model"
)

// VersionStore 提供谱系版本与版本事件快照的读写。
type VersionStore struct {
	db *DB
}

// NewVersionStore 构造版本存储。
func NewVersionStore(db *DB) *VersionStore { return &VersionStore{db: db} }

const versionCols = `id, batch_id, rev, status, note, event_count, frame_count, frozen_at, superseded_by, created_at, published_at`

func scanVersion(row interface{ Scan(...any) error }) (*model.Version, error) {
	var v model.Version
	err := row.Scan(&v.ID, &v.BatchID, &v.Rev, &v.Status, &v.Note, &v.EventCount, &v.FrameCount,
		&v.FrozenAt, &v.SupersededBy, &v.CreatedAt, &v.PublishedAt)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

// Create 插入版本与版本事件快照；事务内完成。
func (s *VersionStore) Create(ctx context.Context, v *model.Version, eventIDs []string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `INSERT INTO versions (`+versionCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		v.ID, v.BatchID, v.Rev, v.Status, v.Note, v.EventCount, v.FrameCount,
		v.FrozenAt, v.SupersededBy, v.CreatedAt, v.PublishedAt); err != nil {
		if isUniqueViolation(err) {
			return model.ErrConflict
		}
		return err
	}
	for _, eid := range eventIDs {
		if _, err := tx.ExecContext(ctx, `INSERT INTO version_events (version_id, event_id) VALUES (?,?)`, v.ID, eid); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Get 按 ID 读取版本。
func (s *VersionStore) Get(ctx context.Context, id string) (*model.Version, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+versionCols+` FROM versions WHERE id = ?`, id)
	v, err := scanVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return v, err
}

// ListByBatch 返回批次内全部版本，按 rev 降序。
func (s *VersionStore) ListByBatch(ctx context.Context, batchID string) ([]*model.Version, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+versionCols+` FROM versions WHERE batch_id=? ORDER BY rev DESC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Version
	for rows.Next() {
		v, err := scanVersion(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// NextRev 返回批次内下一个版本号（rev+1）。
func (s *VersionStore) NextRev(ctx context.Context, batchID string) (int, error) {
	var max sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MAX(rev) FROM versions WHERE batch_id=?`, batchID).Scan(&max)
	if err != nil {
		return 0, err
	}
	if !max.Valid {
		return 1, nil
	}
	return int(max.Int64) + 1, nil
}

// UpdateStatus 更新版本状态（draft→shared/frozen、frozen→superseded）。
func (s *VersionStore) UpdateStatus(ctx context.Context, id, status, publishedAt string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE versions SET status=?, published_at=? WHERE id=?`, status, publishedAt, id)
	if err != nil {
		return err
	}
	if err := ensureRowsAffected(res, nil); err != nil {
		return model.ErrNotFound
	}
	return nil
}

// MarkFrozen 标记版本冻结时间。
func (s *VersionStore) MarkFrozen(ctx context.Context, id, frozenAt string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE versions SET frozen_at=? WHERE id=?`, frozenAt, id)
	if err != nil {
		return err
	}
	return ensureRowsAffected(res, nil)
}

// MarkSuperseded 把旧版本标记为被新版本替代。
func (s *VersionStore) MarkSuperseded(ctx context.Context, oldID, newID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE versions SET status=?, superseded_by=? WHERE id=?`, model.VersionSuperseded, newID, oldID)
	return err
}

// EventIDs 返回版本内固化的谱系事件 ID 列表。
func (s *VersionStore) EventIDs(ctx context.Context, versionID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_id FROM version_events WHERE version_id=? ORDER BY event_id ASC`, versionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var eid string
		if err := rows.Scan(&eid); err != nil {
			return nil, err
		}
		out = append(out, eid)
	}
	return out, rows.Err()
}

// FrozenVersion 返回批次内最新的冻结版本（无则 ErrNotFound）。
func (s *VersionStore) FrozenVersion(ctx context.Context, batchID string) (*model.Version, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+versionCols+` FROM versions WHERE batch_id=? AND status=? ORDER BY rev DESC LIMIT 1`, batchID, model.VersionFrozen)
	v, err := scanVersion(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return v, err
}

// FrameCount 返回批次内帧总数（版本快照摘要用）。
func (s *VersionStore) FrameCount(ctx context.Context, batchID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM frames WHERE batch_id=?`, batchID).Scan(&n)
	return n, err
}
