package tracking

import (
	"context"
	"sort"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
)

// Service 编排跟踪：读取观测 → 分帧 → 匹配 → 落库轨迹与边 → 更新观测状态。
// 可重复运行（幂等）：每次以当前观测全集重建轨迹，旧轨迹整体替换。
type Service struct {
	db     *store.DB
	obs    *store.ObservationStore
	tracks *store.TrackStore
}

// NewService 构造跟踪服务。
func NewService(db *store.DB, o *store.ObservationStore, t *store.TrackStore) *Service {
	return &Service{db: db, obs: o, tracks: t}
}

// Result 是跟踪运行后的统计摘要。
type Result struct {
	TrackCount   int              `json:"track_count"`
	LinkCount    int              `json:"link_count"`
	BrokenCount  int              `json:"broken_count"`
	SingleCount  int              `json:"single_count"`
	Hints        []OcclusionHint  `json:"occlusion_hints"`
	MatchedObs   int              `json:"matched_obs"`
	OccludedObs  int              `json:"occluded_obs"`
}

// Run 执行一次完整跟踪，返回统计结果。
func (s *Service) Run(ctx context.Context, batchID string, m *Matcher) (*Result, error) {
	all, err := s.obs.ListByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	byFrame := map[int][]*model.Observation{}
	var order []int
	for _, o := range all {
		if _, ok := byFrame[o.FrameSeq]; !ok {
			order = append(order, o.FrameSeq)
		}
		byFrame[o.FrameSeq] = append(byFrame[o.FrameSeq], o)
	}
	sort.Ints(order)

	built := m.Build(batchID, byFrame, order)
	if err := s.tracks.ReplaceForBatch(ctx, batchID, built.Tracks, built.Links); err != nil {
		return nil, err
	}
	// 更新观测状态：参与关联 → tracked；遮挡断裂边缘 → occluded
	for id := range built.LinkedObs {
		if err := s.obs.UpdateStatus(ctx, id, model.ObsTracked); err != nil && err != model.ErrNotFound {
			return nil, err
		}
	}
	for id := range built.OccludedObs {
		if err := s.obs.UpdateStatus(ctx, id, model.ObsOccluded); err != nil && err != model.ErrNotFound {
			return nil, err
		}
	}
	res := &Result{
		TrackCount:  len(built.Tracks),
		LinkCount:   len(built.Links),
		BrokenCount: countBroken(built.Tracks),
		SingleCount: len(built.SingleFrame),
		Hints:       built.Hints,
		MatchedObs:  len(built.LinkedObs),
		OccludedObs: len(built.OccludedObs),
	}
	return res, nil
}

func countBroken(tracks []*model.Track) int {
	n := 0
	for _, t := range tracks {
		if t.State == model.TrackBroken {
			n++
		}
	}
	return n
}
