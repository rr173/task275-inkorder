package service

import (
	"context"
	"encoding/json"
	"fmt"

	"task275-inkorder/internal/model"
)

// SnapshotService 处理鉴定快照的生命周期。
type SnapshotService struct {
	app *App
}

func NewSnapshotService(app *App) *SnapshotService { return &SnapshotService{app: app} }

// CreateDraft 为已确认候选创建快照草稿，并保留扫描基准（标尺摘要）。
func (s *SnapshotService) CreateDraft(ctx context.Context, batchID, candidateID int64, note string) (*model.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := s.app.Batches.Get(batchID)
	if err != nil {
		return nil, fmt.Errorf("load batch for snapshot: %w", err)
	}
	if b.Status != model.BatchPendingReview && b.Status != model.BatchPublished {
		return nil, model.NewError(model.ErrCodeBadState, "批次状态 %s 不允许创建快照", b.Status)
	}
	c, err := s.app.Candidates.Get(candidateID)
	if err != nil {
		return nil, fmt.Errorf("load candidate for snapshot: %w", err)
	}
	if c.BatchID != batchID {
		return nil, model.NewError(model.ErrCodeConflict, "候选 %d 不属于批次 %d", candidateID, batchID)
	}
	if c.Status != model.CandConfirmed {
		return nil, model.NewError(model.ErrCodeBadState, "候选状态 %s 未确认，不能创建快照", c.Status)
	}
	snaps, err := s.app.Snapshots.ListByBatch(batchID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	for _, sn := range snaps {
		if sn.Status == model.SnapFrozen {
			return nil, model.NewError(model.ErrCodeConflict, "批次 %d 已有冻结快照 %d", batchID, sn.ID)
		}
	}

	rulerRef, err := s.rulerRef(batchID)
	if err != nil {
		return nil, err
	}
	sn := &model.Snapshot{BatchID: batchID, CandidateID: candidateID, RulerRef: rulerRef, Note: note}
	id, err := s.app.Snapshots.Create(sn)
	if err != nil {
		return nil, fmt.Errorf("save snapshot draft: %w", err)
	}
	got, err := s.app.Snapshots.Get(id)
	if err != nil {
		return nil, fmt.Errorf("reload snapshot draft: %w", err)
	}
	return got, nil
}

// rulerRef 汇总批次扫描层与标尺，形成不可变基准摘要。
func (s *SnapshotService) rulerRef(batchID int64) (string, error) {
	layers, err := s.app.Layers.ListByBatch(batchID)
	if err != nil {
		return "", fmt.Errorf("list layers for ruler ref: %w", err)
	}
	ref := ""
	for i, l := range layers {
		if i > 0 {
			ref += "; "
		}
		ref += fmt.Sprintf("%s(scale=%.3f,offset=(%.1f,%.1f))", l.Name, l.Scale, l.OffsetX, l.OffsetY)
	}
	return ref, nil
}

// captureEvidence 把冻结瞬间的候选边与标尺打成 JSON，之后 live 数据变更不得回写。
func (s *SnapshotService) captureEvidence(sn *model.Snapshot) (string, string, error) {
	rulerRef, err := s.rulerRef(sn.BatchID)
	if err != nil {
		return "", "", err
	}
	edges, err := s.app.Candidates.ListEdges(sn.CandidateID)
	if err != nil {
		return "", "", fmt.Errorf("list edges for freeze: %w", err)
	}
	cand, err := s.app.Candidates.Get(sn.CandidateID)
	if err != nil {
		return "", "", fmt.Errorf("load candidate for freeze: %w", err)
	}
	payload := map[string]interface{}{
		"candidate_id":     cand.ID,
		"candidate_status": cand.Status,
		"ruler_ref":        rulerRef,
		"edges":            edges,
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", "", fmt.Errorf("marshal freeze evidence: %w", err)
	}
	return string(raw), rulerRef, nil
}

// transition 通用快照状态流转（非冻结路径）。
func (s *SnapshotService) transition(snapshotID int64, to model.SnapshotStatus) (*model.Snapshot, error) {
	sn, err := s.app.Snapshots.Get(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("load snapshot: %w", err)
	}
	if sn.Status == to {
		return sn, nil
	}
	if !model.CanTransitionSnapshot(sn.Status, to) {
		return nil, model.NewError(model.ErrCodeBadState, "快照状态 %s 不能流转到 %s", sn.Status, to)
	}
	if err := s.app.Snapshots.UpdateStatus(snapshotID, to); err != nil {
		return nil, fmt.Errorf("update snapshot status: %w", err)
	}
	got, err := s.app.Snapshots.Get(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("reload snapshot: %w", err)
	}
	return got, nil
}

// Share 共享快照（draft → shared）。
func (s *SnapshotService) Share(snapshotID int64) (*model.Snapshot, error) {
	return s.transition(snapshotID, model.SnapShared)
}

// Freeze 冻结快照（shared → frozen），并把当时的标尺与偏序边写入 evidence_json。
// ctx 关联研究者请求：若请求在冻结前被取消，事务不会提交，快照保持未冻结。
func (s *SnapshotService) Freeze(ctx context.Context, snapshotID int64) (*model.Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	sn, err := s.app.Snapshots.Get(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("load snapshot for freeze: %w", err)
	}
	if sn.Status == model.SnapFrozen {
		return sn, nil
	}
	evidence, rulerRef, err := s.captureEvidence(sn)
	if err != nil {
		return nil, err
	}
	if err := s.app.Snapshots.FreezeWithEvidence(ctx, snapshotID, evidence, rulerRef); err != nil {
		return nil, fmt.Errorf("persist frozen snapshot: %w", err)
	}
	got, err := s.app.Snapshots.Get(snapshotID)
	if err != nil {
		return nil, fmt.Errorf("reload frozen snapshot: %w", err)
	}
	return got, nil
}

// Get 读取快照。冻结后返回库内 evidence_json / ruler_ref，不回读 live 扫描层。
func (s *SnapshotService) Get(id int64) (*model.Snapshot, error) {
	sn, err := s.app.Snapshots.Get(id)
	if err != nil {
		return nil, fmt.Errorf("load snapshot: %w", err)
	}
	return sn, nil
}

// Supersede 替代快照（frozen → superseded）。
func (s *SnapshotService) Supersede(snapshotID int64) (*model.Snapshot, error) {
	return s.transition(snapshotID, model.SnapSuperseded)
}

// List 列批次快照。
func (s *SnapshotService) List(batchID int64) ([]model.Snapshot, error) {
	return s.app.Snapshots.ListByBatch(batchID)
}
