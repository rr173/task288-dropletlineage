package store

import (
	"context"
	"database/sql"
	"errors"

	"task288-dropletlineage/internal/model"
)

// BatchStore 提供批次表的读写。
type BatchStore struct {
	db *DB
}

// NewBatchStore 构造批次存储。
func NewBatchStore(db *DB) *BatchStore { return &BatchStore{db: db} }

const batchCols = `id, name, chamber_width, chamber_height, status,
	frame_count, obs_count, event_count, version_count, created_at, updated_at`

func scanBatch(row interface{ Scan(...any) error }) (*model.Batch, error) {
	var b model.Batch
	err := row.Scan(&b.ID, &b.Name, &b.ChamberWidth, &b.ChamberHeight, &b.Status,
		&b.FrameCount, &b.ObsCount, &b.EventCount, &b.VersionCount, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}

// Create 插入一个新批次。
func (s *BatchStore) Create(ctx context.Context, b *model.Batch) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO batches (`+batchCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		b.ID, b.Name, b.ChamberWidth, b.ChamberHeight, b.Status,
		b.FrameCount, b.ObsCount, b.EventCount, b.VersionCount, b.CreatedAt, b.UpdatedAt)
	return err
}

// Get 按 ID 读取批次。
func (s *BatchStore) Get(ctx context.Context, id string) (*model.Batch, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+batchCols+` FROM batches WHERE id = ?`, id)
	b, err := scanBatch(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return b, err
}

// List 返回全部批次，按创建时间倒序。
func (s *BatchStore) List(ctx context.Context) ([]*model.Batch, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+batchCols+` FROM batches ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Batch
	for rows.Next() {
		b, err := scanBatch(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateStatus 在事务中更新批次状态并刷新更新时间。
func (s *BatchStore) UpdateStatus(ctx context.Context, tx *sql.Tx, id, status, now string) error {
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, `UPDATE batches SET status=?, updated_at=? WHERE id=?`, status, now, id)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE batches SET status=?, updated_at=? WHERE id=?`, status, now, id)
	}
	return err
}

// IncrFrameCount 在事务中把批次帧计数 +1。
func (s *BatchStore) IncrFrameCount(ctx context.Context, tx *sql.Tx, id string) error {
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, `UPDATE batches SET frame_count=frame_count+1, updated_at=? WHERE id=?`, Now(), id)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE batches SET frame_count=frame_count+1, updated_at=? WHERE id=?`, Now(), id)
	}
	return err
}

// IncrObsCount 在事务中把批次观测计数 +1。
func (s *BatchStore) IncrObsCount(ctx context.Context, tx *sql.Tx, id string) error {
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, `UPDATE batches SET obs_count=obs_count+1, updated_at=? WHERE id=?`, Now(), id)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE batches SET obs_count=obs_count+1, updated_at=? WHERE id=?`, Now(), id)
	}
	return err
}

// IncrEventCount 在事务中把批次谱系事件计数 +1。
func (s *BatchStore) IncrEventCount(ctx context.Context, tx *sql.Tx, id string) error {
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, `UPDATE batches SET event_count=event_count+1, updated_at=? WHERE id=?`, Now(), id)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE batches SET event_count=event_count+1, updated_at=? WHERE id=?`, Now(), id)
	}
	return err
}

// IncrVersionCount 在事务中把批次版本计数 +1。
func (s *BatchStore) IncrVersionCount(ctx context.Context, tx *sql.Tx, id string) error {
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, `UPDATE batches SET version_count=version_count+1, updated_at=? WHERE id=?`, Now(), id)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE batches SET version_count=version_count+1, updated_at=? WHERE id=?`, Now(), id)
	}
	return err
}
