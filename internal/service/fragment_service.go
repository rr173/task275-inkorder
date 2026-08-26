package service

import (
	"fmt"

	"task275-inkorder/internal/geometry"
	"task275-inkorder/internal/model"
)

// FragmentService 处理片段导入、标尺校正、伪影与交叉登记。
type FragmentService struct {
	app *App
}

func NewFragmentService(app *App) *FragmentService { return &FragmentService{app: app} }

// AddLayer 注册扫描层；首个层自动成为基准层。
func (s *FragmentService) AddLayer(batchID int64, name, scanRef string, width, height float64, isBase bool) (*model.Layer, error) {
	if name == "" {
		return nil, model.NewError(model.ErrCodeInvalid, "扫描层名称不能为空")
	}
	if width <= 0 || height <= 0 {
		return nil, model.NewError(model.ErrCodeInvalid, "扫描层宽高必须为正数")
	}
	b, err := s.app.Batches.Get(batchID)
	if err != nil {
		return nil, fmt.Errorf("load batch for layer: %w", err)
	}
	if b.Status != model.BatchImporting {
		return nil, model.NewError(model.ErrCodeBadState, "批次状态 %s 不允许注册扫描层", b.Status)
	}
	l := &model.Layer{BatchID: batchID, Name: name, ScanRef: scanRef, Width: width, Height: height, IsBase: isBase}
	id, err := s.app.Layers.Create(l)
	if err != nil {
		return nil, err
	}
	return s.app.Layers.Get(id)
}

// AddFragment 导入一条片段（原始坐标）。
func (s *FragmentService) AddFragment(batchID, layerID int64, label string, sx, sy, ex, ey, pressure float64) (*model.Fragment, error) {
	if label == "" {
		return nil, model.NewError(model.ErrCodeInvalid, "片段标签不能为空")
	}
	if pressure < 0 || pressure > 1 {
		return nil, model.NewError(model.ErrCodeInvalid, "笔压必须在 0~1 之间")
	}
	b, err := s.app.Batches.Get(batchID)
	if err != nil {
		return nil, fmt.Errorf("load batch for fragment: %v", err)
	}
	if b.Status != model.BatchImporting {
		return nil, model.NewError(model.ErrCodeBadState, "批次状态 %s 不允许导入片段", b.Status)
	}
	layer, err := s.app.Layers.Get(layerID)
	if err != nil {
		return nil, fmt.Errorf("load layer for fragment: %w", err)
	}
	if layer.BatchID != batchID {
		return nil, model.NewError(model.ErrCodeConflict, "扫描层 %d 不属于批次 %d", layerID, batchID)
	}
	// 片段标签在同一批次内唯一
	fs, err := s.app.Fragments.ListByBatch(batchID)
	if err != nil {
		return nil, err
	}
	for _, f := range fs {
		if f.Label == label {
			return nil, model.NewError(model.ErrCodeDuplicate, "片段标签 %q 已存在", label)
		}
	}
	f := &model.Fragment{
		BatchID: batchID, LayerID: layerID, Label: label,
		StartX: sx, StartY: sy, EndX: ex, EndY: ey, Pressure: pressure,
	}
	id, err := s.app.Fragments.Create(f)
	if err != nil {
		return nil, err
	}
	return s.app.Fragments.Get(id)
}

// Calibrate 用标尺把片段坐标变换到基准层并写入校正坐标。
// 若片段所在层即基准层，直接复制坐标。
func (s *FragmentService) Calibrate(fragmentID int64) (*model.Fragment, error) {
	f, err := s.app.Fragments.Get(fragmentID)
	if err != nil {
		return nil, fmt.Errorf("load fragment for calibrate: %w", err)
	}
	layer, err := s.app.Layers.Get(f.LayerID)
	if err != nil {
		return nil, err
	}
	if layer.IsBase {
		if err := s.app.Fragments.UpdateCalibration(fragmentID, f.StartX, f.StartY, f.EndX, f.EndY); err != nil {
			return nil, err
		}
	} else {
		ruler := model.FromLayer(layer)
		bsx, bsy, bex, bey := geometry.AlignFragment(ruler, f.StartX, f.StartY, f.EndX, f.EndY)
		if err := s.app.Fragments.UpdateCalibration(fragmentID, bsx, bsy, bex, bey); err != nil {
			return nil, err
		}
	}
	return s.app.Fragments.Get(fragmentID)
}

// CalibrateBatch 校正批次全部片段。
func (s *FragmentService) CalibrateBatch(batchID int64) (int, error) {
	fs, err := s.app.Fragments.ListByBatch(batchID)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, f := range fs {
		if f.Status == model.FragmentRaw {
			if _, err := s.Calibrate(f.ID); err != nil {
				return n, err
			}
			n++
		}
	}
	return n, nil
}

// SetRuler 为层设置标尺（非基准层）。基点对以基准层坐标为参考。
func (s *FragmentService) SetRuler(layerID int64, basePts, layerPts []geometry.Point) (*model.Layer, error) {
	layer, err := s.app.Layers.Get(layerID)
	if err != nil {
		return nil, err
	}
	if layer.IsBase {
		return nil, model.NewError(model.ErrCodeConflict, "基准层不可设置标尺")
	}
	if len(basePts) == 0 || len(basePts) != len(layerPts) {
		return nil, model.NewError(model.ErrCodeInvalid, "基点对数量不匹配或为空")
	}
	r := geometry.EstimateRuler(basePts, layerPts)
	resid := geometry.Residual(r, basePts, layerPts)
	if resid > 20 {
		return nil, model.NewError(model.ErrCodeInvalid, "标尺校正残差过大(%.2f)，请检查基点", resid)
	}
	if err := s.app.Layers.UpdateRuler(layerID, r.Scale, r.OffsetX, r.OffsetY); err != nil {
		return nil, err
	}
	return s.app.Layers.Get(layerID)
}

// MarkFragmentArtifact 标记片段为伪影（raw/calibrated → artifact）。
func (s *FragmentService) MarkFragmentArtifact(fragmentID int64) (*model.Fragment, error) {
	f, err := s.app.Fragments.Get(fragmentID)
	if err != nil {
		return nil, err
	}
	if !model.CanTransitionFragment(f.Status, model.FragmentArtifact) {
		return nil, model.NewError(model.ErrCodeBadState, "片段状态 %s 不能标记伪影", f.Status)
	}
	if err := s.app.Fragments.UpdateStatus(fragmentID, model.FragmentArtifact); err != nil {
		return nil, err
	}
	return s.app.Fragments.Get(fragmentID)
}

// ExcludeFragment 排除片段（任何非终态 → excluded）。
func (s *FragmentService) ExcludeFragment(fragmentID int64) (*model.Fragment, error) {
	f, err := s.app.Fragments.Get(fragmentID)
	if err != nil {
		return nil, err
	}
	if f.Status == model.FragmentExcluded {
		return f, nil
	}
	if !model.CanTransitionFragment(f.Status, model.FragmentExcluded) {
		return nil, model.NewError(model.ErrCodeBadState, "片段状态 %s 不能排除", f.Status)
	}
	if err := s.app.Fragments.UpdateStatus(fragmentID, model.FragmentExcluded); err != nil {
		return nil, err
	}
	return s.app.Fragments.Get(fragmentID)
}

// AddObservation 登记观测点。
func (s *FragmentService) AddObservation(batchID, fragmentID int64, kind model.ObsKind, x, y float64, note string) (*model.Observation, error) {
	f, err := s.app.Fragments.Get(fragmentID)
	if err != nil {
		return nil, err
	}
	if f.BatchID != batchID {
		return nil, model.NewError(model.ErrCodeConflict, "片段 %d 不属于批次 %d", fragmentID, batchID)
	}
	o := &model.Observation{BatchID: batchID, FragmentID: fragmentID, Kind: kind, X: x, Y: y, Note: note}
	id, err := s.app.Observations.Create(o)
	if err != nil {
		return nil, err
	}
	obs, err := s.app.Observations.ListByFragment(fragmentID)
	if err != nil {
		return nil, err
	}
	if got := listObservationsByID(obs, id); got != nil {
		return got, nil
	}
	return nil, model.NewError(model.ErrCodeNotFound, "observation %d not found", id)
}

// listObservationsByID 取单个观测点（helper）。
func listObservationsByID(obs []model.Observation, id int64) *model.Observation {
	for i := range obs {
		if obs[i].ID == id {
			return &obs[i]
		}
	}
	return nil
}
