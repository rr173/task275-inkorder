package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"task275-inkorder/internal/model"
)

// CandidateStore 提供笔顺候选及其偏序边的持久化。
type CandidateStore struct {
	g *WriteGuard
}

func NewCandidateStore(g *WriteGuard) *CandidateStore { return &CandidateStore{g: g} }

// Create 创建候选并写入全部偏序边（同一把写锁、同一事务）。
func (s *CandidateStore) Create(ctx context.Context, c *model.OrderCandidate, edges []model.CandidateEdge) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var id int64
	err := s.g.WithTx(ctx, func(tx *sql.Tx) error {
		res, err := tx.Exec(
			`INSERT INTO order_candidates(batch_id,version,status,score,conflict_reason,created_at,updated_at)
			 VALUES(?,?,?,?,?,?,?)`,
			c.BatchID, 1, c.Status, c.Score, c.ConflictReason, now, now)
		if err != nil {
			return fmt.Errorf("insert candidate: %w", err)
		}
		got, err := res.LastInsertId()
		if err != nil {
			return fmt.Errorf("candidate id: %w", err)
		}
		for _, e := range edges {
			if _, err := tx.Exec(
				`INSERT INTO candidate_edges(candidate_id,before_fragment_id,after_fragment_id,source,weight)
				 VALUES(?,?,?,?,?)`,
				got, e.BeforeFragmentID, e.AfterFragmentID, e.Source, e.Weight); err != nil {
				return fmt.Errorf("insert candidate edge: %w", err)
			}
		}
		id = got
		return nil
	})
	if err != nil {
		return 0, err
	}
	c.ID = id
	c.Version = 1
	c.CreatedAt = parseTime(now)
	c.UpdatedAt = c.CreatedAt
	return id, nil
}

// Get 按 ID 取候选（含版本）。
func (s *CandidateStore) Get(id int64) (*model.OrderCandidate, error) {
	var ct, ut timeText
	row := s.g.DB.QueryRow(
		`SELECT id,batch_id,version,status,score,conflict_reason,created_at,updated_at
		 FROM order_candidates WHERE id=?`, id)
	c := &model.OrderCandidate{}
	err := row.Scan(&c.ID, &c.BatchID, &c.Version, &c.Status, &c.Score, &c.ConflictReason, &ct, &ut)
	if err == sql.ErrNoRows {
		return nil, model.NewError(model.ErrCodeNotFound, "candidate %d not found", id)
	}
	if err != nil {
		return nil, fmt.Errorf("scan candidate: %w", err)
	}
	c.CreatedAt = ct.time()
	c.UpdatedAt = ut.time()
	return c, nil
}

// ListByBatch 列出批次候选（按 ID 倒序）。
func (s *CandidateStore) ListByBatch(batchID int64) ([]model.OrderCandidate, error) {
	rows, err := s.g.DB.Query(
		`SELECT id,batch_id,version,status,score,conflict_reason,created_at,updated_at
		 FROM order_candidates WHERE batch_id=? ORDER BY id DESC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list candidates: %w", err)
	}
	defer rows.Close()
	var out []model.OrderCandidate
	for rows.Next() {
		var c model.OrderCandidate
		var ct, ut timeText
		if err := rows.Scan(&c.ID, &c.BatchID, &c.Version, &c.Status, &c.Score, &c.ConflictReason, &ct, &ut); err != nil {
			return nil, fmt.Errorf("scan candidate row: %w", err)
		}
		c.CreatedAt = ct.time()
		c.UpdatedAt = ut.time()
		out = append(out, c)
	}
	return out, rows.Err()
}

// ListEdges 列出候选的偏序边（每次返回独立底层数组，避免调用方持有的切片被后续查询覆盖）。
func (s *CandidateStore) ListEdges(candidateID int64) ([]model.CandidateEdge, error) {
	rows, err := s.g.DB.Query(
		`SELECT id,candidate_id,before_fragment_id,after_fragment_id,source,weight
		 FROM candidate_edges WHERE candidate_id=? ORDER BY id`, candidateID)
	if err != nil {
		return nil, fmt.Errorf("list edges: %w", err)
	}
	defer rows.Close()
	var out []model.CandidateEdge
	for rows.Next() {
		var e model.CandidateEdge
		if err := rows.Scan(&e.ID, &e.CandidateID, &e.BeforeFragmentID, &e.AfterFragmentID, &e.Source, &e.Weight); err != nil {
			return nil, fmt.Errorf("scan edge: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	copied := make([]model.CandidateEdge, len(out))
	copy(copied, out)
	return copied, nil
}

// UpdateStatusCAS 带版本号乐观锁更新候选状态（裁决并发安全）。
// 读当前行（version/status）与条件更新在同一事务、同一把写锁内完成，
// 杜绝"一个裁决读到 v1、另一裁决读到 v2 后两者 CAS 都命中"的串行丢失更新。
// 返回 (更新后的候选, true, nil) 成功；返回 (nil, false, nil) 表示版本冲突。
func (s *CandidateStore) UpdateStatusCAS(ctx context.Context, id int64, expectVersion int, status model.CandidateStatus) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	var n int64
	err := s.g.WithTx(ctx, func(tx *sql.Tx) error {
		// 在事务内锁定读：SELECT 当前行，确认期望版本未被并发裁决推进。
		var curVersion int
		var curStatus model.CandidateStatus
		err := tx.QueryRow(
			`SELECT version, status FROM order_candidates WHERE id=?`, id).
			Scan(&curVersion, &curStatus)
		if err == sql.ErrNoRows {
			return model.NewError(model.ErrCodeNotFound, "candidate %d not found", id)
		}
		if err != nil {
			return fmt.Errorf("load candidate for cas: %w", err)
		}
		// 版本或状态机已被并发裁决推进，则放弃本次更新。
		if curVersion != expectVersion || !model.CanTransitionCandidate(curStatus, status) {
			return nil
		}
		res, err := tx.Exec(
			`UPDATE order_candidates SET status=?, version=version+1, updated_at=?
			 WHERE id=? AND version=?`,
			status, now, id, expectVersion)
		if err != nil {
			return fmt.Errorf("cas update candidate: %w", err)
		}
		n, err = res.RowsAffected()
		return err
	})
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
