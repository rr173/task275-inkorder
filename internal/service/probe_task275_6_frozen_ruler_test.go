package service

import (
	"context"
	"testing"

	"task275-inkorder/internal/geometry"
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

func TestFrozenSnapshotKeepsPinnedRulerAfterLiveChange(t *testing.T) {
	app := openProbeApp(t)
	ctx := context.Background()
	bID, _, _, _ := seedChain(t, app)
	order := NewOrderService(app)
	cand, err := order.RebuildCandidate(ctx, bID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := order.ConfirmCandidate(ctx, cand.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := NewBatchService(app).ToReview(ctx, bID); err != nil {
		t.Fatal(err)
	}
	frag := NewFragmentService(app)
	layers, err := app.Layers.ListByBatch(bID)
	if err != nil {
		t.Fatal(err)
	}
	var l2ID int64
	for _, ly := range layers {
		if !ly.IsBase {
			l2ID = ly.ID
			break
		}
	}
	if l2ID == 0 {
		t.Fatal("missing contrast layer")
	}
	if _, err := frag.SetRuler(l2ID, []geometry.Point{{X: 0, Y: 0}, {X: 100, Y: 0}}, []geometry.Point{{X: 10, Y: 2}, {X: 110, Y: 2}}); err != nil {
		t.Fatal(err)
	}
	snapSvc := NewSnapshotService(app)
	sn, err := snapSvc.CreateDraft(ctx, bID, cand.ID, "note")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snapSvc.Share(sn.ID); err != nil {
		t.Fatal(err)
	}
	frozen, err := snapSvc.Freeze(ctx, sn.ID)
	if err != nil {
		t.Fatal(err)
	}
	pinned := frozen.RulerRef
	if pinned == "" && frozen.EvidenceJSON == "" {
		t.Fatal("frozen snapshot has empty evidence")
	}
	if _, err := frag.SetRuler(l2ID, []geometry.Point{{X: 0, Y: 0}, {X: 100, Y: 0}}, []geometry.Point{{X: 40, Y: 8}, {X: 140, Y: 8}}); err != nil {
		t.Fatal(err)
	}
	got, err := snapSvc.Get(sn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.RulerRef != pinned {
		t.Fatalf("frozen ruler mutated: got %q want %q", got.RulerRef, pinned)
	}
}
