package model

// Frame 表示高速成像中的一帧事件：记录拍摄序号、相对时间与所属通道。
// 帧是液滴观测的容器，观测序号（seq）在同一批次内必须单调递增。
type Frame struct {
	ID        string `json:"id"`
	BatchID   string `json:"batch_id"`
	Seq       int    `json:"seq"`
	TMs       int64  `json:"t_ms"`
	Channel   string `json:"channel"`
	ImageRef  string `json:"image_ref"`
	ObsCount  int    `json:"obs_count"`
	CreatedAt string `json:"created_at"`
}

// NewFrame 构造一帧；调用方负责先通过校验器保证 seq 不倒退。
func NewFrame(id, batchID string, seq int, tMs int64, channel, imageRef, now string) *Frame {
	return &Frame{
		ID: id, BatchID: batchID, Seq: seq, TMs: tMs,
		Channel: channel, ImageRef: imageRef, CreatedAt: now,
	}
}
