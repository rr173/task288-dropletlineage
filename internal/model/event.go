package model

// LineageEvent 是一个谱系事件：液滴分裂、液滴合并或遮挡候选。
// 事件申报后立即执行守恒校验，保存 conservation 检查结果；
// 研究者可确认或否决。已确认的事件进入谱系版本。
type LineageEvent struct {
	ID         string  `json:"id"`
	BatchID    string  `json:"batch_id"`
	Kind       string  `json:"kind"` // split / merge / occlusion
	Status     string  `json:"status"`
	Tolerance  float64 `json:"tolerance"` // 守恒相对容差（默认 0.05）
	Reason     string  `json:"reason"`
	CreatedAt  string  `json:"created_at"`
	DecidedAt  string  `json:"decided_at"`
}

// 谱系事件状态机：
// candidate → split | merge（守恒通过自动定型）| conservation_conflict（守恒失败）
// 任意状态 → confirmed / rejected（研究者裁决，终态）。
const (
	EventCandidate             = "candidate"
	EventSplit                 = "split"
	EventMerge                 = "merge"
	EventConservationConflict  = "conservation_conflict"
	EventConfirmed             = "confirmed"
	EventRejected              = "rejected"
)

// EventParticipant 记录谱系事件中的参与者观测及其角色。
type EventParticipant struct {
	EventID   string  `json:"event_id"`
	ObsID     string  `json:"obs_id"`
	DropletKey string  `json:"droplet_key"`
	Role      string  `json:"role"` // parent / child
	FrameSeq  int     `json:"frame_seq"`
	Volume    float64 `json:"volume"`
	Intensity float64 `json:"intensity"`
}

// ConservationCheck 是一次守恒校验的结果：期望守恒量 vs 实际守恒量。
type ConservationCheck struct {
	ID            string  `json:"id"`
	EventID       string  `json:"event_id"`
	Kind          string  `json:"kind"` // volume / marker
	ExpectedValue float64 `json:"expected_value"`
	ActualValue   float64 `json:"actual_value"`
	Delta         float64 `json:"delta"`
	Tolerance     float64 `json:"tolerance"`
	Pass          bool    `json:"pass"`
	Comment       string  `json:"comment"`
	CreatedAt     string  `json:"created_at"`
}

// IsClosed 判断事件是否处于可裁决的开放状态。
func (e *LineageEvent) IsClosed() bool {
	return e.Status == EventConfirmed || e.Status == EventRejected
}

// NewLineageEvent 构造候选谱系事件。
func NewLineageEvent(id, batchID, kind string, tolerance float64, reason, now string) *LineageEvent {
	return &LineageEvent{
		ID: id, BatchID: batchID, Kind: kind,
		Status: EventCandidate, Tolerance: tolerance,
		Reason: reason, CreatedAt: now,
	}
}
