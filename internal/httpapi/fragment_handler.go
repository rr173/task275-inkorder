package httpapi

import (
	"encoding/json"
	"io"
	"net/http"

	"task275-inkorder/internal/geometry"
	"task275-inkorder/internal/model"
	"task275-inkorder/internal/service"
)

// FragmentHandler 处理扫描层与片段相关请求。
type FragmentHandler struct {
	app *service.App
	svc *service.FragmentService
}

func NewFragmentHandler(app *service.App) *FragmentHandler {
	return &FragmentHandler{app: app, svc: service.NewFragmentService(app)}
}

type addLayerReq struct {
	Name    string  `json:"name"`
	ScanRef string  `json:"scan_ref"`
	Width   float64 `json:"width"`
	Height  float64 `json:"height"`
	IsBase  bool    `json:"is_base"`
}

// AddLayer POST /api/batches/{id}/layers
func (h *FragmentHandler) AddLayer(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req addLayerReq
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	l, err := h.svc.AddLayer(id, req.Name, req.ScanRef, req.Width, req.Height, req.IsBase)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, l)
}

// ListLayers GET /api/batches/{id}/layers
func (h *FragmentHandler) ListLayers(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	ls, err := h.app.Layers.ListByBatch(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, ls)
}

type setRulerReq struct {
	BasePoints  []pointReq `json:"base_points"`
	LayerPoints []pointReq `json:"layer_points"`
}

type pointReq struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

// SetRuler POST /api/layers/{id}/ruler
func (h *FragmentHandler) SetRuler(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req setRulerReq
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	var basePts, layerPts []geometry.Point
	for _, p := range req.BasePoints {
		basePts = append(basePts, geometry.Point{X: p.X, Y: p.Y})
	}
	for _, p := range req.LayerPoints {
		layerPts = append(layerPts, geometry.Point{X: p.X, Y: p.Y})
	}
	l, err := h.svc.SetRuler(id, basePts, layerPts)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, l)
}

type addFragmentReq struct {
	LayerID  int64   `json:"layer_id"`
	Label    string  `json:"label"`
	StartX   float64 `json:"start_x"`
	StartY   float64 `json:"start_y"`
	EndX     float64 `json:"end_x"`
	EndY     float64 `json:"end_y"`
	Pressure float64 `json:"pressure"`
}

// AddFragment POST /api/batches/{id}/fragments
func (h *FragmentHandler) AddFragment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req addFragmentReq
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	f, err := h.svc.AddFragment(id, req.LayerID, req.Label, req.StartX, req.StartY, req.EndX, req.EndY, req.Pressure)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, f)
}

// ListFragments GET /api/batches/{id}/fragments
func (h *FragmentHandler) ListFragments(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	fs, err := h.app.Fragments.ListByBatch(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, fs)
}

// GetFragment GET /api/fragments/{id}
func (h *FragmentHandler) GetFragment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	f, err := h.app.Fragments.Get(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// CalibrateBatch POST /api/batches/{id}/calibrate
func (h *FragmentHandler) CalibrateBatch(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	n, err := h.svc.CalibrateBatch(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"calibrated": n})
}

// CalibrateFragment POST /api/fragments/{id}/calibrate
func (h *FragmentHandler) CalibrateFragment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	f, err := h.svc.Calibrate(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// MarkArtifact POST /api/fragments/{id}/artifact
func (h *FragmentHandler) MarkArtifact(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	f, err := h.svc.MarkFragmentArtifact(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

// Exclude POST /api/fragments/{id}/exclude
func (h *FragmentHandler) Exclude(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	f, err := h.svc.ExcludeFragment(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, f)
}

type addObservationReq struct {
	Kind   string  `json:"kind"`
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Note   string  `json:"note"`
}

// AddObservation POST /api/fragments/{id}/observations
func (h *FragmentHandler) AddObservation(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	var req addObservationReq
	if err := decodeBody(r, &req); err != nil {
		writeError(w, err)
		return
	}
	kind, ok := model.ValidObsKind(req.Kind)
	if !ok {
		writeError(w, model.NewError(model.ErrCodeInvalid, "无效观测类型 %q", req.Kind))
		return
	}
	f, err := h.app.Fragments.Get(id)
	if err != nil {
		writeError(w, err)
		return
	}
	o, err := h.svc.AddObservation(f.BatchID, id, kind, req.X, req.Y, req.Note)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, o)
}

// ListObservations GET /api/fragments/{id}/observations
func (h *FragmentHandler) ListObservations(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r, "id")
	if err != nil {
		writeError(w, err)
		return
	}
	obs, err := h.app.Observations.ListByFragment(id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, obs)
}

func decodeBody(r *http.Request, v interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	if err := dec.Decode(v); err != nil {
		return model.NewError(model.ErrCodeInvalid, "请求体解析失败: %v", err)
	}
	return nil
}
