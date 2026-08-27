package service

import (
	"context"
	"testing"

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

func TestListEdgesDoesNotAliasAcrossCandidates(t *testing.T) {
	app := openProbeApp(t)
	ctx := context.Background()
	bID, lID, aID, bFrag := seedChain(t, app)
	order := NewOrderService(app)
	c1, err := order.RebuildCandidate(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := order.AddCrossing(ctx, bID, lID, bFrag, aID, 200, 100, 0.4, "rev"); err != nil {
		t.Fatal(err)
	}
	c2, err := order.RebuildCandidate(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}
	e1, err := app.Candidates.ListEdges(c1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(e1) == 0 {
		t.Fatal("first candidate has no edges")
	}
	firstID := e1[0].CandidateID
	e2, err := app.Candidates.ListEdges(c2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(e2) == 0 {
		t.Fatal("second candidate has no edges")
	}
	if e1[0].CandidateID != firstID || e1[0].CandidateID != c1.ID {
		t.Fatalf("holding first edge list after second list: got candidate %d want %d", e1[0].CandidateID, c1.ID)
	}
}
