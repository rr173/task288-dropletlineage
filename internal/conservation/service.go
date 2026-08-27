package conservation

import (
	"context"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
)

// Service 编排守恒校验：读取事件与参与者 → 计算体积/标记检查 → 落库 → 定型事件。
type Service struct {
	db     *store.DB
	events *store.EventStore
}

// NewService 构造守恒校验服务。
func NewService(db *store.DB, e *store.EventStore) *Service {
	return &Service{db: db, events: e}
}

// CheckResult 是一次事件守恒校验的输出。
type CheckResult struct {
	EventID string                    `json:"event_id"`
	Kind    string                    `json:"kind"`
	Status  string                    `json:"status"` // 定型后的事件状态
	Checks  []model.ConservationCheck `json:"checks"`
	Passed  bool                      `json:"passed"`
}

// CheckEvent 校验单个谱系事件的守恒并定型。
// 已关闭事件（confirmed/rejected）直接返回现状；遮挡事件不做守恒定型。
func (s *Service) CheckEvent(ctx context.Context, eventID string, quench float64) (*CheckResult, error) {
	ev, err := s.events.Get(ctx, eventID)
	if err != nil {
		return nil, err
	}
	if ev.IsClosed() {
		return &CheckResult{EventID: ev.ID, Kind: ev.Kind, Status: ev.Status, Passed: ev.Status == model.EventConfirmed}, nil
	}
	if ev.Kind == "occlusion" {
		return &CheckResult{EventID: ev.ID, Kind: ev.Kind, Status: model.EventCandidate, Passed: false}, nil
	}
	parts, err := s.events.Participants(ctx, eventID)
	if err != nil {
		return nil, err
	}
	parents, children := groupObservations(parts)
	tol := ev.Tolerance
	if tol <= 0 {
		tol = DefaultTolerance
	}
	checks := ComputeVolumeChecks(ev.Kind, parents, children, tol)
	checks = append(checks, ComputeMarkerChecks(ev.Kind, parents, children, tol, quench)...)

	passed := true
	for i := range checks {
		if !checks[i].Pass {
			passed = false
		}
	}
	status := model.EventConservationConflict
	if passed {
		if ev.Kind == "split" {
			status = model.EventSplit
		} else if ev.Kind == "merge" {
			status = model.EventMerge
		}
	}
	now := store.Now()
	withIDs := make([]model.ConservationCheck, len(checks))
	for i, c := range checks {
		c.ID = store.NewID()
		c.EventID = eventID
		c.CreatedAt = now
		withIDs[i] = c
	}
	if err := s.events.AddChecks(ctx, withIDs); err != nil {
		return nil, err
	}
	if err := s.events.SetEventStatus(ctx, eventID, status); err != nil {
		return nil, err
	}
	return &CheckResult{EventID: ev.ID, Kind: ev.Kind, Status: status, Checks: withIDs, Passed: passed}, nil
}

// AllResult 是批次批量校验的统计。
type AllResult struct {
	Total       int `json:"total"`
	Conflict    int `json:"conflict"`
	Typed       int `json:"typed"`
	AlreadyDone int `json:"already_done"`
}

// CheckAll 对批次内全部开放的分裂/合并候选事件执行守恒校验。
func (s *Service) CheckAll(ctx context.Context, batchID string, quench float64) (*AllResult, error) {
	events, err := s.events.ListByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	res := &AllResult{}
	for _, ev := range events {
		if ev.IsClosed() {
			res.AlreadyDone++
			continue
		}
		if ev.Kind == "occlusion" {
			continue // 遮挡候选由裁决环节处理，不做守恒定型
		}
		r, err := s.CheckEvent(ctx, ev.ID, quench)
		if err != nil {
			return nil, err
		}
		res.Total++
		if r.Status == model.EventConservationConflict {
			res.Conflict++
		} else {
			res.Typed++
		}
	}
	return res, nil
}

// ListResults 返回批次内全部守恒检查记录。
func (s *Service) ListResults(ctx context.Context, batchID string) ([]model.ConservationCheck, error) {
	events, err := s.events.ListByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	var out []model.ConservationCheck
	for _, ev := range events {
		checks, err := s.events.ChecksByEvent(ctx, ev.ID)
		if err != nil {
			return nil, err
		}
		out = append(out, checks...)
	}
	return out, nil
}
