package httpapi

import (
	"net/http"

	"task275-inkorder/internal/service"
)

// BatchHandler 处理批次相关请求。
type BatchHandler struct {
	svc *service.BatchService
}

func NewBatchHandler(app *service.App) *BatchHandler {
	return &BatchHandler{svc: service.NewBatchService(app)}
}

type createBatchReq struct {
	CaseRef     string `json:"case_ref"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

// Create POST /api/batches
func (h *BatchHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createBatchReq
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	b, err := h.svc.Create(req.CaseRef, req.Title, req.Description)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, b)
}

// List GET /api/batches
func (h *BatchHandler) List(w http.ResponseWriter, r *http.Request) {
	bs, err := h.svc.List(100)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bs)
}

// Get GET /api/batches/{id}
func (h *BatchHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	b, err := h.svc.Get(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}

// Rebuild POST /api/batches/{id}/rebuild
func (h *BatchHandler) Rebuild(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "rebuild")
}

// ToReview POST /api/batches/{id}/review
func (h *BatchHandler) ToReview(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "review")
}

// Publish POST /api/batches/{id}/publish
func (h *BatchHandler) Publish(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "publish")
}

// Seal POST /api/batches/{id}/seal
func (h *BatchHandler) Seal(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "seal")
}

func (h *BatchHandler) transition(w http.ResponseWriter, r *http.Request, action string) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var b interface{}
	ctx := r.Context()
	switch action {
	case "rebuild":
		b, err = h.svc.Rebuild(ctx, id)
	case "review":
		b, err = h.svc.ToReview(ctx, id)
	case "publish":
		b, err = h.svc.Publish(ctx, id)
	case "seal":
		b, err = h.svc.Seal(ctx, id)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, b)
}
