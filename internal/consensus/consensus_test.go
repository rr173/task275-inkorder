package consensus

import (
	"testing"

	"task275-inkorder/internal/model"
)

func TestAggregateAgree(t *testing.T) {
	key := NewPairKey(1, 2)
	votes := []LayerVote{
		{LayerID: 1, FirstBefore: true, Confidence: 0.8},
		{LayerID: 2, FirstBefore: true, Confidence: 0.7},
	}
	res := Aggregate(key, votes)
	if res.Conflict {
		t.Fatal("expected no conflict")
	}
	if !res.FirstBefore {
		t.Error("expected direction first-before-second")
	}
	if res.ConsistentLayers != 2 || res.TotalLayers != 2 {
		t.Errorf("layer counts wrong: %d/%d", res.ConsistentLayers, res.TotalLayers)
	}
}

func TestAggregateConflict(t *testing.T) {
	votes := []LayerVote{
		{LayerID: 1, FirstBefore: true, Confidence: 0.8},
		{LayerID: 2, FirstBefore: false, Confidence: 0.8},
	}
	res := Aggregate(NewPairKey(1, 2), votes)
	if !res.Conflict {
		t.Error("expected conflict flag when layers disagree")
	}
}

func TestArtifactJudgerSingleLayer(t *testing.T) {
	j := NewArtifactJudger()
	artifact, why := j.Judge(1, 3, 0.6, false)
	if !artifact {
		t.Errorf("expected artifact for single-layer low confidence, why=%s", why)
	}
	ok, _ := j.Judge(2, 3, 0.9, false)
	if ok {
		t.Error("multi-layer high confidence should not be artifact")
	}
}

func TestConflictDetector(t *testing.T) {
	crossings := []model.Crossing{
		{ID: 1, FirstFragmentID: 1, SecondFragmentID: 2},
		{ID: 2, FirstFragmentID: 2, SecondFragmentID: 1},
		{ID: 3, FirstFragmentID: 2, SecondFragmentID: 3},
	}
	pairs := (ConflictDetector{}).Detect(crossings)
	if len(pairs) != 1 {
		t.Fatalf("expected 1 conflict pair, got %d", len(pairs))
	}
	if pairs[0].FirstFragmentID != 1 || pairs[0].SecondFragmentID != 2 {
		t.Errorf("wrong pair: %+v", pairs[0])
	}
}
