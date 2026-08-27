package httpapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"task275-inkorder/internal/service"
	"task275-inkorder/internal/store"
)

// TestGetBatchNotFound 当研究者请求不存在的批次时，应返回 404 而非 500。
// 复现：修复前 service 层用 fmt.Errorf("...%v") 丢弃了 store 返回的
// NOT_FOUND 领域错误，writeError 的直接类型断言无法穿透，回退为 500。
func TestGetBatchNotFound(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	app := service.NewApp(db)
	h := NewHandler(app)

	req := httptest.NewRequest(http.MethodGet, "/api/batches/9999", nil)
	req.SetPathValue("id", "9999")
	rec := httptest.NewRecorder()
	h.batches.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "NOT_FOUND") {
		t.Fatalf("body = %q, want code NOT_FOUND", rec.Body.String())
	}
}

// TestGetSnapshotNotFound 同样的 %w 包装链对快照也成立。
func TestGetSnapshotNotFound(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	app := service.NewApp(db)
	h := NewHandler(app)

	req := httptest.NewRequest(http.MethodGet, "/api/snapshots/9999", nil)
	req.SetPathValue("id", "9999")
	rec := httptest.NewRecorder()
	h.snapshots.Get(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d (body=%s)", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
