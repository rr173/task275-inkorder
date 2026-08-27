package service

import (
	"context"
	"fmt"

	"task275-inkorder/internal/geometry"
	"task275-inkorder/internal/model"
	"task275-inkorder/internal/order"
)

// OrderService 处理交叉登记、笔顺重建与候选裁决。
type OrderService struct {
	app *App
}

func NewOrderService(app *App) *OrderService { return &OrderService{app: app} }

// AddCrossing 登记一条交叉覆盖证据。
// firstFragmentID 先写，secondFragmentID 后写（覆盖）。
func (s *OrderService) AddCrossing(ctx context.Context, batchID, layerID, firstID, secondID int64, x, y float64, confidence float64, evidence string) (*model.Crossing, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f1, err := s.app.Fragments.Get(firstID)
	if err != nil {
		return nil, fmt.Errorf("load first fragment: %w", err)
	}
	f2, err := s.app.Fragments.Get(secondID)
	if err != nil {
		return nil, fmt.Errorf("load second fragment: %w", err)
	}
	if f1.BatchID != batchID || f2.BatchID != batchID {
		return nil, model.NewError(model.ErrCodeConflict, "片段不属于该批次")
	}
	if firstID == secondID {
		return nil, model.NewError(model.ErrCodeInvalid, "交叉证据的两端不能是同一片段")
	}
	if confidence < 0 || confidence > 1 {
		return nil, model.NewError(model.ErrCodeInvalid, "置信度必须在 0~1 之间")
	}
	c := &model.Crossing{
		BatchID: batchID, LayerID: layerID,
		FirstFragmentID: firstID, SecondFragmentID: secondID,
		X: x, Y: y, Confidence: confidence, Evidence: evidence,
	}
	id, err := s.app.Crossings.Create(c)
	if err != nil {
		return nil, fmt.Errorf("save crossing: %w", err)
	}
	got, err := s.app.Crossings.Get(id)
	if err != nil {
		return nil, fmt.Errorf("reload crossing: %w", err)
	}
	return got, nil
}

// AutoCross 用几何算法自动判定一对片段的交叉覆盖关系并登记。
func (s *OrderService) AutoCross(ctx context.Context, batchID, layerID, firstID, secondID int64) (*model.Crossing, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f1, err := s.app.Fragments.Get(firstID)
	if err != nil {
		return nil, fmt.Errorf("load first fragment: %w", err)
	}
	f2, err := s.app.Fragments.Get(secondID)
	if err != nil {
		return nil, fmt.Errorf("load second fragment: %w", err)
	}
	if f1.Status == model.FragmentRaw || f2.Status == model.FragmentRaw {
		return nil, model.NewError(model.ErrCodeBadState, "片段尚未校正，无法做交叉判定")
	}
	judge := geometry.NewCrossJudge()
	ev := judge.Judge(f1, f2)
	if !ev.Intersect {
		return nil, model.NewError(model.ErrCodeConflict, "两笔画不相交")
	}
	evidence := fmt.Sprintf("自动判定：交叉点(%.1f,%.1f)，%s", ev.X, ev.Y, ev.ArtifactWhy)
	c := &model.Crossing{
		BatchID: batchID, LayerID: layerID,
		FirstFragmentID: firstID, SecondFragmentID: secondID,
		X: ev.X, Y: ev.Y, Confidence: ev.Confidence,
		Evidence: evidence, IsArtifact: ev.IsArtifact,
	}
	id, err := s.app.Crossings.Create(c)
	if err != nil {
		return nil, fmt.Errorf("save auto crossing: %w", err)
	}
	got, err := s.app.Crossings.Get(id)
	if err != nil {
		return nil, fmt.Errorf("reload auto crossing: %w", err)
	}
	return got, nil
}

// RebuildCandidate 重建批次笔顺候选。
// 从全部活跃片段 + 非伪影交叉证据 + 观测点构建偏序图，输出候选。
// ctx 取消时立即终止重建，不再生成或保存候选（避免研究者已取消重建却仍落库候选）。
func (s *OrderService) RebuildCandidate(ctx context.Context, batchID int64) (*model.OrderCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := s.app.Batches.Get(batchID)
	if err != nil {
		return nil, fmt.Errorf("load batch for rebuild: %w", err)
	}
	if b.Status != model.BatchImporting && b.Status != model.BatchPendingRebuild {
		return nil, model.NewError(model.ErrCodeBadState, "批次状态 %s 不允许重建候选", b.Status)
	}
	fs, err := s.app.Fragments.ListByBatch(batchID)
	if err != nil {
		return nil, fmt.Errorf("list fragments for rebuild: %w", err)
	}
	cs, err := s.app.Crossings.ListByBatch(batchID)
	if err != nil {
		return nil, fmt.Errorf("list crossings for rebuild: %w", err)
	}
	obs, err := s.app.Observations.ListByBatch(batchID)
	if err != nil {
		return nil, fmt.Errorf("list observations for rebuild: %w", err)
	}
	// 加载完证据后再次确认未被取消：重建本身是 CPU/IO 重活，
	// 研究者可能在数据加载阶段就取消了重建。
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	obsEdges := pauseToEdges(obs, fs)
	res := (order.Builder{}).Build(fs, cs, obs)
	allEdges := append([]model.CandidateEdge{}, res.Edges...)
	allEdges = append(allEdges, obsEdges...)

	// 落库前最后确认未被取消：一旦取消就绝不再写入候选，
	// 否则会出现“研究者已取消重建，系统却仍生成并保存候选”的问题。
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	status := model.CandConsistent
	conflictReason := ""
	if res.HasCycle {
		status = model.CandConflict
		conflictReason = res.ConflictReason
	}
	cand := &model.OrderCandidate{
		BatchID: batchID, Status: status,
		Score: res.Score, ConflictReason: conflictReason,
	}
	id, err := s.app.Candidates.Create(ctx, cand, allEdges)
	if err != nil {
		return nil, fmt.Errorf("persist candidate: %w", err)
	}
	got, err := s.app.Candidates.Get(id)
	if err != nil {
		return nil, fmt.Errorf("reload candidate: %w", err)
	}
	return got, nil
}

// pauseToEdges 把停顿观测映射为偏序边：停顿点在片段 B 终点附近且属于片段 A 时，
// 暗示 A 停顿后接写 B（A 先于 B）。
func pauseToEdges(obs []model.Observation, fs []model.Fragment) []model.CandidateEdge {
	var out []model.CandidateEdge
	for _, o := range obs {
		if o.Kind != model.ObsPause {
			continue
		}
		for _, f := range fs {
			if f.ID == o.FragmentID || !f.Active() {
				continue
			}
			dist := geometry.Distance(geometry.Point{X: o.X, Y: o.Y}, geometry.Point{X: f.CalibEndX, Y: f.CalibEndY})
			if dist < 12 {
				out = append(out, model.CandidateEdge{
					BeforeFragmentID: f.ID, AfterFragmentID: o.FragmentID,
					Source: model.EdgeFromPause, Weight: 0.6,
				})
			}
		}
	}
	return out
}

// ConfirmCandidate 确认候选（版本号乐观锁）。
func (s *OrderService) ConfirmCandidate(ctx context.Context, candidateID int64) (*model.OrderCandidate, error) {
	return s.adjudicate(ctx, candidateID, model.CandConfirmed)
}

// RejectCandidate 否决候选。
func (s *OrderService) RejectCandidate(ctx context.Context, candidateID int64) (*model.OrderCandidate, error) {
	return s.adjudicate(ctx, candidateID, model.CandRejected)
}

func (s *OrderService) adjudicate(ctx context.Context, candidateID int64, to model.CandidateStatus) (*model.OrderCandidate, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	c, err := s.app.Candidates.Get(candidateID)
	if err != nil {
		return nil, fmt.Errorf("load candidate for adjudicate: %w", err)
	}
	if c.Status == to {
		return c, nil
	}
	if !model.CanTransitionCandidate(c.Status, to) {
		return nil, model.NewError(model.ErrCodeBadState, "候选状态 %s 不能流转到 %s", c.Status, to)
	}
	ok, err := s.app.Candidates.UpdateStatusCAS(ctx, candidateID, c.Version, to)
	if err != nil {
		return nil, fmt.Errorf("cas adjudicate: %w", err)
	}
	if !ok {
		return nil, model.NewError(model.ErrCodeConflict, "候选已被其他裁决者修改，请刷新后重试")
	}
	got, err := s.app.Candidates.Get(candidateID)
	if err != nil {
		return nil, fmt.Errorf("reload adjudicated candidate: %w", err)
	}
	return got, nil
}

// AddObjection 登记异议。
func (s *OrderService) AddObjection(candidateID, fragmentID int64, kind model.ObjectionKind, reason string) (*model.Objection, error) {
	c, err := s.app.Candidates.Get(candidateID)
	if err != nil {
		return nil, fmt.Errorf("load candidate for objection: %w", err)
	}
	if c.Status == model.CandConfirmed || c.Status == model.CandRejected {
		return nil, model.NewError(model.ErrCodeBadState, "候选已裁决，不可再登记异议")
	}
	if reason == "" {
		return nil, model.NewError(model.ErrCodeInvalid, "异议原因不能为空")
	}
	o := &model.Objection{CandidateID: candidateID, FragmentID: fragmentID, Kind: kind, Reason: reason}
	id, err := s.app.Objections.Create(o)
	if err != nil {
		return nil, fmt.Errorf("save objection: %w", err)
	}
	obs, err := s.app.Objections.ListByCandidate(candidateID)
	if err != nil {
		return nil, fmt.Errorf("list objections: %w", err)
	}
	for i := range obs {
		if obs[i].ID == id {
			return &obs[i], nil
		}
	}
	return nil, model.NewError(model.ErrCodeNotFound, "objection %d not found", id)
}
