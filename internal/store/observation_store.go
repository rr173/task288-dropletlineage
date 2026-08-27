package store

import (
	"context"
	"database/sql"
	"errors"

	"task288-dropletlineage/internal/model"
)

// ObservationStore 提供液滴观测表的读写。
type ObservationStore struct {
	db *DB
}

// NewObservationStore 构造观测存储。
func NewObservationStore(db *DB) *ObservationStore { return &ObservationStore{db: db} }

const obsCols = `id, batch_id, frame_id, frame_seq, droplet_key, x, y, radius, volume, intensity, marker_id, status, created_at`

func scanObservation(row interface{ Scan(...any) error }) (*model.Observation, error) {
	var o model.Observation
	err := row.Scan(&o.ID, &o.BatchID, &o.FrameID, &o.FrameSeq, &o.DropletKey,
		&o.X, &o.Y, &o.Radius, &o.Volume, &o.Intensity, &o.MarkerID, &o.Status, &o.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// Create 插入一条观测；同帧同液滴重复返回唯一约束错误。
func (s *ObservationStore) Create(ctx context.Context, tx *sql.Tx, o *model.Observation) error {
	var err error
	if tx != nil {
		_, err = tx.ExecContext(ctx, `INSERT INTO observations (`+obsCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			o.ID, o.BatchID, o.FrameID, o.FrameSeq, o.DropletKey, o.X, o.Y, o.Radius,
			o.Volume, o.Intensity, o.MarkerID, o.Status, o.CreatedAt)
	} else {
		_, err = s.db.ExecContext(ctx, `INSERT INTO observations (`+obsCols+`) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			o.ID, o.BatchID, o.FrameID, o.FrameSeq, o.DropletKey, o.X, o.Y, o.Radius,
			o.Volume, o.Intensity, o.MarkerID, o.Status, o.CreatedAt)
	}
	if err != nil && isUniqueViolation(err) {
		return model.ErrConflict
	}
	return err
}

// Get 按 ID 读取观测。
func (s *ObservationStore) Get(ctx context.Context, id string) (*model.Observation, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+obsCols+` FROM observations WHERE id = ?`, id)
	o, err := scanObservation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return o, err
}

// ListByBatch 返回批次内全部观测，按帧序 + 液滴 ID 排序。
func (s *ObservationStore) ListByBatch(ctx context.Context, batchID string) ([]*model.Observation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+obsCols+` FROM observations WHERE batch_id=? ORDER BY frame_seq ASC, droplet_key ASC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectObservations(rows)
}

// ListByFrameSeq 返回某帧内全部观测。
func (s *ObservationStore) ListByFrameSeq(ctx context.Context, batchID string, seq int) ([]*model.Observation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+obsCols+` FROM observations WHERE batch_id=? AND frame_seq=? ORDER BY droplet_key ASC`, batchID, seq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectObservations(rows)
}

// ListByDroplet 返回某液滴在批次内的全部观测（按帧序升序）。
func (s *ObservationStore) ListByDroplet(ctx context.Context, batchID, dropletKey string) ([]*model.Observation, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+obsCols+` FROM observations WHERE batch_id=? AND droplet_key=? ORDER BY frame_seq ASC`, batchID, dropletKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectObservations(rows)
}

// ListByIDs 批量读取观测（用于谱系事件参与者）。
func (s *ObservationStore) ListByIDs(ctx context.Context, ids []string) ([]*model.Observation, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	q := `SELECT ` + obsCols + ` FROM observations WHERE id IN (` + placeholders(len(ids)) + `)`
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectObservations(rows)
}

// UpdateStatus 更新观测状态（raw/tracked/occluded/excluded）。
func (s *ObservationStore) UpdateStatus(ctx context.Context, id, status string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE observations SET status=? WHERE id=?`, status, id)
	if err != nil {
		return err
	}
	if err := ensureRowsAffected(res, nil); err != nil {
		return model.ErrNotFound
	}
	return nil
}

// MarkStatusByDroplet 把某液滴的全部观测统一置为指定状态。
func (s *ObservationStore) MarkStatusByDroplet(ctx context.Context, batchID, dropletKey, status string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE observations SET status=? WHERE batch_id=? AND droplet_key=?`, status, batchID, dropletKey)
	return err
}

// Count 返回批次内观测总数。
func (s *ObservationStore) Count(ctx context.Context, batchID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM observations WHERE batch_id=?`, batchID).Scan(&n)
	return n, err
}

func collectObservations(rows *sql.Rows) ([]*model.Observation, error) {
	var out []*model.Observation
	for rows.Next() {
		o, err := scanObservation(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// placeholders 生成 "?,?,?" 形式的占位符串。
func placeholders(n int) string {
	out := ""
	for i := 0; i < n; i++ {
		if i > 0 {
			out += ","
		}
		out += "?"
	}
	return out
}
