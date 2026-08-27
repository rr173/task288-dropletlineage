package model

// Version 是一个谱系版本：把已确认的谱系事件固化为不可变快照。
// 状态机：draft → shared → frozen → superseded；冻结后不可增删事件，
// 只能基于它派生新版本（替代关系）。
type Version struct {
	ID          string `json:"id"`
	BatchID     string `json:"batch_id"`
	Rev         int    `json:"rev"`
	Status      string `json:"status"`
	Note        string `json:"note"`
	EventCount  int    `json:"event_count"`
	FrameCount  int    `json:"frame_count"`
	FrozenAt    string `json:"frozen_at"`
	SupersededBy string `json:"superseded_by"`
	CreatedAt   string `json:"created_at"`
	PublishedAt string `json:"published_at"`
}

// 谱系版本状态机。
const (
	VersionDraft      = "draft"
	VersionShared     = "shared"
	VersionFrozen     = "frozen"
	VersionSuperseded = "superseded"
)

// VersionEvent 是版本内固定的事件引用（版本快照的一部分）。
type VersionEvent struct {
	VersionID string `json:"version_id"`
	EventID   string `json:"event_id"`
}

// NewVersion 构造一个草稿版本。
func NewVersion(id, batchID string, rev int, note, now string) *Version {
	return &Version{
		ID: id, BatchID: batchID, Rev: rev,
		Status: VersionDraft, Note: note, CreatedAt: now,
	}
}

// CanTransition 判断谱系版本的状态转移是否合法。
func (v *Version) CanTransition(to string) bool {
	switch v.Status {
	case VersionDraft:
		return to == VersionShared || to == VersionFrozen
	case VersionShared:
		return to == VersionFrozen
	case VersionFrozen:
		return to == VersionSuperseded
	case VersionSuperseded:
		return false
	}
	return false
}
