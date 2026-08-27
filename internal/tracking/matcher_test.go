package tracking

import (
	"testing"

	"task288-dropletlineage/internal/model"
)

func obs(id, key string, seq int, x, y float64) *model.Observation {
	return &model.Observation{ID: id, DropletKey: key, FrameSeq: seq, X: x, Y: y}
}

func TestMatcherNearestAndExtrapolation(t *testing.T) {
	m := DefaultMatcher() // MaxGap=2, MaxJumpDist=60, ExtrapTolerance=30
	byFrame := map[int][]*model.Observation{
		1: {obs("o1", "D", 1, 100, 100)},
		2: {obs("o2", "D", 2, 120, 100)},   // 最近邻
		4: {obs("o3", "D", 4, 150, 100)},   // 缺帧3，外推匹配（位移 30 <= 30）
	}
	res := m.Build("batch", byFrame, []int{1, 2, 4})
	if len(res.Tracks) != 1 {
		t.Fatalf("tracks = %d, want 1", len(res.Tracks))
	}
	track := res.Tracks[0]
	if track.State != model.TrackActive || track.BrokenAtSeq != 0 {
		t.Fatalf("track state = %s broken_at=%d, want active/0", track.State, track.BrokenAtSeq)
	}
	if len(res.Links) != 2 {
		t.Fatalf("links = %d, want 2", len(res.Links))
	}
	// 第二条边应为外推匹配
	if res.Links[1].MatchType != "extrapolated" || res.Links[1].Extrapolation != 1 {
		t.Fatalf("link[1] = %s ext=%.0f, want extrapolated/1", res.Links[1].MatchType, res.Links[1].Extrapolation)
	}
	if len(res.SingleFrame) != 0 {
		t.Fatalf("single frames = %d, want 0", len(res.SingleFrame))
	}
}

func TestMatcherOcclusionBreak(t *testing.T) {
	m := DefaultMatcher()
	// D 在帧1、2 正常，帧3 缺失，帧4 远处重现 → 断裂
	byFrame := map[int][]*model.Observation{
		1: {obs("o1", "D", 1, 100, 100)},
		2: {obs("o2", "D", 2, 120, 100)},
		4: {obs("o3", "D", 4, 400, 300)}, // 距离 ≈ 344 > 30
	}
	res := m.Build("batch", byFrame, []int{1, 2, 4})
	if len(res.Tracks) != 1 {
		t.Fatalf("tracks = %d, want 1", len(res.Tracks))
	}
	track := res.Tracks[0]
	if track.State != model.TrackBroken || track.BrokenAtSeq != 3 {
		t.Fatalf("track state = %s broken_at=%d, want broken/3", track.State, track.BrokenAtSeq)
	}
	if len(res.Hints) != 1 || res.Hints[0].DropletKey != "D" || res.Hints[0].BrokenAtSeq != 3 {
		t.Fatalf("hints = %+v, want single hint D@3", res.Hints)
	}
	if !res.OccludedObs["o2"] || !res.OccludedObs["o3"] {
		t.Fatalf("o2/o3 not marked occluded")
	}
}

func TestMatcherExceedsJumpDistance(t *testing.T) {
	m := DefaultMatcher()
	// 相邻帧位移 100 > MaxJumpDist 60 → 断裂
	byFrame := map[int][]*model.Observation{
		1: {obs("o1", "D", 1, 100, 100)},
		2: {obs("o2", "D", 2, 200, 100)},
	}
	res := m.Build("batch", byFrame, []int{1, 2})
	if res.Tracks[0].State != model.TrackBroken {
		t.Fatalf("state = %s, want broken", res.Tracks[0].State)
	}
	if len(res.Links) != 0 {
		t.Fatalf("links = %d, want 0", len(res.Links))
	}
}

func TestMatcherSingleFrameDroplet(t *testing.T) {
	m := DefaultMatcher()
	byFrame := map[int][]*model.Observation{
		3: {obs("o1", "S", 3, 50, 50)},
	}
	res := m.Build("batch", byFrame, []int{3})
	if len(res.Tracks) != 1 || res.Tracks[0].ObsCount != 1 {
		t.Fatalf("tracks = %+v, want 1 single-obs track", res.Tracks)
	}
	if !res.SingleFrame["o1"] {
		t.Fatalf("o1 not in single frame set")
	}
}
