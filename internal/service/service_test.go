package service

import (
	"context"
	"testing"

	"task275-inkorder/internal/model"
	"task275-inkorder/internal/store"
)

// TestFullFlow 端到端业务流：批次→层→片段→校正→交叉→重建→确认→快照→发布封存。
func TestFullFlow(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	app := NewApp(db)

	ctx := context.Background()
	batchSvc := NewBatchService(app)
	fragSvc := NewFragmentService(app)
	orderSvc := NewOrderService(app)
	snapSvc := NewSnapshotService(app)

	// 批次
	b, err := batchSvc.Create("CASE-T", "笔顺测试", "")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	if b.Status != model.BatchImporting {
		t.Fatalf("initial status = %s", b.Status)
	}

	// 层
	l1, err := fragSvc.AddLayer(b.ID, "L1", "a.tif", 1000, 800, true)
	if err != nil {
		t.Fatalf("add layer1: %v", err)
	}
	if !l1.IsBase {
		t.Fatal("first layer should be base")
	}
	if _, err := fragSvc.AddLayer(b.ID, "L2", "b.tif", 1000, 800, false); err != nil {
		t.Fatalf("add layer2: %v", err)
	}

	// 片段
	f1, err := fragSvc.AddFragment(b.ID, l1.ID, "A", 100, 100, 300, 100, 0.3)
	if err != nil {
		t.Fatalf("add A: %v", err)
	}
	f2, err := fragSvc.AddFragment(b.ID, l1.ID, "B", 200, 40, 200, 200, 0.62)
	if err != nil {
		t.Fatalf("add B: %v", err)
	}
	// 标签重复应拒绝
	if _, err := fragSvc.AddFragment(b.ID, l1.ID, "A", 0, 0, 1, 1, 0.5); err == nil {
		t.Error("duplicate label should fail")
	}

	// 校正
	n, err := fragSvc.CalibrateBatch(b.ID)
	if err != nil || n != 2 {
		t.Fatalf("calibrate: n=%d err=%v", n, err)
	}

	// 交叉
	if _, err := orderSvc.AddCrossing(ctx, b.ID, l1.ID, f1.ID, f2.ID, 200, 100, 0.85, "B covers A"); err != nil {
		t.Fatalf("add crossing: %v", err)
	}

	// 状态流转：importing → pending_rebuild
	if _, err := batchSvc.Rebuild(ctx, b.ID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	// 候选
	cand, err := orderSvc.RebuildCandidate(ctx, b.ID)
	if err != nil {
		t.Fatalf("rebuild candidate: %v", err)
	}
	if cand.Status != model.CandConsistent {
		t.Fatalf("candidate status = %s (%s)", cand.Status, cand.ConflictReason)
	}

	// 确认前可登记异议；确认后不可再登记
	if _, err := orderSvc.AddObjection(cand.ID, f2.ID, model.ObjPressure, "笔压判读存疑"); err != nil {
		t.Fatalf("add objection: %v", err)
	}
	if _, err := orderSvc.ConfirmCandidate(ctx, cand.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := orderSvc.AddObjection(cand.ID, f2.ID, model.ObjGeometry, "确认后异议应拒绝"); err == nil {
		t.Error("objection after adjudication should fail")
	}
	if _, err := batchSvc.ToReview(ctx, b.ID); err != nil {
		t.Fatalf("to review: %v", err)
	}

	// 快照
	sn, err := snapSvc.CreateDraft(ctx, b.ID, cand.ID, "结论")
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if _, err := snapSvc.Share(sn.ID); err != nil {
		t.Fatalf("share: %v", err)
	}
	if _, err := snapSvc.Freeze(ctx, sn.ID); err != nil {
		t.Fatalf("freeze: %v", err)
	}

	// 发布封存
	if _, err := batchSvc.Publish(ctx, b.ID); err != nil {
		t.Fatalf("publish: %v", err)
	}
	if _, err := batchSvc.Seal(ctx, b.ID); err != nil {
		t.Fatalf("seal: %v", err)
	}

	// 非法流转应拒绝
	if _, err := batchSvc.Seal(ctx, b.ID); err != nil {
		t.Fatalf("seal should be idempotent: %v", err)
	}
	b2, _ := app.Batches.Get(b.ID)
	if b2.Status != model.BatchSealed {
		t.Fatalf("final status = %s", b2.Status)
	}
}

// TestBatchTransitionGuards 验证流转前置条件。
func TestBatchTransitionGuards(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	app := NewApp(db)
	batchSvc := NewBatchService(app)

	b, _ := batchSvc.Create("C", "T", "")
	// 无片段时不能进入待重建
	if _, err := batchSvc.Rebuild(context.Background(), b.ID); err == nil {
		t.Error("rebuild without fragments should fail")
	}
}

// TestFreezeRespectsCanceledContext 回归：研究者取消冻结请求时，快照不得变为冻结。
// 修复前各层把请求 ctx 替换为 context.Background()，取消信号被丢弃，事务照常提交。
func TestFreezeRespectsCanceledContext(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	app := NewApp(db)
	ctx := context.Background()
	batchSvc := NewBatchService(app)
	fragSvc := NewFragmentService(app)
	orderSvc := NewOrderService(app)
	snapSvc := NewSnapshotService(app)

	b, err := batchSvc.Create("CASE-X", "取消冻结", "")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	l1, err := fragSvc.AddLayer(b.ID, "L1", "a.tif", 1000, 800, true)
	if err != nil {
		t.Fatalf("add layer1: %v", err)
	}
	if _, err := fragSvc.AddFragment(b.ID, l1.ID, "A", 100, 100, 300, 100, 0.3); err != nil {
		t.Fatalf("add A: %v", err)
	}
	if _, err := fragSvc.AddFragment(b.ID, l1.ID, "B", 200, 40, 200, 200, 0.62); err != nil {
		t.Fatalf("add B: %v", err)
	}
	if _, err := fragSvc.CalibrateBatch(b.ID); err != nil {
		t.Fatalf("calibrate: %v", err)
	}
	fragSvcAsFragment := func(label string) *model.Fragment {
		fs, _ := app.Fragments.ListByBatch(b.ID)
		for i := range fs {
			if fs[i].Label == label {
				return &fs[i]
			}
		}
		t.Fatalf("fragment %s missing", label)
		return nil
	}
	fa, fb := fragSvcAsFragment("A"), fragSvcAsFragment("B")
	if _, err := orderSvc.AddCrossing(ctx, b.ID, l1.ID, fa.ID, fb.ID, 200, 100, 0.85, "B covers A"); err != nil {
		t.Fatalf("add crossing: %v", err)
	}
	if _, err := batchSvc.Rebuild(ctx, b.ID); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	cand, err := orderSvc.RebuildCandidate(ctx, b.ID)
	if err != nil {
		t.Fatalf("rebuild candidate: %v", err)
	}
	if _, err := orderSvc.ConfirmCandidate(ctx, cand.ID); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if _, err := batchSvc.ToReview(ctx, b.ID); err != nil {
		t.Fatalf("to review: %v", err)
	}
	sn, err := snapSvc.CreateDraft(ctx, b.ID, cand.ID, "取消冻结")
	if err != nil {
		t.Fatalf("create snapshot: %v", err)
	}
	if _, err := snapSvc.Share(sn.ID); err != nil {
		t.Fatalf("share: %v", err)
	}

	// 研究者在提交前取消请求：用已取消的 ctx 发起冻结。
	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := snapSvc.Freeze(canceledCtx, sn.ID); err == nil {
		t.Fatal("freeze with canceled context should fail, not silently freeze")
	}
	got, err := snapSvc.Get(sn.ID)
	if err != nil {
		t.Fatalf("reload snapshot: %v", err)
	}
	if got.Status == model.SnapFrozen {
		t.Fatalf("snapshot must NOT be frozen after canceled request; got %s", got.Status)
	}
	if got.EvidenceJSON != "" {
		t.Fatalf("canceled freeze must not write evidence; got %q", got.EvidenceJSON)
	}
}
