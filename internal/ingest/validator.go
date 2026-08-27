// Package ingest 负责帧事件与液滴观测的导入：
// 校验帧序单调、通道坐标越界、液滴 ID 冲突，并保证同序号导入幂等。
package ingest

import (
	"errors"
	"math"

	"task288-dropletlineage/internal/model"
)

// Validator 封装导入前的业务约束校验，需要批次几何信息。
type Validator struct {
	Width  float64
	Height float64
}

// NewValidator 用批次通道尺寸构造校验器。
func NewValidator(width, height float64) *Validator {
	return &Validator{Width: width, Height: height}
}

// ValidateCoordinate 校验通道坐标不越界（半开区间 [0, W) × [0, H)）。
func (v *Validator) ValidateCoordinate(x, y float64) error {
	if math.IsNaN(x) || math.IsNaN(y) || math.IsInf(x, 0) || math.IsInf(y, 0) {
		return model.ErrCoordOutOfRange
	}
	if x < 0 || x >= v.Width || y < 0 || y >= v.Height {
		return model.ErrCoordOutOfRange
	}
	return nil
}

// ValidateObservation 校验观测字段：坐标范围、半径与强度的物理合理性。
func (v *Validator) ValidateObservation(o *model.Observation) error {
	if o.DropletKey == "" {
		return errors.Join(model.ErrInvalid, errors.New("droplet_key required"))
	}
	if err := v.ValidateCoordinate(o.X, o.Y); err != nil {
		return err
	}
	if o.Radius <= 0 || math.IsNaN(o.Radius) || math.IsInf(o.Radius, 0) {
		return errors.Join(model.ErrInvalid, errors.New("radius must be positive finite"))
	}
	if o.Intensity < 0 || math.IsNaN(o.Intensity) || math.IsInf(o.Intensity, 0) {
		return errors.Join(model.ErrInvalid, errors.New("intensity must be non-negative finite"))
	}
	if o.MarkerID == "" {
		return errors.Join(model.ErrInvalid, errors.New("marker_id required"))
	}
	return nil
}

// ValidateFrame 校验帧字段：时间戳与通道。
func ValidateFrame(seq int, tMs int64, channel string) error {
	if seq < 1 {
		return errors.Join(model.ErrInvalid, errors.New("frame seq must be >= 1"))
	}
	if tMs < 0 {
		return errors.Join(model.ErrInvalid, errors.New("t_ms must be non-negative"))
	}
	if channel == "" {
		return errors.Join(model.ErrInvalid, errors.New("channel required"))
	}
	return nil
}
