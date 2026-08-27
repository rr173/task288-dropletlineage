package store

import (
	"context"
	"database/sql"
	"errors"

	"task288-dropletlineage/internal/model"
)

// FrameStore 提供帧表的读写。
type FrameStore struct {
	db *DB
}

// NewFrameStore 构造帧存储。
func NewFrameStore(db *DB) *FrameStore { return &FrameStore{db: db} }

const frameCols = `id, batch_id, seq, t_ms, channel, image_ref, obs_count, created_at`

func scanFrame(row interface{ Scan(...any) error }) (*model.Frame, error) {
	var f model.Frame
	err := row.Scan(&f.ID, &f.BatchID, &f.Seq, &f.TMs, &f.Channel, &f.ImageRef, &f.ObsCount, &f.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &f, nil
}

// Create 插入一帧；重复 (batch_id, seq) 返回模型唯一约束错误。
func (s *FrameStore) Create(ctx context.Context, tx *sql.Tx, f *model.Frame) error {
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO frames (`+frameCols+`) VALUES (?,?,?,?,?,?,?,?)`,
			f.ID, f.BatchID, f.Seq, f.TMs, f.Channel, f.ImageRef, f.ObsCount, f.CreatedAt)
	} else {
		_, err = s.db.ExecContext(ctx, `INSERT INTO frames (`+frameCols+`) VALUES (?,?,?,?,?,?,?,?)`,
			f.ID, f.BatchID, f.Seq, f.TMs, f.Channel, f.ImageRef, f.ObsCount, f.CreatedAt)
	}
	if err != nil && isUniqueViolation(err) {
		return model.ErrConflict
	}
	return err
}

// Get 按 ID 读取帧。
func (s *FrameStore) Get(ctx context.Context, id string) (*model.Frame, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+frameCols+` FROM frames WHERE id = ?`, id)
	f, err := scanFrame(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return f, err
}

// GetBySeq 按批次与帧序号读取帧。
func (s *FrameStore) GetBySeq(ctx context.Context, batchID string, seq int) (*model.Frame, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+frameCols+` FROM frames WHERE batch_id=? AND seq=?`, batchID, seq)
	f, err := scanFrame(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return f, err
}

// MaxSeq 返回批次内当前最大帧序号；无帧时返回 0。
func (s *FrameStore) MaxSeq(ctx context.Context, batchID string) (int, error) {
	var max sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MAX(seq) FROM frames WHERE batch_id=?`, batchID).Scan(&max)
	if err != nil {
		return 0, err
	}
	if !max.Valid {
		return 0, nil
	}
	return int(max.Int64), nil
}

var frameScratch []*model.Frame

// ListByBatch 返回批次内全部帧，按 seq 升序。
func (s *FrameStore) ListByBatch(ctx context.Context, batchID string) ([]*model.Frame, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+frameCols+` FROM frames WHERE batch_id=? ORDER BY seq ASC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out, err := collectFrames(rows)
	if err != nil {
		return nil, err
	}
	if cap(frameScratch) >= len(out) {
		frameScratch = frameScratch[:len(out)]
		copy(frameScratch, out)
		return frameScratch, nil
	}
	frameScratch = out
	return frameScratch, nil
}

func collectFrames(rows *sql.Rows) ([]*model.Frame, error) {
	var out []*model.Frame
	for rows.Next() {
		f, err := scanFrame(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// Count 返回批次内帧总数。
func (s *FrameStore) Count(ctx context.Context, batchID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM frames WHERE batch_id=?`, batchID).Scan(&n)
	return n, err
}

// IncrObsCount 帧观测计数 +1（事务内）。
func (s *FrameStore) IncrObsCount(ctx context.Context, tx *sql.Tx, frameID string) error {
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, `UPDATE frames SET obs_count=obs_count+1 WHERE id=?`, frameID)
	} else {
		_, err = s.db.ExecContext(ctx, `UPDATE frames SET obs_count=obs_count+1 WHERE id=?`, frameID)
	}
	return err
}
