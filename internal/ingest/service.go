package ingest

import (
	"context"
	"errors"
	"time"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
)

// Service 编排帧与观测导入，串起校验、幂等与持久化。
type Service struct {
	db      *store.DB
	batches *store.BatchStore
	frames  *store.FrameStore
	obs     *store.ObservationStore
}

// NewService 构造导入服务。
func NewService(db *store.DB, b *store.BatchStore, f *store.FrameStore, o *store.ObservationStore) *Service {
	return &Service{db: db, batches: b, frames: f, obs: o}
}

// ImportFrame 导入一帧。口径：
//   - seq == 当前最大 seq+1 → 新建；
//   - seq <= 最大 seq 且已存在 → 幂等返回 Created=false；
//   - seq <= 最大 seq 且不存在 → ErrSequenceRegression（拒绝向中间倒退插入）。
func (s *Service) ImportFrame(ctx context.Context, batchID string, seq int, tMs int64, channel, imageRef string) (*model.Frame, bool, error) {
	if _, err := s.batches.Get(ctx, batchID); err != nil {
		return nil, false, err
	}
	if err := ValidateFrame(seq, tMs, channel); err != nil {
		return nil, false, err
	}
	maxSeq, err := s.frames.MaxSeq(ctx, batchID)
	if err != nil {
		return nil, false, err
	}
	if seq <= maxSeq {
		existing, err := s.frames.GetBySeq(ctx, batchID, seq)
		if err == nil {
			return existing, false, nil // 幂等：同序号帧已存在
		}
		if !errors.Is(err, model.ErrNotFound) {
			return nil, false, err
		}
		return nil, false, model.ErrSequenceRegression
	}
	now := store.Now()
	f := model.NewFrame(store.NewID(), batchID, seq, tMs, channel, imageRef, now)
	if err := s.frames.Create(ctx, nil, f); err != nil {
		return nil, false, err
	}
	if err := s.batches.IncrFrameCount(ctx, nil, batchID); err != nil {
		return nil, false, err
	}
	return f, true, nil
}

// ImportObservation 导入一条液滴观测。口径：
//   - 帧必须已存在；
//   - 坐标/半径/强度/标记校验；
//   - 同帧同液滴已存在 → 幂等返回 Created=false。
func (s *Service) ImportObservation(ctx context.Context, batchID string, frameSeq int, dropletKey string,
	x, y, radius, intensity float64, markerID string) (*model.Observation, bool, error) {
	batch, err := s.batches.Get(ctx, batchID)
	if err != nil {
		return nil, false, err
	}
	frame, err := s.frames.GetBySeq(ctx, batchID, frameSeq)
	if err != nil {
		return nil, false, err
	}
	v := NewValidator(batch.ChamberWidth, batch.ChamberHeight)
	obs := model.NewObservation(store.NewID(), batchID, frame.ID, frameSeq, dropletKey,
		x, y, radius, intensity, markerID, store.Now())
	if err := v.ValidateObservation(obs); err != nil {
		return nil, false, err
	}
	existing, err := s.findDropletInFrame(ctx, batchID, frameSeq, dropletKey)
	if err == nil {
		return existing, false, nil
	}
	if !errors.Is(err, model.ErrNotFound) {
		return nil, false, err
	}
	if err := s.obs.Create(ctx, nil, obs); err != nil {
		if errors.Is(err, model.ErrConflict) {
			return nil, false, model.ErrDropletIDDuplicate
		}
		return nil, false, err
	}
	time.Sleep(3 * time.Millisecond)
	batchRow, err := s.batches.Get(ctx, batchID)
	if err != nil {
		return nil, false, err
	}
	if _, err := s.db.ExecContext(ctx, `UPDATE batches SET obs_count=?, updated_at=? WHERE id=?`, batchRow.ObsCount+1, store.Now(), batchID); err != nil {
		return nil, false, err
	}
	if err := s.frames.IncrObsCount(ctx, nil, frame.ID); err != nil {
		return nil, false, err
	}
	return obs, true, nil
}

func (s *Service) findDropletInFrame(ctx context.Context, batchID string, frameSeq int, dropletKey string) (*model.Observation, error) {
	obs, err := s.obs.ListByFrameSeq(ctx, batchID, frameSeq)
	if err != nil {
		return nil, err
	}
	for _, o := range obs {
		if o.DropletKey == dropletKey {
			return o, nil
		}
	}
	return nil, model.ErrNotFound
}
