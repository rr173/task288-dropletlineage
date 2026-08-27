// Package service 是应用编排层：聚合各业务包，承载批次状态机联动
// 与跨模块的端到端流程（导入→跟踪→守恒→裁决→版本发布）。
package service

import (
	"context"
	"errors"

	"task288-dropletlineage/internal/conservation"
	"task288-dropletlineage/internal/ingest"
	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
	"task288-dropletlineage/internal/tracking"
	"task288-dropletlineage/internal/verdict"
	"task288-dropletlineage/internal/version"
)

// App 聚合全部存储与业务服务，HTTP 层只与 App 交互。
type App struct {
	DB           *store.DB
	Batches      *store.BatchStore
	Frames       *store.FrameStore
	Obs          *store.ObservationStore
	Tracks       *store.TrackStore
	Events       *store.EventStore
	Versions     *store.VersionStore
	Ingest       *ingest.Service
	Tracking     *tracking.Service
	Conservation *conservation.Service
	Verdict      *verdict.Service
	Version      *version.Service
}

// New 构造应用聚合。
func New(db *store.DB) *App {
	batches := store.NewBatchStore(db)
	frames := store.NewFrameStore(db)
	obs := store.NewObservationStore(db)
	tracks := store.NewTrackStore(db)
	events := store.NewEventStore(db)
	versions := store.NewVersionStore(db)
	return &App{
		DB: db, Batches: batches, Frames: frames, Obs: obs,
		Tracks: tracks, Events: events, Versions: versions,
		Ingest:       ingest.NewService(db, batches, frames, obs),
		Tracking:     tracking.NewService(db, obs, tracks),
		Conservation: conservation.NewService(db, events),
		Verdict:      verdict.NewService(db, events, batches),
		Version:      version.NewService(db, versions, events, batches),
	}
}

// CreateBatch 创建新批次（importing）。
func (a *App) CreateBatch(ctx context.Context, name string, width, height float64) (*model.Batch, error) {
	if name == "" || width <= 0 || height <= 0 {
		return nil, errors.Join(model.ErrInvalid, errors.New("name and positive chamber size required"))
	}
	now := store.Now()
	b := model.NewBatch(store.NewID(), name, width, height, now)
	if err := a.Batches.Create(ctx, b); err != nil {
		return nil, err
	}
	return b, nil
}

// ListBatches 返回全部批次。
func (a *App) ListBatches(ctx context.Context) ([]*model.Batch, error) {
	return a.Batches.List(ctx)
}

// GetBatch 按 ID 读取批次。
func (a *App) GetBatch(ctx context.Context, id string) (*model.Batch, error) {
	return a.Batches.Get(ctx, id)
}

// TransitionBatch 推进批次状态机。
// 前置约束：review→published 必须已有冻结谱系版本。
func (a *App) TransitionBatch(ctx context.Context, id, to string) (*model.Batch, error) {
	b, err := a.Batches.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if !model.CanTransition(b.Status, to) {
		return nil, model.ErrStateMachine
	}
	if to == model.BatchPublished {
		has, err := a.Version.HasFrozen(ctx, id)
		if err != nil {
			return nil, err
		}
		if !has {
			return nil, model.ErrNoFrozenVersion
		}
	}
	if err := a.Batches.UpdateStatus(ctx, nil, id, to, store.Now()); err != nil {
		return nil, err
	}
	return a.Batches.Get(ctx, id)
}

// RunTracking 执行跨帧跟踪，并把新产生的遮挡断裂自动申报为候选事件。
func (a *App) RunTracking(ctx context.Context, batchID string) (*tracking.Result, error) {
	res, err := a.Tracking.Run(ctx, batchID, tracking.DefaultMatcher())
	if err != nil {
		return nil, err
	}
	if err := a.syncOcclusionEvents(ctx, batchID, res.Hints); err != nil {
		return nil, err
	}
	return res, nil
}

// syncOcclusionEvents 把遮挡线索转为候选事件；同液滴已有开放候选时不重复建。
func (a *App) syncOcclusionEvents(ctx context.Context, batchID string, hints []tracking.OcclusionHint) error {
	events, err := a.Events.ListByBatch(ctx, batchID)
	if err != nil {
		return err
	}
	openKeys := map[string]bool{}
	for _, e := range events {
		if e.Kind == "occlusion" && !e.IsClosed() {
			parts, err := a.Events.Participants(ctx, e.ID)
			if err != nil {
				return err
			}
			for _, p := range parts {
				openKeys[p.DropletKey] = true
			}
		}
	}
	for _, h := range hints {
		if openKeys[h.DropletKey] {
			continue
		}
		ev := h.ToEvent(batchID, conservation.DefaultTolerance, store.Now())
		var parts []model.EventParticipant
		if h.PrevObs != nil {
			parts = append(parts, participantOf(ev.ID, h.PrevObs, "parent"))
		}
		if h.NextObs != nil {
			parts = append(parts, participantOf(ev.ID, h.NextObs, "child"))
		}
		if err := a.Events.Create(ctx, ev, parts); err != nil {
			return err
		}
	}
	return nil
}

func participantOf(eventID string, o *model.Observation, role string) model.EventParticipant {
	return model.EventParticipant{
		EventID: eventID, ObsID: o.ID, DropletKey: o.DropletKey, Role: role,
		FrameSeq: o.FrameSeq, Volume: o.Volume, Intensity: o.Intensity,
	}
}

// Stats 是批次汇总视图。
type Stats struct {
	BatchID     string `json:"batch_id"`
	Status      string `json:"status"`
	FrameCount  int    `json:"frame_count"`
	ObsCount    int    `json:"obs_count"`
	TrackCount  int    `json:"track_count"`
	BrokenTracks int   `json:"broken_tracks"`
	EventCount  int    `json:"event_count"`
	VersionCount int   `json:"version_count"`
}

// BatchStats 汇总批次各实体数量。
func (a *App) BatchStats(ctx context.Context, batchID string) (*Stats, error) {
	b, err := a.Batches.Get(ctx, batchID)
	if err != nil {
		return nil, err
	}
	tracks, err := a.Tracks.ListByBatch(ctx, batchID)
	if err != nil {
		return nil, err
	}
	broken := 0
	for _, t := range tracks {
		if t.State == model.TrackBroken {
			broken++
		}
	}
	return &Stats{
		BatchID: b.ID, Status: b.Status,
		FrameCount: b.FrameCount, ObsCount: b.ObsCount,
		TrackCount: len(tracks), BrokenTracks: broken,
		EventCount: b.EventCount, VersionCount: b.VersionCount,
	}, nil
}
