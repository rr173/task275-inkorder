package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"task275-inkorder/internal/model"
	"task275-inkorder/internal/service"
	"task275-inkorder/internal/store"
)

func TestCancelledRebuildDoesNotPersistCandidate(t *testing.T) {
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
	if _, err := service.NewOrderService(app).AddCrossing(ctx, b.ID, l.ID, a.ID, bb.ID, 200, 100, 0.85, "c"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.NewBatchService(app).Rebuild(ctx, b.ID); err != nil {
		t.Fatal(err)
	}
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	h := NewHandler(app).Router()
	req := httptest.NewRequest(http.MethodPost, "/api/batches/1/candidates", nil).WithContext(cancelCtx)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusCreated {
		t.Fatalf("cancelled rebuild persisted candidate: %s", rec.Body.String())
	}
	list, err := app.Candidates.ListByBatch(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("cancelled rebuild still saved %d candidates", len(list))
	}
	_ = model.CandConsistent
}
