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

// TestAutoCrossMissingFragment 验证对不存在的片段做自动交叉判定时返回明确的
// 领域错误而非 panic，且不会误创建交叉证据。
func TestAutoCrossMissingFragment(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()
	app := NewApp(db)
	batchSvc := NewBatchService(app)
	fragSvc := NewFragmentService(app)
	orderSvc := NewOrderService(app)

	ctx := context.Background()
	b, err := batchSvc.Create("CASE-M", "缺失片段自检", "")
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	l1, err := fragSvc.AddLayer(b.ID, "L1", "a.tif", 1000, 800, true)
	if err != nil {
		t.Fatalf("add layer: %v", err)
	}
	f1, err := fragSvc.AddFragment(b.ID, l1.ID, "A", 100, 100, 300, 100, 0.3)
	if err != nil {
		t.Fatalf("add A: %v", err)
	}
	if _, err := fragSvc.CalibrateBatch(b.ID); err != nil {
		t.Fatalf("calibrate: %v", err)
	}

	const ghostID int64 = 9999
	// 第二个片段不存在：应返回 NOT_FOUND，不 panic、不误建交叉
	_, err = orderSvc.AutoCross(ctx, b.ID, l1.ID, f1.ID, ghostID)
	if err == nil {
		t.Fatal("auto cross with missing fragment should fail, got nil")
	}
	if ae := model.AsAppError(err); ae == nil || ae.Code != model.ErrCodeNotFound {
		t.Fatalf("auto cross with missing fragment should surface NOT_FOUND, got %v", err)
	}

	// 反过来：第一个片段不存在
	_, err = orderSvc.AutoCross(ctx, b.ID, l1.ID, ghostID, f1.ID)
	if err == nil {
		t.Fatal("auto cross with missing first fragment should fail, got nil")
	}
	if ae := model.AsAppError(err); ae == nil || ae.Code != model.ErrCodeNotFound {
		t.Fatalf("auto cross with missing first fragment should surface NOT_FOUND, got %v", err)
	}

	// 两侧都不存在
	_, err = orderSvc.AutoCross(ctx, b.ID, l1.ID, ghostID, ghostID+1)
	if err == nil {
		t.Fatal("auto cross with both fragments missing should fail, got nil")
	}
	if ae := model.AsAppError(err); ae == nil || ae.Code != model.ErrCodeNotFound {
		t.Fatalf("auto cross with both fragments missing should surface NOT_FOUND, got %v", err)
	}

	// 确保失败未误创建任何交叉证据
	cs, err := app.Crossings.ListByBatch(b.ID)
	if err != nil {
		t.Fatalf("list crossings: %v", err)
	}
	if len(cs) != 0 {
		t.Fatalf("no crossing should have been created, got %d", len(cs))
	}
}
