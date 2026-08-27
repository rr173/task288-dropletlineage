package store

import (
	"context"
	"database/sql"
	"errors"

	"task288-dropletlineage/internal/model"
)

// EventStore 提供谱系事件、参与者与守恒校验记录的读写。
type EventStore struct {
	db *DB
}

// NewEventStore 构造谱系事件存储。
func NewEventStore(db *DB) *EventStore { return &EventStore{db: db} }

const eventCols = `id, batch_id, kind, status, tolerance, reason, created_at, decided_at`

func scanEvent(row interface{ Scan(...any) error }) (*model.LineageEvent, error) {
	var e model.LineageEvent
	err := row.Scan(&e.ID, &e.BatchID, &e.Kind, &e.Status, &e.Tolerance, &e.Reason, &e.CreatedAt, &e.DecidedAt)
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// Create 插入谱系事件与参与者，返回事件记录。
// 事务内完成：事件 + 参与者 + 批次事件计数自增，任一失败整体回滚。
func (s *EventStore) Create(ctx context.Context, e *model.LineageEvent, participants []model.EventParticipant) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `INSERT INTO lineage_events (`+eventCols+`) VALUES (?,?,?,?,?,?,?,?)`,
		e.ID, e.BatchID, e.Kind, e.Status, e.Tolerance, e.Reason, e.CreatedAt, e.DecidedAt); err != nil {
		return err
	}
	for _, p := range participants {
		if _, err := tx.ExecContext(ctx, `INSERT INTO event_participants (event_id, obs_id, droplet_key, role, frame_seq, volume, intensity) VALUES (?,?,?,?,?,?,?)`,
			p.EventID, p.ObsID, p.DropletKey, p.Role, p.FrameSeq, p.Volume, p.Intensity); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// Get 按 ID 读取谱系事件。
func (s *EventStore) Get(ctx context.Context, id string) (*model.LineageEvent, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+eventCols+` FROM lineage_events WHERE id = ?`, id)
	e, err := scanEvent(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return e, err
}

// ListByBatch 返回批次内全部谱系事件，按创建时间倒序。
func (s *EventStore) ListByBatch(ctx context.Context, batchID string) ([]*model.LineageEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+eventCols+` FROM lineage_events WHERE batch_id=? ORDER BY created_at DESC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.LineageEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// ListConfirmedByBatch 返回批次内已确认的谱系事件（版本发布的素材）。
func (s *EventStore) ListConfirmedByBatch(ctx context.Context, batchID string) ([]*model.LineageEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+eventCols+` FROM lineage_events WHERE batch_id=? AND status=? ORDER BY created_at ASC`, batchID, model.EventConfirmed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.LineageEvent
	for rows.Next() {
		e, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// UpdateDecision 记录裁决：确认或否决，写入终态与决定时间。
func (s *EventStore) UpdateDecision(ctx context.Context, id, status, decidedAt string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE lineage_events SET status=?, decided_at=? WHERE id=?`, status, decidedAt, id)
	if err != nil {
		return err
	}
	if err := ensureRowsAffected(res, nil); err != nil {
		return model.ErrNotFound
	}
	return nil
}

// Participants 返回事件的全部参与者观测。
func (s *EventStore) Participants(ctx context.Context, eventID string) ([]model.EventParticipant, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT event_id, obs_id, droplet_key, role, frame_seq, volume, intensity FROM event_participants WHERE event_id=?`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.EventParticipant
	for rows.Next() {
		var p model.EventParticipant
		if err := rows.Scan(&p.EventID, &p.ObsID, &p.DropletKey, &p.Role, &p.FrameSeq, &p.Volume, &p.Intensity); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// AddChecks 写入事件的守恒校验结果（批量）。
func (s *EventStore) AddChecks(ctx context.Context, checks []model.ConservationCheck) error {
	for _, c := range checks {
		if _, err := s.db.ExecContext(ctx, `INSERT INTO conservation_checks (id, event_id, kind, expected_value, actual_value, delta, tolerance, pass, created_at) VALUES (?,?,?,?,?,?,?,?,?)`,
			c.ID, c.EventID, c.Kind, c.ExpectedValue, c.ActualValue, c.Delta, c.Tolerance, boolToInt(c.Pass), c.CreatedAt); err != nil {
			return err
		}
	}
	return nil
}

// ChecksByEvent 返回事件的守恒校验记录。
func (s *EventStore) ChecksByEvent(ctx context.Context, eventID string) ([]model.ConservationCheck, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, event_id, kind, expected_value, actual_value, delta, tolerance, pass, created_at FROM conservation_checks WHERE event_id=?`, eventID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.ConservationCheck
	for rows.Next() {
		var c model.ConservationCheck
		var pass int
		if err := rows.Scan(&c.ID, &c.EventID, &c.Kind, &c.ExpectedValue, &c.ActualValue, &c.Delta, &c.Tolerance, &pass, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Pass = pass != 0
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetEventStatus 更新事件状态（候选→定型）。
func (s *EventStore) SetEventStatus(ctx context.Context, id, status string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE lineage_events SET status=? WHERE id=?`, status, id)
	if err != nil {
		return err
	}
	if err := ensureRowsAffected(res, nil); err != nil {
		return model.ErrNotFound
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
