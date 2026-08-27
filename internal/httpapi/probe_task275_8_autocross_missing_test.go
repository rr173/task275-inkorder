package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task275-inkorder/internal/service"
	"task275-inkorder/internal/store"
)

func TestAutoCrossMissingFragmentDoesNotPanic(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := service.NewApp(db)
	b, err := service.NewBatchService(app).Create("C", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	frag := service.NewFragmentService(app)
	if _, err := frag.AddLayer(b.ID, "L1", "s.tif", 1000, 800, true); err != nil {
		t.Fatal(err)
	}
	h := NewHandler(app).Router()
	body, _ := json.Marshal(map[string]int64{"layer_id": 1, "first_fragment_id": 999, "second_fragment_id": 1000})
	req := httptest.NewRequest(http.MethodPost, "/api/batches/1/crossings/auto", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code == http.StatusCreated {
		t.Fatalf("missing fragment returned created: %s", rec.Body.String())
	}
	if rec.Code == http.StatusInternalServerError {
		t.Fatalf("missing fragment became internal error: %s", rec.Body.String())
	}
	if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest && rec.Code != http.StatusConflict {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}
