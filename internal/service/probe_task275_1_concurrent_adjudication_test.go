package service

import (
	"context"
	"sync"
	"testing"

	"task275-inkorder/internal/model"
	"task275-inkorder/internal/store"
)

func openProbeApp(t *testing.T) *App {
	t.Helper()
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewApp(db)
}

func seedChain(t *testing.T, app *App) (batchID, layerID, aID, bID int64) {
	t.Helper()
	ctx := context.Background()
	b, err := NewBatchService(app).Create("CASE-P", "probe", "")
	if err != nil {
		t.Fatal(err)
	}
	frag := NewFragmentService(app)
	l, err := frag.AddLayer(b.ID, "L1", "s.tif", 1000, 800, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := frag.AddLayer(b.ID, "L2", "t.tif", 1000, 800, false); err != nil {
		t.Fatal(err)
	}
	a, err := frag.AddFragment(b.ID, l.ID, "A", 100, 100, 300, 100, 0.3)
	if err != nil {
		t.Fatal(err)
	}
	bb, err := frag.AddFragment(b.ID, l.ID, "B", 200, 40, 200, 200, 0.62)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := frag.CalibrateBatch(b.ID); err != nil {
		t.Fatal(err)
	}
	order := NewOrderService(app)
	if _, err := order.AddCrossing(ctx, b.ID, l.ID, a.ID, bb.ID, 200, 100, 0.85, "cover"); err != nil {
		t.Fatal(err)
	}
	if _, err := NewBatchService(app).Rebuild(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	return b.ID, l.ID, a.ID, bb.ID
}

func TestConcurrentAdjudicationOnlyOneWins(t *testing.T) {
	app := openProbeApp(t)
	ctx := context.Background()
	bID, _, _, _ := seedChain(t, app)
	order := NewOrderService(app)
	cand, err := order.RebuildCandidate(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}
	const workers = 20
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			edges, _ := app.Candidates.ListEdges(cand.ID)
			for _, e := range edges {
				if e.CandidateID != cand.ID {
					t.Errorf("edge candidate %d leaked onto %d", e.CandidateID, cand.ID)
				}
			}
			extra := &model.OrderCandidate{BatchID: bID, Status: model.CandConsistent, Score: 0.1}
			dummy := []model.CandidateEdge{{CandidateID: 999, BeforeFragmentID: 1, AfterFragmentID: 2, Weight: 0.2}}
			_, _ = app.Candidates.Create(ctx, extra, dummy)
			if i%2 == 0 {
				_, _ = order.ConfirmCandidate(ctx, cand.ID)
			} else {
				_, _ = order.RejectCandidate(ctx, cand.ID)
			}
		}(i)
	}
	wg.Wait()
	edges, err := app.Candidates.ListEdges(cand.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(edges) == 0 {
		t.Fatal("candidate edges missing")
	}
	for _, e := range edges {
		if e.CandidateID != cand.ID {
			t.Fatalf("list edges returned candidate %d want %d", e.CandidateID, cand.ID)
		}
	}
	got, err := app.Candidates.Get(cand.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != model.CandConfirmed && got.Status != model.CandRejected {
		t.Fatalf("status=%s want confirmed or rejected", got.Status)
	}
}
