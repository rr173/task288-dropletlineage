package model

import (
	"math"
	"testing"
)

func TestSphereVolume(t *testing.T) {
	// V = 4/3·π·r³，r=1 → 4.18879...
	v := SphereVolume(1)
	if math.Abs(v-4.1887902047863905) > 1e-9 {
		t.Fatalf("SphereVolume(1) = %v, want 4.1887902047863905", v)
	}
	// 半径 0 → 0
	if SphereVolume(0) != 0 {
		t.Fatalf("SphereVolume(0) = %v, want 0", SphereVolume(0))
	}
	// 半径翻倍 → 体积 ×8
	if math.Abs(SphereVolume(2)-8*SphereVolume(1)) > 1e-9 {
		t.Fatalf("SphereVolume(2) != 8*SphereVolume(1)")
	}
}

func TestCheckVolumeConservation(t *testing.T) {
	cases := []struct {
		name      string
		expected  float64
		actual    float64
		tolerance float64
		wantPass  bool
	}{
		{"exact", 100, 100, 0.05, true},
		{"within tolerance", 100, 104.9, 0.05, true},
		{"at boundary", 100, 105, 0.05, true},
		{"beyond tolerance", 100, 105.1, 0.05, false},
		{"zero expected rejected", 0, 10, 0.05, false},
	}
	for _, c := range cases {
		r := CheckVolumeConservation(c.expected, c.actual, c.tolerance)
		if r.Pass != c.wantPass {
			t.Errorf("%s: pass=%v want=%v (rel_err=%.4f)", c.name, r.Pass, c.wantPass, r.RelErr)
		}
	}
}

func TestCheckMarkerConservation(t *testing.T) {
	// expected=父代强度 100，actual=子代之和
	cases := []struct {
		name     string
		actual   float64
		quench   float64
		wantPass bool
	}{
		{"full conservation", 100, 0.8, true},
		{"5% loss allowed", 95, 0.8, true},
		{"quench floor ok", 80, 0.8, true},       // 100×0.8=80，允许下限
		{"too much loss", 75, 0.8, false},        // 低于 80×0.95=76
		{"marker created from nothing", 120, 0.8, false}, // 高于 100×1.05
	}
	for _, c := range cases {
		r := CheckMarkerConservation(100, c.actual, 0.05, c.quench)
		if r.Pass != c.wantPass {
			t.Errorf("%s: pass=%v want=%v (delta=%.2f)", c.name, r.Pass, c.wantPass, r.Delta)
		}
	}
}

func TestSumVolumesAndIntensities(t *testing.T) {
	obs := []Observation{
		{Radius: 1, Volume: SphereVolume(1), Intensity: 10},
		{Radius: 2, Volume: SphereVolume(2), Intensity: 20},
	}
	wantV := SphereVolume(1) + SphereVolume(2)
	if SumVolumes(obs) != wantV {
		t.Fatalf("SumVolumes = %v, want %v", SumVolumes(obs), wantV)
	}
	if SumIntensities(obs) != 30 {
		t.Fatalf("SumIntensities = %v, want 30", SumIntensities(obs))
	}
}
