package service

import (
	"context"
	"database/sql"
	"fmt"

	"task275-inkorder/internal/model"
)

// BatchService 处理鉴定批次的生命周期。
type BatchService struct {
	app *App
}

func NewBatchService(app *App) *BatchService { return &BatchService{app: app} }

// Create 创建批次（状态 importing）。
func (s *BatchService) Create(caseRef, title, description string) (*model.Batch, error) {
	if title == "" {
		return nil, model.NewError(model.ErrCodeInvalid, "title 不能为空")
	}
	b := &model.Batch{CaseRef: caseRef, Title: title, Description: description}
	id, err := s.app.Batches.Create(b)
	if err != nil {
		return nil, fmt.Errorf("save batch: %w", err)
	}
	got, err := s.app.Batches.Get(id)
	if err != nil {
		return nil, fmt.Errorf("reload batch: %w", err)
	}
	return got, nil
}

// Get 取批次。
func (s *BatchService) Get(id int64) (*model.Batch, error) {
	b, err := s.app.Batches.Get(id)
	if err != nil {
		return nil, fmt.Errorf("load batch: %v", err)
	}
	return b, nil
}

// List 列批次。
func (s *BatchService) List(limit int) ([]model.Batch, error) { return s.app.Batches.List(limit) }

// Transition 校验并执行批次状态流转（写锁+事务内完成，含配套动作）。
func (s *BatchService) Transition(ctx context.Context, id int64, to model.BatchStatus) (*model.Batch, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b, err := s.app.Batches.Get(id)
	if err != nil {
		return nil, fmt.Errorf("load batch for transition: %w", err)
	}
	if b.Status == to {
		return b, nil
	}
	if !model.CanTransition(b.Status, to) {
		return nil, model.NewError(model.ErrCodeBadState, "批次状态 %s 不能流转到 %s", b.Status, to)
	}

	err = s.app.Guard.WithTx(ctx, func(tx *sql.Tx) error {
		switch to {
		case model.BatchPendingRebuild:
			fs, err := s.app.Fragments.ListByBatch(id)
			if err != nil {
				return fmt.Errorf("list fragments for rebuild gate: %w", err)
			}
			active := 0
			for _, f := range fs {
				if f.Active() {
					active++
				}
			}
			if active == 0 {
				return model.NewError(model.ErrCodeBadState, "批次无活跃片段，无法进入待重建")
			}
		case model.BatchPendingReview:
			cs, err := s.app.Candidates.ListByBatch(id)
			if err != nil {
				return fmt.Errorf("list candidates for review gate: %w", err)
			}
			if len(cs) == 0 {
				return model.NewError(model.ErrCodeBadState, "批次无笔顺候选，无法进入待复核")
			}
		case model.BatchPublished:
			cs, err := s.app.Candidates.ListByBatch(id)
			if err != nil {
				return fmt.Errorf("list candidates for publish gate: %w", err)
			}
			confirmed := false
			for _, c := range cs {
				if c.Status == model.CandConfirmed {
					confirmed = true
					break
				}
			}
			if !confirmed {
				return model.NewError(model.ErrCodeBadState, "批次无已确认候选，无法发布")
			}
			snaps, err := s.app.Snapshots.ListByBatch(id)
			if err != nil {
				return fmt.Errorf("list snapshots for publish gate: %w", err)
			}
			frozen := false
			for _, sn := range snaps {
				if sn.Status == model.SnapFrozen {
					frozen = true
					break
				}
			}
			if !frozen {
				return model.NewError(model.ErrCodeBadState, "批次无冻结快照，无法发布")
			}
		}
		if err := s.app.Batches.UpdateStatus(tx, id, to); err != nil {
			return fmt.Errorf("update batch status: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	got, err := s.app.Batches.Get(id)
	if err != nil {
		return nil, fmt.Errorf("reload batch after transition: %w", err)
	}
	return got, nil
}

// Rebuild 便捷方法：importing → pending_rebuild。
func (s *BatchService) Rebuild(ctx context.Context, id int64) (*model.Batch, error) {
	return s.Transition(ctx, id, model.BatchPendingRebuild)
}

// ToReview 便捷方法：pending_rebuild → pending_review。
func (s *BatchService) ToReview(ctx context.Context, id int64) (*model.Batch, error) {
	return s.Transition(ctx, id, model.BatchPendingReview)
}

// Publish 便捷方法：pending_review → published。
func (s *BatchService) Publish(ctx context.Context, id int64) (*model.Batch, error) {
	return s.Transition(ctx, id, model.BatchPublished)
}

// Seal 便捷方法：published → sealed。
func (s *BatchService) Seal(ctx context.Context, id int64) (*model.Batch, error) {
	return s.Transition(ctx, id, model.BatchSealed)
}
