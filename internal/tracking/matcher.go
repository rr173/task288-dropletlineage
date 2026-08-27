// Package tracking 实现液滴跨帧跟踪：
// 空间最近邻匹配 + 速度外推跨越短暂缺失帧，检测遮挡断裂并生成候选。
package tracking

import (
	"math"
	"sort"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
)

// Matcher 是跨帧关联的核心算法对象。
// 参数语义（均由调用方显式给出，保证判定口径唯一）：
//   - MaxGap：允许速度外推的最大帧间隙（缺 1 帧 = 2；缺 2 帧 = 3）；
//   - MaxJumpDist：相邻帧液滴位移超过该值即判不匹配；
//   - ExtrapTolerance：外推位置与观测位置的最大偏差。
type Matcher struct {
	MaxGap          int
	MaxJumpDist     float64
	ExtrapTolerance float64
}

// DefaultMatcher 返回一组适合微流控成像（10~100 像素级位移）的默认参数。
func DefaultMatcher() *Matcher {
	return &Matcher{MaxGap: 2, MaxJumpDist: 60, ExtrapTolerance: 30}
}

// BuildResult 是一次跟踪构建的完整产物。
type BuildResult struct {
	Tracks       []*model.Track
	Links        []*model.TrackLink
	Hints        []OcclusionHint
	LinkedObs    map[string]bool // 参与跨帧关联的观测 ID
	OccludedObs  map[string]bool // 遮挡断裂边缘的观测 ID
	SingleFrame  map[string]bool // 只在单帧出现的观测 ID（raw）
}

// Build 按帧分组观测构建全部轨迹。
// byFrame: frameSeq → 该帧观测列表（已排序）；frameOrder: 帧序升序。
func (m *Matcher) Build(batchID string, byFrame map[int][]*model.Observation, frameOrder []int) *BuildResult {
	res := &BuildResult{
		LinkedObs:   map[string]bool{},
		OccludedObs: map[string]bool{},
		SingleFrame: map[string]bool{},
	}
	if len(frameOrder) == 0 {
		return res
	}

	// 按液滴分组，观测按帧序升序
	dropletObs := map[string][]*model.Observation{}
	for _, seq := range frameOrder {
		for _, o := range byFrame[seq] {
			dropletObs[o.DropletKey] = append(dropletObs[o.DropletKey], o)
		}
	}

	for key, obsList := range dropletObs {
		sort.SliceStable(obsList, func(i, j int) bool { return obsList[i].FrameSeq < obsList[j].FrameSeq })
		if len(obsList) == 1 {
			res.SingleFrame[obsList[0].ID] = true
			res.Tracks = append(res.Tracks, &model.Track{
				ID: newTrackID(), BatchID: batchID, DropletKey: key,
				StartFrameSeq: obsList[0].FrameSeq, EndFrameSeq: obsList[0].FrameSeq,
				ObsCount: 1, State: model.TrackActive, CreatedAt: obsList[0].CreatedAt,
			})
			continue
		}
		m.buildDroplet(res, batchID, key, obsList)
	}
	sort.SliceStable(res.Tracks, func(i, j int) bool { return res.Tracks[i].DropletKey < res.Tracks[j].DropletKey })
	return res
}

// buildDroplet 构建单个液滴的轨迹与关联边。
func (m *Matcher) buildDroplet(res *BuildResult, batchID, key string, obsList []*model.Observation) {
	track := &model.Track{
		ID: newTrackID(), BatchID: batchID, DropletKey: key,
		StartFrameSeq: obsList[0].FrameSeq, EndFrameSeq: obsList[len(obsList)-1].FrameSeq,
		ObsCount: len(obsList), State: model.TrackActive, CreatedAt: obsList[0].CreatedAt,
	}
	linked := 0
	for i := 0; i+1 < len(obsList); i++ {
		a, b := obsList[i], obsList[i+1]
		gap := b.FrameSeq - a.FrameSeq
		dist := euclid(a.X, a.Y, b.X, b.Y)
		switch {
		case gap == 1 && dist <= m.MaxJumpDist:
			res.Links = append(res.Links, &model.TrackLink{
				ID: newLinkID(), TrackID: track.ID,
				FromObsID: a.ID, ToObsID: b.ID, FromSeq: a.FrameSeq, ToSeq: b.FrameSeq,
				Distance: dist, MatchType: "nearest", CreatedAt: b.CreatedAt,
			})
			res.LinkedObs[a.ID] = true
			res.LinkedObs[b.ID] = true
			linked++
		case gap <= m.MaxGap && dist <= m.ExtrapTolerance:
			// 速度外推匹配：位移仍在上限内则接受跨越缺失帧的关联
			res.Links = append(res.Links, &model.TrackLink{
				ID: newLinkID(), TrackID: track.ID,
				FromObsID: a.ID, ToObsID: b.ID, FromSeq: a.FrameSeq, ToSeq: b.FrameSeq,
				Distance: dist, MatchType: "extrapolated",
				Extrapolation: float64(gap - 1), CreatedAt: b.CreatedAt,
			})
			res.LinkedObs[a.ID] = true
			res.LinkedObs[b.ID] = true
			linked++
		default:
			// 断裂：从缺失帧起标记 broken，产出遮挡候选
			track.State = model.TrackBroken
			if track.BrokenAtSeq == 0 {
				track.BrokenAtSeq = a.FrameSeq + 1
			}
			res.OccludedObs[a.ID] = true
			res.OccludedObs[b.ID] = true
			res.Hints = append(res.Hints, OcclusionHint{
				DropletKey: key, BrokenAtSeq: a.FrameSeq + 1,
				PrevObs: a, NextObs: b, Gap: gap,
			})
		}
	}
	res.Tracks = append(res.Tracks, track)
}

// euclid 计算二维欧氏距离。
func euclid(x1, y1, x2, y2 float64) float64 {
	return math.Hypot(x2-x1, y2-y1)
}

// newTrackID / newLinkID 生成轨迹与关联边的唯一 ID。
func newTrackID() string { return store.NewID() }
func newLinkID() string  { return store.NewID() }
