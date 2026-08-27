package model

import "math"

// SphereVolume 由液滴半径（球形假设）计算体积：V = 4/3·π·r³。
func SphereVolume(radius float64) float64 {
	return (4.0 / 3.0) * math.Pi * radius * radius * radius
}

// ConservationResult 是一次守恒判定的结构化输出。
type ConservationResult struct {
	Pass    bool    `json:"pass"`
	Delta   float64 `json:"delta"`     // |expected - actual|
	RelErr  float64 `json:"rel_err"`   // |delta / expected|
	Comment string  `json:"comment"`
}

// CheckVolumeConservation 校验父代体积之和是否等于子代体积之和（相对容差内）。
// split：expected = Σ children，actual = parent；merge：expected = parent 之和，actual = child。
// 返回结构化判定结果，不依赖调用方自行比较，保证判定口径唯一。
func CheckVolumeConservation(expected, actual, tolerance float64) ConservationResult {
	if expected <= 0 {
		return ConservationResult{Pass: false, Comment: "expected volume must be positive"}
	}
	delta := math.Abs(expected - actual)
	rel := delta / expected
	return ConservationResult{
		Pass:    rel <= tolerance,
		Delta:   delta,
		RelErr:  rel,
		Comment: "volume conservation within tolerance",
	}
}

// CheckMarkerConservation 校验荧光标记强度守恒：
// expected = 分裂前父代总量 / 合并后子代总量，actual = 分割后子代之和 / 合并前父代之和。
// 允许淬灭系数 quenchFactor ∈ (0,1]：actual 允许低于 expected（光漂白损耗），
// 但不得高于 expected 的 (1+tolerance) 倍（标记不凭空增加），
// 也不得低于 expected × quenchFactor × (1-tolerance)（损耗过大视为数据异常）。
func CheckMarkerConservation(expected, actual, tolerance, quenchFactor float64) ConservationResult {
	if expected <= 0 {
		return ConservationResult{Pass: false, Comment: "expected intensity must be positive"}
	}
	lower := expected * quenchFactor
	delta := math.Abs(actual - expected)
	rel := delta / expected
	pass := actual <= expected*(1+tolerance) && actual >= lower*(1-tolerance)
	return ConservationResult{
		Pass:    pass,
		Delta:   delta,
		RelErr:  rel,
		Comment: "marker conservation with quench allowance",
	}
}

// SumVolumes 求一组观测的体积之和。
func SumVolumes(obs []Observation) float64 {
	var s float64
	for _, o := range obs {
		s += o.Volume
	}
	return s
}

// SumIntensities 求一组观测的荧光强度之和。
func SumIntensities(obs []Observation) float64 {
	var s float64
	for _, o := range obs {
		s += o.Intensity
	}
	return s
}
