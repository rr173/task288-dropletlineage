// Package version 管理液滴谱系版本的发布与派生：
// 把已确认的谱系事件固化为不可变快照（冻结），支持派生修订版本。
package version

import (
	"context"
	"errors"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
)

// Service 编排谱系版本生命周期。
type Service struct {
	db       *store.DB
	versions *store.VersionStore
	events   *store.EventStore
	batches  *store.BatchStore
}

// NewService 构造版本服务。
func NewService(db *store.DB, v *store.VersionStore, e *store.EventStore, b *store.BatchStore) *Service {
	return &Service{db: db, versions: v, events: e, batches: b}
}

// CreateDraft 基于批次当前已确认事件创建草稿版本（rev 自动递增）。
func (s *Service) CreateDraft(ctx context.Context, batchID, note string) (*model.Version, error) {
	if _, err := s.batches.Get(ctx, batchID); err != nil {
		return nil, err
	}
	confirmed, err := s.events.ListConfirmedByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	rev, err := s.versions.NextRev(ctx, batchID)
	if err != nil {
		return nil, err
	}
	now := store.Now()
	v := model.NewVersion(store.NewID(), batchID, rev, note, now)
	eventIDs := make([]string, 0, len(confirmed))
	for _, e := range confirmed {
		eventIDs = append(eventIDs, e.ID)
	}
	v.EventCount = len(eventIDs)
	if v.FrameCount, err = s.versions.FrameCount(ctx, batchID); err != nil {
		return nil, err
	}
	if err := s.versions.Create(ctx, v, eventIDs); err != nil {
		return nil, err
	}
	if err := s.batches.IncrVersionCount(ctx, nil, batchID); err != nil {
		return nil, err
	}
	return v, nil
}

// Publish 推进版本状态：draft→shared、draft→frozen、shared→frozen。
// frozen 为终态发布：事件集固化，不再接受任何修改。
func (s *Service) Publish(ctx context.Context, versionID, target string) (*model.Version, error) {
	v, err := s.versions.Get(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if !v.CanTransition(target) {
		return nil, model.ErrStateMachine
	}
	now := store.Now()
	if err := s.versions.UpdateStatus(ctx, versionID, target, now); err != nil {
		return nil, err
	}
	if target == model.VersionFrozen {
		if err := s.versions.MarkFrozen(ctx, versionID, now); err != nil {
			return nil, err
		}
	}
	return s.versions.Get(ctx, versionID)
}

// Derive 从冻结版本派生新草稿版本：继承事件集，旧版本标记 superseded。
func (s *Service) Derive(ctx context.Context, versionID, note string) (*model.Version, error) {
	v, err := s.versions.Get(ctx, versionID)
	if err != nil {
		return nil, err
	}
	if v.Status != model.VersionFrozen {
		return nil, errors.Join(model.ErrStateMachine, errors.New("derive requires a frozen version"))
	}
	eventIDs, err := s.versions.EventIDs(ctx, versionID)
	if err != nil {
		return nil, err
	}
	rev, err := s.versions.NextRev(ctx, v.BatchID)
	if err != nil {
		return nil, err
	}
	now := store.Now()
	nv := model.NewVersion(store.NewID(), v.BatchID, rev, note, now)
	nv.EventCount = len(eventIDs)
	if nv.FrameCount, err = s.versions.FrameCount(ctx, v.BatchID); err != nil {
		return nil, err
	}
	if err := s.versions.Create(ctx, nv, eventIDs); err != nil {
		return nil, err
	}
	if err := s.versions.MarkSuperseded(ctx, versionID, nv.ID); err != nil {
		return nil, err
	}
	if err := s.batches.IncrVersionCount(ctx, nil, v.BatchID); err != nil {
		return nil, err
	}
	return nv, nil
}

// List 返回批次内全部版本（rev 降序）。
func (s *Service) List(ctx context.Context, batchID string) ([]*model.Version, error) {
	return s.versions.ListByBatch(ctx, batchID)
}

// Get 按 ID 读取版本。
func (s *Service) Get(ctx context.Context, versionID string) (*model.Version, error) {
	return s.versions.Get(ctx, versionID)
}

// Events 返回版本内固化的谱系事件列表。
func (s *Service) Events(ctx context.Context, versionID string) ([]*model.LineageEvent, error) {
	eventIDs, err := s.versions.EventIDs(ctx, versionID)
	if err != nil {
		return nil, err
	}
	var out []*model.LineageEvent
	for _, eid := range eventIDs {
		ev, err := s.events.Get(ctx, eid)
		if err != nil {
			if errors.Is(err, model.ErrNotFound) {
				continue // 版本内引用的事件被清理时的防御
			}
			return nil, err
		}
		out = append(out, ev)
	}
	return out, nil
}

// HasFrozen 判断批次是否存在冻结版本（发布前置条件）。
func (s *Service) HasFrozen(ctx context.Context, batchID string) (bool, error) {
	_, err := s.versions.FrozenVersion(ctx, batchID)
	if errors.Is(err, model.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}
