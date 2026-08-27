package tracking

import (
	"strconv"

	"task288-dropletlineage/internal/model"
	"task288-dropletlineage/internal/store"
)

// OcclusionHint 是一次跟踪断裂的摘要：液滴在 BrokenAtSeq 处消失，
// PrevObs 为消失前最后观测，NextObs 为重现后首条观测。
// 该线索驱动研究者判断：真合并/分裂被误判为断裂，还是成像遮挡。
type OcclusionHint struct {
	DropletKey  string             `json:"droplet_key"`
	BrokenAtSeq int                `json:"broken_at_seq"`
	Gap         int                `json:"gap"`
	PrevObs     *model.Observation `json:"prev_obs"`
	NextObs     *model.Observation `json:"next_obs"`
}

// ToEvent 把遮挡线索转为一条候选谱系事件（kind=occlusion），供裁决环节使用。
func (h OcclusionHint) ToEvent(batchID string, tolerance float64, now string) *model.LineageEvent {
	return model.NewLineageEvent(newEventID(), batchID, "occlusion", tolerance, h.describe(), now)
}

// describe 生成遮挡事件的人类可读理由。
func (h OcclusionHint) describe() string {
	return "droplet " + h.DropletKey + " lost at frame " + strconv.Itoa(h.BrokenAtSeq) +
		" (gap=" + strconv.Itoa(h.Gap) + "), reappears at frame " + strconv.Itoa(h.NextObs.FrameSeq) +
		"; occlusion candidate"
}

func newEventID() string { return store.NewID() }
