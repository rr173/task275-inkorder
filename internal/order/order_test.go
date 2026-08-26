package order

import (
	"testing"

	"task275-inkorder/internal/model"
)

func TestDiGraphCycleDetection(t *testing.T) {
	g := NewDiGraph()
	g.AddEdge(Edge{Before: 1, After: 2, Weight: 0.8})
	g.AddEdge(Edge{Before: 2, After: 3, Weight: 0.8})
	if cyc := g.Cycle(); len(cyc) != 0 {
		t.Fatalf("unexpected cycle: %v", cyc)
	}
	g.AddEdge(Edge{Before: 3, After: 1, Weight: 0.5})
	if cyc := g.Cycle(); len(cyc) == 0 {
		t.Fatal("expected cycle detection")
	}
	if _, ok := g.TopoOrder(); ok {
		t.Fatal("TopoOrder should fail on cyclic graph")
	}
}

func TestDiGraphTopoOrder(t *testing.T) {
	g := NewDiGraph()
	g.AddEdge(Edge{Before: 1, After: 2, Weight: 0.9})
	g.AddEdge(Edge{Before: 2, After: 3, Weight: 0.9})
	order, ok := g.TopoOrder()
	if !ok {
		t.Fatal("topo order failed")
	}
	pos := make(map[int64]int)
	for i, n := range order {
		pos[n] = i
	}
	if pos[1] >= pos[2] || pos[2] >= pos[3] {
		t.Errorf("order violates edges: %v", order)
	}
}

func TestBuilderConsistentChain(t *testing.T) {
	fragments := []model.Fragment{
		{ID: 1, Status: model.FragmentCalibrated},
		{ID: 2, Status: model.FragmentCalibrated},
		{ID: 3, Status: model.FragmentCalibrated},
	}
	crossings := []model.Crossing{
		{FirstFragmentID: 1, SecondFragmentID: 2, Confidence: 0.85},
		{FirstFragmentID: 2, SecondFragmentID: 3, Confidence: 0.80},
	}
	res := (Builder{}).Build(fragments, crossings, nil)
	if res.HasCycle {
		t.Fatalf("unexpected cycle: %v", res.Cycle)
	}
	// 1 必须先于 3
	pos := map[int64]int{}
	for i, id := range res.Order {
		pos[id] = i
	}
	if pos[1] >= pos[3] {
		t.Errorf("expected 1 before 3, order=%v", res.Order)
	}
	if len(res.Edges) < 2 {
		t.Errorf("expected >=2 edges, got %d", len(res.Edges))
	}
}

func TestBuilderConflictOnCycle(t *testing.T) {
	fragments := []model.Fragment{
		{ID: 1, Status: model.FragmentCalibrated},
		{ID: 2, Status: model.FragmentCalibrated},
	}
	crossings := []model.Crossing{
		{FirstFragmentID: 1, SecondFragmentID: 2, Confidence: 0.8},
		{FirstFragmentID: 2, SecondFragmentID: 1, Confidence: 0.8},
	}
	res := (Builder{}).Build(fragments, crossings, nil)
	if !res.HasCycle {
		t.Fatal("expected cycle for contradictory crossings")
	}
}

func TestValidatorRejectsSelfLoop(t *testing.T) {
	cand := &model.OrderCandidate{Status: model.CandGenerated}
	edges := []model.CandidateEdge{
		{BeforeFragmentID: 1, AfterFragmentID: 1, Source: model.EdgeFromCrossing},
	}
	fragments := []model.Fragment{{ID: 1, Status: model.FragmentCalibrated}}
	res := (Validator{}).Validate(cand, edges, fragments)
	if res.OK {
		t.Error("self-loop edge should fail validation")
	}
}

func TestConsistencyCounting(t *testing.T) {
	a := []int64{1, 2, 3}
	b := []int64{3, 2, 1}
	if n := Consistency(a, b); n != 3 {
		t.Errorf("expected 3 inversions, got %d", n)
	}
	if n := Consistency(a, []int64{1, 2, 3}); n != 0 {
		t.Errorf("expected 0 inversions, got %d", n)
	}
}
