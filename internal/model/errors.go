// Package model 定义微流控液滴分裂谱系复核台的领域实体、状态机与错误类型。
package model

import "errors"

// 领域错误统一命名空间，HTTP 层据此映射状态码：
// ErrNotFound→404、ErrConflict→409、ErrInvalid→422、ErrStateMachine→409。
var (
	// ErrNotFound 表示目标实体不存在。
	ErrNotFound = errors.New("not found")
	// ErrConflict 表示乐观并发或唯一约束冲突。
	ErrConflict = errors.New("conflict")
	// ErrInvalid 表示输入违反业务约束（坐标越界、帧序倒退、液滴 ID 冲突等）。
	ErrInvalid = errors.New("invalid input")
	// ErrStateMachine 表示状态机转移非法。
	ErrStateMachine = errors.New("illegal state transition")
	// ErrFrozen 表示对已冻结谱系版本执行修改操作。
	ErrFrozen = errors.New("version is frozen")
	// ErrSequenceRegression 表示帧序号倒退（不变量：帧序单调递增）。
	ErrSequenceRegression = errors.New("frame sequence regression")
	// ErrCoordOutOfRange 表示通道坐标越界。
	ErrCoordOutOfRange = errors.New("coordinate out of chamber range")
	// ErrDropletIDDuplicate 表示同一批次内液滴 ID 冲突。
	ErrDropletIDDuplicate = errors.New("duplicate droplet id in batch")
	// ErrEventClosed 表示对已确认/否决的谱系事件再次裁决。
	ErrEventClosed = errors.New("event already closed")
	// ErrNoFrozenVersion 表示发布批次前缺少冻结版本。
	ErrNoFrozenVersion = errors.New("batch requires a frozen version before publish")
)
