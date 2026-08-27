// Package conservation 实现液滴分裂/合并的守恒验证：
// 体积守恒（球形假设）与荧光标记强度守恒（含淬灭容差）。
// 判定口径唯一收敛在 model.CheckVolumeConservation / CheckMarkerConservation。
package conservation

import (
	"task288-dropletlineage/internal/model"
)

// 守恒判定默认参数。
const (
	// DefaultTolerance 是体积守恒的相对容差（5%）。
	DefaultTolerance = 0.05
	// DefaultQuenchFactor 是荧光标记允许的最小强度保留比（80%）。
	DefaultQuenchFactor = 0.80
)

// groupObservations 把事件参与者按角色拆为父代/子代观测。
func groupObservations(participants []model.EventParticipant) (parents, children []model.Observation) {
	for _, p := range participants {
		o := model.Observation{
			ID: p.ObsID, DropletKey: p.DropletKey, FrameSeq: p.FrameSeq,
			Volume: p.Volume, Intensity: p.Intensity,
		}
		if p.Role == "parent" {
			parents = append(parents, o)
		} else if p.Role == "child" {
			children = append(children, o)
		}
	}
	return parents, children
}

// ComputeVolumeChecks 计算一个谱系事件的体积守恒检查项。
// split：期望 = 父代体积，实际 = Σ 子代体积（单父多子）；
// merge：期望 = Σ 父代体积，实际 = 子代体积（多父单子）。
// 若角色数量不符合事件类型（如 split 无父代），返回不通过的检查并注明原因。
func ComputeVolumeChecks(kind string, parents, children []model.Observation, tolerance float64) []model.ConservationCheck {
	checks := make([]model.ConservationCheck, 0, 1)
	switch kind {
	case "split":
		if len(parents) != 1 || len(children) < 2 {
			checks = append(checks, model.ConservationCheck{
				Kind: "volume", Pass: false, Comment: "split requires 1 parent and >=2 children",
			})
			return checks
		}
		expected := parents[0].Volume
		actual := model.SumVolumes(children)
		r := model.CheckVolumeConservation(expected, actual, tolerance)
		checks = append(checks, toCheck("volume", expected, actual, tolerance, r))
	case "merge":
		if len(children) != 1 || len(parents) < 2 {
			checks = append(checks, model.ConservationCheck{
				Kind: "volume", Pass: false, Comment: "merge requires >=2 parents and 1 child",
			})
			return checks
		}
		expected := model.SumVolumes(parents)
		actual := children[0].Volume
		r := model.CheckVolumeConservation(expected, actual, tolerance)
		checks = append(checks, toCheck("volume", expected, actual, tolerance, r))
	default:
		checks = append(checks, model.ConservationCheck{
			Kind: "volume", Pass: false, Comment: "unknown event kind " + kind,
		})
	}
	return checks
}

// ComputeMarkerChecks 计算荧光标记强度守恒检查项（含淬灭容差）。
// split：期望 = 父代强度，实际 = Σ 子代强度（允许淬灭损耗）；
// merge：期望 = Σ 父代强度，实际 = 子代强度。
func ComputeMarkerChecks(kind string, parents, children []model.Observation, tolerance, quench float64) []model.ConservationCheck {
	checks := make([]model.ConservationCheck, 0, 1)
	switch kind {
	case "split":
		if len(parents) != 1 || len(children) < 2 {
			checks = append(checks, model.ConservationCheck{
				Kind: "marker", Pass: false, Comment: "split requires 1 parent and >=2 children",
			})
			return checks
		}
		expected := parents[0].Intensity
		actual := model.SumIntensities(children)
		r := model.CheckMarkerConservation(expected, actual, tolerance, quench)
		checks = append(checks, toCheck("marker", expected, actual, tolerance, r))
	case "merge":
		if len(children) != 1 || len(parents) < 2 {
			checks = append(checks, model.ConservationCheck{
				Kind: "marker", Pass: false, Comment: "merge requires >=2 parents and 1 child",
			})
			return checks
		}
		expected := model.SumIntensities(parents)
		actual := children[0].Intensity
		r := model.CheckMarkerConservation(expected, actual, tolerance, quench)
		checks = append(checks, toCheck("marker", expected, actual, tolerance, r))
	default:
		checks = append(checks, model.ConservationCheck{
			Kind: "marker", Pass: false, Comment: "unknown event kind " + kind,
		})
	}
	return checks
}

func toCheck(kind string, expected, actual, tolerance float64, r model.ConservationResult) model.ConservationCheck {
	return model.ConservationCheck{
		Kind: kind, ExpectedValue: expected, ActualValue: actual,
		Delta: r.Delta, Tolerance: tolerance, Pass: r.Pass, Comment: r.Comment,
	}
}
