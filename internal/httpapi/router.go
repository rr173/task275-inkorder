package httpapi

import (
	"net/http"

	"task275-inkorder/internal/service"
)

// Handler 聚合全部 HTTP 处理器。
type Handler struct {
	app       *service.App
	batches   *BatchHandler
	fragments *FragmentHandler
	candidates *CandidateHandler
	snapshots *SnapshotHandler
}

// NewHandler 构造处理器。
func NewHandler(app *service.App) *Handler {
	return &Handler{
		app:        app,
		batches:    NewBatchHandler(app),
		fragments:  NewFragmentHandler(app),
		candidates: NewCandidateHandler(app),
		snapshots:  NewSnapshotHandler(app),
	}
}

// Router 组装全部 /api 路由（Go 1.22+ PathValue 风格）。
func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()

	// 健康与自检
	mux.HandleFunc("GET /api/health", h.health)
	mux.HandleFunc("GET /api/stats", h.stats)

	// 批次
	mux.HandleFunc("POST /api/batches", h.batches.Create)
	mux.HandleFunc("GET /api/batches", h.batches.List)
	mux.HandleFunc("GET /api/batches/{id}", h.batches.Get)
	mux.HandleFunc("POST /api/batches/{id}/rebuild", h.batches.Rebuild)
	mux.HandleFunc("POST /api/batches/{id}/review", h.batches.ToReview)
	mux.HandleFunc("POST /api/batches/{id}/publish", h.batches.Publish)
	mux.HandleFunc("POST /api/batches/{id}/seal", h.batches.Seal)

	// 扫描层
	mux.HandleFunc("POST /api/batches/{id}/layers", h.fragments.AddLayer)
	mux.HandleFunc("GET /api/batches/{id}/layers", h.fragments.ListLayers)
	mux.HandleFunc("POST /api/layers/{id}/ruler", h.fragments.SetRuler)

	// 片段
	mux.HandleFunc("POST /api/batches/{id}/fragments", h.fragments.AddFragment)
	mux.HandleFunc("GET /api/batches/{id}/fragments", h.fragments.ListFragments)
	mux.HandleFunc("GET /api/fragments/{id}", h.fragments.GetFragment)
	mux.HandleFunc("POST /api/batches/{id}/calibrate", h.fragments.CalibrateBatch)
	mux.HandleFunc("POST /api/fragments/{id}/calibrate", h.fragments.CalibrateFragment)
	mux.HandleFunc("POST /api/fragments/{id}/artifact", h.fragments.MarkArtifact)
	mux.HandleFunc("POST /api/fragments/{id}/exclude", h.fragments.Exclude)
	mux.HandleFunc("POST /api/fragments/{id}/observations", h.fragments.AddObservation)
	mux.HandleFunc("GET /api/fragments/{id}/observations", h.fragments.ListObservations)

	// 交叉与笔顺
	mux.HandleFunc("POST /api/batches/{id}/crossings", h.candidates.AddCrossing)
	mux.HandleFunc("POST /api/batches/{id}/crossings/auto", h.candidates.AutoCross)
	mux.HandleFunc("GET /api/batches/{id}/crossings", h.candidates.ListCrossings)
	mux.HandleFunc("POST /api/batches/{id}/candidates", h.candidates.Rebuild)
	mux.HandleFunc("GET /api/batches/{id}/candidates", h.candidates.List)
	mux.HandleFunc("GET /api/candidates/{id}", h.candidates.Get)
	mux.HandleFunc("POST /api/candidates/{id}/confirm", h.candidates.Confirm)
	mux.HandleFunc("POST /api/candidates/{id}/reject", h.candidates.Reject)
	mux.HandleFunc("POST /api/candidates/{id}/objections", h.candidates.AddObjection)
	mux.HandleFunc("GET /api/candidates/{id}/objections", h.candidates.ListObjections)

	// 快照
	mux.HandleFunc("POST /api/batches/{id}/snapshots", h.snapshots.CreateDraft)
	mux.HandleFunc("GET /api/batches/{id}/snapshots", h.snapshots.List)
	mux.HandleFunc("GET /api/snapshots/{id}", h.snapshots.Get)
	mux.HandleFunc("POST /api/snapshots/{id}/share", h.snapshots.Share)
	mux.HandleFunc("POST /api/snapshots/{id}/freeze", h.snapshots.Freeze)
	mux.HandleFunc("POST /api/snapshots/{id}/supersede", h.snapshots.Supersede)

	return withMethodCheck(mux)
}

// withMethodCheck 包装 405 处理（ServeMux 已支持方法路由，此包装仅为统一 JSON 错误）。
func withMethodCheck(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) stats(w http.ResponseWriter, r *http.Request) {
	s := struct {
		Batches int `json:"batches"`
	}{
		Batches: countRows(h.app),
	}
	writeJSON(w, http.StatusOK, s)
}

// countRows 统计批次总数（统计入口）。
func countRows(app *service.App) int {
	bs, err := app.Batches.List(500)
	if err != nil {
		return -1
	}
	return len(bs)
}
