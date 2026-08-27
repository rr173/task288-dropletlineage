package model

// Observation 是一个液滴在某帧中的观测：位置、半径（由半径可推体积）、
// 荧光标记强度与所属通道坐标。同一 dropletKey 跨帧出现即构成一条轨迹。
type Observation struct {
	ID         string  `json:"id"`
	BatchID    string  `json:"batch_id"`
	FrameID    string  `json:"frame_id"`
	FrameSeq   int     `json:"frame_seq"`
	DropletKey string  `json:"droplet_key"` // 液滴 ID，批次内唯一
	X          float64 `json:"x"`           // 通道内 x 坐标（0..ChamberWidth）
	Y          float64 `json:"y"`           // 通道内 y 坐标（0..ChamberHeight）
	Radius     float64 `json:"radius"`      // 液滴半径（像素/微米）
	Volume     float64 `json:"volume"`      // 由半径派生的球体积（缓存）
	Intensity  float64 `json:"intensity"`   // 荧光标记强度
	MarkerID   string  `json:"marker_id"`   // 试剂标记类型（如 GFP / RFP / 无）
	Status     string  `json:"status"`      // raw / tracked / occluded / excluded
	CreatedAt  string  `json:"created_at"`
}

// 观测状态机：raw → tracked（跨帧关联成功）；raw/tracked → occluded（遮挡候选）；
// 任意 → excluded（研究者排除误检）。被排除的观测不参与守恒校验。
const (
	ObsRaw      = "raw"
	ObsTracked  = "tracked"
	ObsOccluded = "occluded"
	ObsExcluded = "excluded"
)

// NewObservation 构造一条原始观测，并据半径缓存球体积。
func NewObservation(id, batchID, frameID string, frameSeq int, dropletKey string,
	x, y, radius, intensity float64, markerID, now string) *Observation {
	return &Observation{
		ID: id, BatchID: batchID, FrameID: frameID, FrameSeq: frameSeq,
		DropletKey: dropletKey, X: x, Y: y, Radius: radius,
		Volume: SphereVolume(radius), Intensity: intensity,
		MarkerID: markerID, Status: ObsRaw, CreatedAt: now,
	}
}

// IsEligibleForConservation 判断观测是否参与守恒校验（排除误检后才有意义）。
func (o *Observation) IsEligibleForConservation() bool {
	return o.Status != ObsExcluded
}
