package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task275-inkorder/internal/model"
	"task275-inkorder/internal/service"
	"task275-inkorder/internal/store"
)

func TestCancelledFreezeDoesNotPinSnapshot(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := service.NewApp(db)
	ctx := context.Background()
	b, err := service.NewBatchService(app).Create("C", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	frag := service.NewFragmentService(app)
	l, err := frag.AddLayer(b.ID, "L1", "s.tif", 1000, 800, true)
	if err != nil {
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
	order := service.NewOrderService(app)
	if _, err := order.AddCrossing(ctx, b.ID, l.ID, a.ID, bb.ID, 200, 100, 0.85, "c"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.NewBatchService(app).Rebuild(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	cand, err := order.RebuildCandidate(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := order.ConfirmCandidate(ctx, cand.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.NewBatchService(app).ToReview(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	snapSvc := service.NewSnapshotService(app)
	sn, err := snapSvc.CreateDraft(ctx, b.ID, cand.ID, "n")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := snapSvc.Share(sn.ID); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(app).Router()
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	req := httptest.NewRequest(http.MethodPost, "/api/snapshots/1/freeze", bytes.NewReader(nil)).WithContext(cancelCtx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	got, err := snapSvc.Get(sn.ID)
	if err != nil {
		t.Fatal(err)
	}
	if rec.Code == http.StatusOK && got.Status == model.SnapFrozen {
		t.Fatalf("cancelled freeze still frozen: status=%s body=%s", got.Status, rec.Body.String())
	}
	if got.Status == model.SnapFrozen {
		t.Fatalf("snapshot status became frozen after cancelled request")
	}
	_ = json.Marshal
}
