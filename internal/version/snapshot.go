package version

import (
	"context"

	"task288-dropletlineage/internal/model"
)

// Detail 是版本的完整快照视图：版本头 + 固化事件 + 每事件的参与者。
// 冻结版本通过该视图对外提供不可变证据（读取路径不落库）。
type Detail struct {
	Version *model.Version       `json:"version"`
	Events  []*EventEvidence     `json:"events"`
}

// EventEvidence 是版本内一个事件的证据：事件 + 参与者 + 守恒检查。
type EventEvidence struct {
	Event        *model.LineageEvent      `json:"event"`
	Participants []model.EventParticipant `json:"participants"`
	Checks       []model.ConservationCheck `json:"checks"`
}

// DetailOf 组装版本详情（事件集与参与者证据）。
func (s *Service) DetailOf(ctx context.Context, versionID string) (*Detail, error) {
	v, err := s.versions.Get(ctx, versionID)
	if err != nil {
		return nil, err
	}
	eventIDs, err := s.versions.EventIDs(ctx, versionID)
	if err != nil {
		return nil, err
	}
	d := &Detail{Version: v}
	for _, eid := range eventIDs {
		ev, err := s.events.Get(ctx, eid)
		if err != nil {
			return nil, err
		}
		parts, err := s.events.Participants(ctx, eid)
		if err != nil {
			return nil, err
		}
		checks, err := s.events.ChecksByEvent(ctx, eid)
		if err != nil {
			return nil, err
		}
		d.Events = append(d.Events, &EventEvidence{Event: ev, Participants: parts, Checks: checks})
	}
	return d, nil
}
