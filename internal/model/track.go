package model

// Track 是一条液滴跨帧轨迹：由同一 dropletKey 在一系列连续帧中的观测组成。
// 若轨迹在某帧断裂（上一帧有、下一帧无匹配），则 state 置为 broken，
// 并生成遮挡候选供研究者裁决。
type Track struct {
	ID            string `json:"id"`
	BatchID       string `json:"batch_id"`
	DropletKey    string `json:"droplet_key"`
	StartFrameSeq int    `json:"start_frame_seq"`
	EndFrameSeq   int    `json:"end_frame_seq"`
	ObsCount      int    `json:"obs_count"`
	State         string `json:"state"` // active / broken
	BrokenAtSeq   int    `json:"broken_at_seq"` // 0 表示未断裂
	CreatedAt     string `json:"created_at"`
}

// TrackLink 记录轨迹中相邻两帧观测的关联边（匹配依据：距离与速度外推）。
type TrackLink struct {
	ID           string  `json:"id"`
	TrackID      string  `json:"track_id"`
	FromObsID    string  `json:"from_obs_id"`
	ToObsID      string  `json:"to_obs_id"`
	FromSeq      int     `json:"from_seq"`
	ToSeq        int     `json:"to_seq"`
	Distance     float64 `json:"distance"`
	MatchType    string  `json:"match_type"` // nearest / extrapolated
	Extrapolation float64 `json:"extrapolation"` // 0 表示未用外推
	CreatedAt    string  `json:"created_at"`
}

// 轨迹状态。
const (
	TrackActive = "active"
	TrackBroken = "broken"
)
