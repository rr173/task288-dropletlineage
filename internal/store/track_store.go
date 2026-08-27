package store

import (
	"context"
	"database/sql"
	"errors"

	"task288-dropletlineage/internal/model"
)

// TrackStore 提供轨迹与轨迹边的读写。
type TrackStore struct {
	db *DB
}

// NewTrackStore 构造轨迹存储。
func NewTrackStore(db *DB) *TrackStore { return &TrackStore{db: db} }

const trackCols = `id, batch_id, droplet_key, start_frame_seq, end_frame_seq, obs_count, state, broken_at_seq, created_at`

func scanTrack(row interface{ Scan(...any) error }) (*model.Track, error) {
	var t model.Track
	err := row.Scan(&t.ID, &t.BatchID, &t.DropletKey, &t.StartFrameSeq, &t.EndFrameSeq,
		&t.ObsCount, &t.State, &t.BrokenAtSeq, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// ReplaceForBatch 清空批次内旧轨迹与边，重建跟踪结果（跟踪可重跑，幂等）。
// 在单事务内完成，失败回滚，避免残留半程跟踪。
func (s *TrackStore) ReplaceForBatch(ctx context.Context, batchID string, tracks []*model.Track, links []*model.TrackLink) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `DELETE FROM track_links WHERE track_id IN (SELECT id FROM tracks WHERE batch_id=?)`, batchID); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM tracks WHERE batch_id=?`, batchID); err != nil {
		return err
	}
	for _, t := range tracks {
		if _, err := tx.ExecContext(ctx, `INSERT INTO tracks (`+trackCols+`) VALUES (?,?,?,?,?,?,?,?,?)`,
			t.ID, t.BatchID, t.DropletKey, t.StartFrameSeq, t.EndFrameSeq, t.ObsCount, t.State, t.BrokenAtSeq, t.CreatedAt); err != nil {
			return err
		}
	}
	for _, l := range links {
		if _, err := tx.ExecContext(ctx, `INSERT INTO track_links (id, track_id, from_obs_id, to_obs_id, from_seq, to_seq, distance, match_type, extrapolation, created_at) VALUES (?,?,?,?,?,?,?,?,?,?)`,
			l.ID, l.TrackID, l.FromObsID, l.ToObsID, l.FromSeq, l.ToSeq, l.Distance, l.MatchType, l.Extrapolation, l.CreatedAt); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListByBatch 返回批次内全部轨迹。
func (s *TrackStore) ListByBatch(ctx context.Context, batchID string) ([]*model.Track, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+trackCols+` FROM tracks WHERE batch_id=? ORDER BY droplet_key ASC`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// Get 按 ID 读取轨迹。
func (s *TrackStore) Get(ctx context.Context, id string) (*model.Track, error) {
	row := s.db.QueryRowContext(ctx, `SELECT `+trackCols+` FROM tracks WHERE id = ?`, id)
	t, err := scanTrack(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, model.ErrNotFound
	}
	return t, err
}

// ListLinksByTrack 返回轨迹的全部关联边（按帧序升序）。
func (s *TrackStore) ListLinksByTrack(ctx context.Context, trackID string) ([]model.TrackLink, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, track_id, from_obs_id, to_obs_id, from_seq, to_seq, distance, match_type, extrapolation, created_at FROM track_links WHERE track_id=? ORDER BY from_seq ASC`, trackID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.TrackLink
	for rows.Next() {
		var l model.TrackLink
		if err := rows.Scan(&l.ID, &l.TrackID, &l.FromObsID, &l.ToObsID, &l.FromSeq, &l.ToSeq,
			&l.Distance, &l.MatchType, &l.Extrapolation, &l.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}

// BrokenTracks 返回批次内断裂轨迹（遮挡候选）。
func (s *TrackStore) BrokenTracks(ctx context.Context, batchID string) ([]*model.Track, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+trackCols+` FROM tracks WHERE batch_id=? AND state=? ORDER BY droplet_key ASC`, batchID, model.TrackBroken)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*model.Track
	for rows.Next() {
		t, err := scanTrack(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
