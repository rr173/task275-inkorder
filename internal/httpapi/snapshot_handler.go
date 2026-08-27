package httpapi

import (
	"net/http"

	"task275-inkorder/internal/service"
)

// SnapshotHandler 处理鉴定快照请求。
type SnapshotHandler struct {
	app *service.App
	svc *service.SnapshotService
}

func NewSnapshotHandler(app *service.App) *SnapshotHandler {
	return &SnapshotHandler{app: app, svc: service.NewSnapshotService(app)}
}

type createSnapshotReq struct {
	CandidateID int64  `json:"candidate_id"`
	Note        string `json:"note"`
}

// CreateDraft POST /api/batches/{id}/snapshots
func (h *SnapshotHandler) CreateDraft(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req createSnapshotReq
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	sn, err := h.svc.CreateDraft(r.Context(), id, req.CandidateID, req.Note)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, sn)
}

// Get GET /api/snapshots/{id}
func (h *SnapshotHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	sn, err := h.svc.Get(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sn)
}

// List GET /api/batches/{id}/snapshots
func (h *SnapshotHandler) List(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	sns, err := h.svc.List(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sns)
}

// Share POST /api/snapshots/{id}/share
func (h *SnapshotHandler) Share(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "share")
}

// Freeze POST /api/snapshots/{id}/freeze
func (h *SnapshotHandler) Freeze(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "freeze")
}

// Supersede POST /api/snapshots/{id}/supersede
func (h *SnapshotHandler) Supersede(w http.ResponseWriter, r *http.Request) {
	h.transition(w, r, "supersede")
}

func (h *SnapshotHandler) transition(w http.ResponseWriter, r *http.Request, action string) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var sn interface{}
	switch action {
	case "share":
		sn, err = h.svc.Share(id)
	case "freeze":
		sn, err = h.svc.Freeze(r.Context(), id)
	case "supersede":
		sn, err = h.svc.Supersede(id)
	}
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, sn)
}
