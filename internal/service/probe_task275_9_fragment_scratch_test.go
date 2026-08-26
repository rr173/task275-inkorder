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

func TestRebuildInPlaceFilterDoesNotCorruptOtherBatchList(t *testing.T) {
	app := openProbeApp(t)
	ctx := context.Background()
	bID, _, aID, _ := seedChain(t, app)
	held, err := app.Fragments.ListByBatch(bID)
	if err != nil {
		t.Fatal(err)
	}
	if len(held) < 2 {
		t.Fatal("expected fragments")
	}
	origBatch := held[0].BatchID
	origLen := len(held)
	if _, err := NewFragmentService(app).MarkFragmentArtifact(aID); err != nil {
		t.Fatal(err)
	}
	order := NewOrderService(app)
	if _, err := order.RebuildCandidate(ctx, bID); err != nil {
		t.Fatal(err)
	}
	b2, err := NewBatchService(app).Create("C2", "other", "")
	if err != nil {
		t.Fatal(err)
	}
	frag := NewFragmentService(app)
	l2, err := frag.AddLayer(b2.ID, "L9", "z.tif", 1000, 800, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := frag.AddFragment(b2.ID, l2.ID, "Z1", 1, 1, 2, 2, 0.5); err != nil {
		t.Fatal(err)
	}
	if _, err := frag.AddFragment(b2.ID, l2.ID, "Z2", 3, 3, 4, 4, 0.6); err != nil {
		t.Fatal(err)
	}
	fs, err := app.Fragments.ListByBatch(b2.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 2 {
		t.Fatalf("other batch list truncated or aliased: len=%d", len(fs))
	}
	for _, f := range fs {
		if f.BatchID != b2.ID {
			t.Fatalf("fragment batch %d leaked into batch %d", f.BatchID, b2.ID)
		}
	}
	if len(held) != origLen {
		t.Fatalf("held list mutated in place: len=%d want %d", len(held), origLen)
	}
	if held[0].BatchID != origBatch {
		t.Fatalf("held list aliased to another batch: got %d want %d", held[0].BatchID, origBatch)
	}
}
