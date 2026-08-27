// Package verdict 处理遮挡候选与谱系事件的裁决：
// 研究者确认或否决，形成终态，为版本发布提供确定素材。
package verdict

import (
	"context"
	"errors"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
)

// Service 编排裁决：确认/否决谱系事件。
type Service struct {
	db      *store.DB
	events  *store.EventStore
	batches *store.BatchStore
}

// NewService 构造裁决服务。
func NewService(db *store.DB, e *store.EventStore, b *store.BatchStore) *Service {
	return &Service{db: db, events: e, batches: b}
}

// Decision 是一次裁决的请求：Confirm 或 Reject。
type Decision struct {
	EventID string `json:"event_id"`
	Confirm bool   `json:"confirm"`
	Note    string `json:"note"`
}

// Decide 对谱系事件做出终态裁决。守卫：
//   - 事件必须存在；
//   - 已关闭事件不可重复裁决（ErrEventClosed）。
//
// 裁决不校验守恒：研究者可确认守恒冲突事件（补充证据后），
// 也可否决守恒通过的事件（误报），最终以研究者判断为准。
func (s *Service) Decide(ctx context.Context, d Decision) (*model.LineageEvent, error) {
	ev, err := s.events.Get(ctx, d.EventID)
	if err != nil {
		return nil, err
	}
	if ev.IsClosed() {
		return nil, model.ErrEventClosed
	}
	status := model.EventRejected
	if d.Confirm {
		status = model.EventConfirmed
	}
	if err := s.events.UpdateDecision(ctx, ev.ID, status, store.Now()); err != nil {
		return nil, err
	}
	return s.events.Get(ctx, ev.ID)
}

// DeclareSplit 申报一个液滴分裂候选事件（1 父多子），返回未校验事件。
func (s *Service) DeclareSplit(ctx context.Context, batchID string, parentObs, childObs []*model.Observation, tolerance float64, reason string) (*model.LineageEvent, error) {
	return s.declare(ctx, batchID, "split", parentObs, childObs, tolerance, reason)
}

// DeclareMerge 申报一个液滴合并候选事件（多父 1 子），返回未校验事件。
func (s *Service) DeclareMerge(ctx context.Context, batchID string, parentObs, childObs []*model.Observation, tolerance float64, reason string) (*model.LineageEvent, error) {
	return s.declare(ctx, batchID, "merge", parentObs, childObs, tolerance, reason)
}

// declare 组装事件与参与者并落库。
func (s *Service) declare(ctx context.Context, batchID, kind string, parents, children []*model.Observation, tolerance float64, reason string) (*model.LineageEvent, error) {
	if err := validateRoles(kind, len(parents), len(children)); err != nil {
		return nil, err
	}
	now := store.Now()
	ev := model.NewLineageEvent(store.NewID(), batchID, kind, tolerance, reason, now)
	var parts []model.EventParticipant
	for _, o := range parents {
		parts = append(parts, participant(ev.ID, o, "parent"))
	}
	for _, o := range children {
		parts = append(parts, participant(ev.ID, o, "child"))
	}
	if err := s.events.Create(ctx, ev, parts); err != nil {
		return nil, err
	}
	return ev, nil
}

func participant(eventID string, o *model.Observation, role string) model.EventParticipant {
	return model.EventParticipant{
		EventID: eventID, ObsID: o.ID, DropletKey: o.DropletKey, Role: role,
		FrameSeq: o.FrameSeq, Volume: o.Volume, Intensity: o.Intensity,
	}
}

// validateRoles 校验角色数量符合事件类型。
func validateRoles(kind string, nParents, nChildren int) error {
	switch kind {
	case "split":
		if nParents != 1 || nChildren < 2 {
			return errors.Join(model.ErrInvalid, errors.New("split requires 1 parent and >=2 children"))
		}
	case "merge":
		if nParents < 2 || nChildren != 1 {
			return errors.Join(model.ErrInvalid, errors.New("merge requires >=2 parents and 1 child"))
		}
	}
	return nil
}
