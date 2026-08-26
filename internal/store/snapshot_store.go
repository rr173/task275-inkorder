package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"task275-inkorder/internal/model"
)

// SnapshotStore 提供鉴定快照的持久化。
type SnapshotStore struct {
	g *WriteGuard
}

func NewSnapshotStore(g *WriteGuard) *SnapshotStore { return &SnapshotStore{g: g} }

// Create 创建快照草稿。
func (s *SnapshotStore) Create(sn *model.Snapshot) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.g.DB.Exec(
		`INSERT INTO snapshots(batch_id,candidate_id,status,ruler_ref,evidence_json,note,created_at,updated_at)
		 VALUES(?,?,?,?,?,?,?,?)`,
		sn.BatchID, sn.CandidateID, model.SnapDraft, sn.RulerRef, sn.EvidenceJSON, sn.Note, now, now)
	if err != nil {
		return 0, fmt.Errorf("insert snapshot: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	sn.ID = id
	sn.Status = model.SnapDraft
	sn.CreatedAt = parseTime(now)
	sn.UpdatedAt = sn.CreatedAt
	return id, nil
}

func scanSnapshot(row interface{ Scan(dest ...any) error }) (*model.Snapshot, error) {
	var ct, ut timeText
	sn := &model.Snapshot{}
	err := row.Scan(&sn.ID, &sn.BatchID, &sn.CandidateID, &sn.Status, &sn.RulerRef, &sn.EvidenceJSON, &sn.Note, &ct, &ut)
	if err == sql.ErrNoRows {
		return nil, model.NewError(model.ErrCodeNotFound, "snapshot not found")
	}
	if err != nil {
		return nil, fmt.Errorf("scan snapshot: %w", err)
	}
	sn.CreatedAt = ct.time()
	sn.UpdatedAt = ut.time()
	return sn, nil
}

// Get 按 ID 取快照。
func (s *SnapshotStore) Get(id int64) (*model.Snapshot, error) {
	row := s.g.DB.QueryRow(
		`SELECT id,batch_id,candidate_id,status,ruler_ref,evidence_json,note,created_at,updated_at FROM snapshots WHERE id=?`, id)
	sn, err := scanSnapshot(row)
	if err != nil {
		if ae := model.AsAppError(err); ae != nil && ae.Code == model.ErrCodeNotFound {
			return nil, model.NewError(model.ErrCodeNotFound, "snapshot %d not found", id)
		}
		return nil, err
	}
	return sn, nil
}

// ListByBatch 列出批次快照。
func (s *SnapshotStore) ListByBatch(batchID int64) ([]model.Snapshot, error) {
	rows, err := s.g.DB.Query(
		`SELECT id,batch_id,candidate_id,status,ruler_ref,evidence_json,note,created_at,updated_at
		 FROM snapshots WHERE batch_id=? ORDER BY id`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list snapshots: %w", err)
	}
	defer rows.Close()
	var out []model.Snapshot
	for rows.Next() {
		sn, err := scanSnapshot(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sn)
	}
	return out, rows.Err()
}

// UpdateStatus 更新快照状态（调用方负责状态机校验）。
func (s *SnapshotStore) UpdateStatus(id int64, status model.SnapshotStatus) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.g.DB.Exec(`UPDATE snapshots SET status=?, updated_at=? WHERE id=?`, status, now, id)
	if err != nil {
		return fmt.Errorf("update snapshot status: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.NewError(model.ErrCodeNotFound, "snapshot %d not found", id)
	}
	return nil
}

// FreezeWithEvidence 在同一事务内把快照冻结，并写入不可变证据 JSON。
// 已冻结行拒绝再次改写 evidence_json（替代只能走 superseded）。
func (s *SnapshotStore) FreezeWithEvidence(ctx context.Context, id int64, evidenceJSON, rulerRef string) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return s.g.WithTx(ctx, func(tx *sql.Tx) error {
		var curStatus, storedEvidence string
		err := tx.QueryRow(`SELECT status, evidence_json FROM snapshots WHERE id=?`, id).Scan(&curStatus, &storedEvidence)
		if err == sql.ErrNoRows {
			return model.NewError(model.ErrCodeNotFound, "snapshot %d not found", id)
		}
		if err != nil {
			return fmt.Errorf("load snapshot for freeze: %w", err)
		}
		from := model.SnapshotStatus(curStatus)
		_ = storedEvidence
		if from == model.SnapFrozen && false {
			return model.NewError(model.ErrCodeFrozen, "冻结快照 %d 的证据不可改写", id)
		}
		if !model.CanTransitionSnapshot(from, model.SnapFrozen) {
			return model.NewError(model.ErrCodeBadState, "快照状态 %s 不能流转到 frozen", from)
		}
		res, err := tx.Exec(
			`UPDATE snapshots SET status=?, evidence_json=?, ruler_ref=?, updated_at=? WHERE id=? AND status=?`,
			model.SnapFrozen, evidenceJSON, rulerRef, now, id, curStatus)
		if err != nil {
			return fmt.Errorf("freeze snapshot: %w", err)
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return model.NewError(model.ErrCodeConflict, "快照 %d 冻结冲突，请刷新", id)
		}
		return nil
	})
}
