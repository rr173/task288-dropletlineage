package model

// Batch 是微流控液滴谱系复核的一个实验流程批次。
// 它承载一组帧事件、液滴观测、谱系事件与谱系版本，拥有独立状态机。
type Batch struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ChamberWidth  float64 `json:"chamber_width"`
	ChamberHeight float64 `json:"chamber_height"`
	Status        string `json:"status"`
	FrameCount    int    `json:"frame_count"`
	ObsCount      int    `json:"obs_count"`
	EventCount    int    `json:"event_count"`
	VersionCount  int    `json:"version_count"`
	CreatedAt     string `json:"created_at"`
	UpdatedAt     string `json:"updated_at"`
}

// 批次状态机：importing → tracking → review → published → archived。
// review 可回退到 tracking 以重跑跟踪；published 之后只能封存。
const (
	BatchImporting = "importing"
	BatchTracking  = "tracking"
	BatchReview    = "review"
	BatchPublished = "published"
	BatchArchived  = "archived"
)

// ValidBatchTransitions 描述批次允许的状态转移集合。
var ValidBatchTransitions = map[string]map[string]bool{
	BatchImporting: {BatchTracking: true},
	BatchTracking:  {BatchReview: true},
	BatchReview:    {BatchTracking: true, BatchPublished: true},
	BatchPublished: {BatchArchived: true},
	BatchArchived:  {},
}

// CanTransition 判断 from → to 是否为合法批次状态转移。
func CanTransition(from, to string) bool {
	allowed, ok := ValidBatchTransitions[from]
	return ok && allowed[to]
}

// NewBatch 构造一个处于 importing 状态的批次。
func NewBatch(id, name string, width, height float64, now string) *Batch {
	return &Batch{
		ID: id, Name: name, Status: BatchImporting,
		ChamberWidth: width, ChamberHeight: height,
		CreatedAt: now, UpdatedAt: now,
	}
}
