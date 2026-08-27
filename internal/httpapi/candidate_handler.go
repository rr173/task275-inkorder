package httpapi

import (
	"context"
	"net/http"

	"task275-inkorder/internal/model"
	"task275-inkorder/internal/service"
)

// CandidateHandler 处理交叉证据与笔顺候选请求。
type CandidateHandler struct {
	app *service.App
	svc *service.OrderService
}

func NewCandidateHandler(app *service.App) *CandidateHandler {
	return &CandidateHandler{app: app, svc: service.NewOrderService(app)}
}

type addCrossingReq struct {
	LayerID    int64   `json:"layer_id"`
	FirstID    int64   `json:"first_fragment_id"`
	SecondID   int64   `json:"second_fragment_id"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	Confidence float64 `json:"confidence"`
	Evidence   string  `json:"evidence"`
}

// AddCrossing POST /api/batches/{id}/crossings
func (h *CandidateHandler) AddCrossing(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req addCrossingReq
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	c, err := h.svc.AddCrossing(r.Context(), id, req.LayerID, req.FirstID, req.SecondID,
		req.X, req.Y, req.Confidence, req.Evidence)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

type autoCrossReq struct {
	LayerID int64 `json:"layer_id"`
	FirstID int64 `json:"first_fragment_id"`
	SecondID int64 `json:"second_fragment_id"`
}

// AutoCross POST /api/batches/{id}/crossings/auto
func (h *CandidateHandler) AutoCross(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req autoCrossReq
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	c, err := h.svc.AutoCross(context.Background(), id, req.LayerID, req.FirstID, req.SecondID)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// ListCrossings GET /api/batches/{id}/crossings
func (h *CandidateHandler) ListCrossings(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	cs, err := h.app.Crossings.ListByBatch(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

// Rebuild POST /api/batches/{id}/candidates
func (h *CandidateHandler) Rebuild(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	c, err := h.svc.RebuildCandidate(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

// List GET /api/batches/{id}/candidates
func (h *CandidateHandler) List(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	cs, err := h.app.Candidates.ListByBatch(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cs)
}

// Get GET /api/candidates/{id}
func (h *CandidateHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	c, err := h.app.Candidates.Get(id)
	if err != nil {
		writeError(w, err)
		return
	}
	edges, err := h.app.Candidates.ListEdges(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"candidate": c,
		"edges":     edges,
	})
}

// Confirm POST /api/candidates/{id}/confirm
func (h *CandidateHandler) Confirm(w http.ResponseWriter, r *http.Request) {
	h.adjudicate(w, r, true)
}

// Reject POST /api/candidates/{id}/reject
func (h *CandidateHandler) Reject(w http.ResponseWriter, r *http.Request) {
	h.adjudicate(w, r, false)
}

func (h *CandidateHandler) adjudicate(w http.ResponseWriter, r *http.Request, confirm bool) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var c interface{}
	if confirm {
		c, err = h.svc.ConfirmCandidate(r.Context(), id)
	} else {
		c, err = h.svc.RejectCandidate(r.Context(), id)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

type addObjectionReq struct {
	FragmentID int64  `json:"fragment_id"`
	Kind       string `json:"kind"`
	Reason     string `json:"reason"`
}

// AddObjection POST /api/candidates/{id}/objections
func (h *CandidateHandler) AddObjection(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req addObjectionReq
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	kind, ok := model.ValidObjectionKind(req.Kind)
	if !ok {
		writeError(w, model.NewError(model.ErrCodeInvalid, "无效异议类型 %q", req.Kind))
		return
	}
	o, err := h.svc.AddObjection(id, req.FragmentID, kind, req.Reason)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, o)
}

// ListObjections GET /api/candidates/{id}/objections
func (h *CandidateHandler) ListObjections(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	obs, err := h.app.Objections.ListByCandidate(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, obs)
}
