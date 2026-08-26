package httpapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"task275-inkorder/internal/service"
	"task275-inkorder/internal/store"
)

func TestMissingBatchReturnsNotFoundNotInternal(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	h := NewHandler(service.NewApp(db)).Router()
	req := httptest.NewRequest(http.MethodGet, "/api/batches/99999", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status=%d body=%s want 404", rec.Code, rec.Body.String())
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["code"] != "NOT_FOUND" {
		t.Fatalf("code=%q want NOT_FOUND", body["code"])
	}
}
