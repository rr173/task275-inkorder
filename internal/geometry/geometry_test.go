package geometry

import (
	"math"
	"testing"

	"task275-inkorder/internal/model"
)

func TestIntersection(t *testing.T) {
	cases := []struct {
		name string
		s1   Segment
		s2   Segment
		want bool
	}{
		{"cross", Segment{Point{0, 0}, Point{10, 10}}, Segment{Point{0, 10}, Point{10, 0}}, true},
		{"parallel", Segment{Point{0, 0}, Point{10, 0}}, Segment{Point{0, 5}, Point{10, 5}}, false},
		{"touch", Segment{Point{0, 0}, Point{10, 0}}, Segment{Point{5, 0}, Point{15, 0}}, false},
		{"disjoint", Segment{Point{0, 0}, Point{1, 1}}, Segment{Point{5, 5}, Point{6, 6}}, false},
	}
	for _, c := range cases {
		p, ok := Intersection(c.s1, c.s2)
		if ok != c.want {
			t.Errorf("%s: got ok=%v want %v (p=%+v)", c.name, ok, c.want, p)
		}
	}
}

func TestEstimateRuler(t *testing.T) {
	base := []Point{{0, 0}, {100, 0}}
	layer := []Point{{12, -8}, {117, -8}}
	r := EstimateRuler(base, layer)
	if math.Abs(r.Scale-1.05) > 1e-6 {
		t.Errorf("scale = %v, want ~1.05", r.Scale)
	}
	// 反变换回基准
	bx, by := r.ApplyLayerToBase(12, -8)
	if math.Abs(bx) > 1e-3 || math.Abs(by) > 1e-3 {
		t.Errorf("back-transform = (%v,%v), want (0,0)", bx, by)
	}
}

func TestCrossJudge(t *testing.T) {
	// B 垂直穿过 A，B 笔压更高 → B 覆盖 A（B 后写）
	fA := &model.Fragment{CalibStartX: 0, CalibStartY: 0, CalibEndX: 100, CalibEndY: 0, Pressure: 0.30}
	fB := &model.Fragment{CalibStartX: 50, CalibStartY: -20, CalibEndX: 50, CalibEndY: 80, Pressure: 0.62}
	ev := NewCrossJudge().Judge(fA, fB)
	if !ev.Intersect {
		t.Fatal("expected intersection")
	}
	if !ev.Suggestion {
		t.Error("expected suggestion: B after A (B covers A)")
	}
	if math.Abs(ev.X-50) > 1 || math.Abs(ev.Y) > 1 {
		t.Errorf("intersection at (%v,%v), want (50,0)", ev.X, ev.Y)
	}
	if ev.IsArtifact {
		t.Errorf("unexpected artifact: %s", ev.ArtifactWhy)
	}
}

func TestCrossJudgeArtifactOnParallel(t *testing.T) {
	// 两笔画近似平行（夹角约 1.1°）但确实相交于 (60,6)
	fA := &model.Fragment{CalibStartX: 0, CalibStartY: 0, CalibEndX: 100, CalibEndY: 10, Pressure: 0.5}
	fB := &model.Fragment{CalibStartX: 10, CalibStartY: 0, CalibEndX: 110, CalibEndY: 12, Pressure: 0.5}
	ev := NewCrossJudge().Judge(fA, fB)
	if !ev.Intersect {
		t.Fatal("expected intersection for near-parallel crossing")
	}
	if !ev.IsArtifact {
		t.Error("expected artifact flag for near-parallel strokes")
	}
}
