// Package smoke 实现 --smoke-test 契约：真实创建业务实体、执行核心算法、
// 关闭并重新打开数据库验证持久化与重启恢复，最后以 0 退出码结束。
package smoke

import (
	"context"
	"fmt"
	"os"

	"task275-inkorder/internal/geometry"
	"task275-inkorder/internal/model"
	"task275-inkorder/internal/service"
	"task275-inkorder/internal/store"
)

// Run 执行完整自检流程。
// 场景：两页连续笔迹 A、B、C 存在交叉覆盖证据（A 先于 B、B 先于 C），
// 研究者确认候选后发布冻结快照；随后重启验证所有状态恢复。
func Run(dbPath string) error {
	// 使用独立临时库，避免污染正常数据
	if dbPath == "" || dbPath == ":memory:" {
		tmp, err := os.CreateTemp("", "task275-smoke-*.db")
		if err != nil {
			return err
		}
		tmp.Close()
		os.Remove(tmp.Name())
		dbPath = tmp.Name() + ".sqlite"
		defer os.Remove(dbPath)
	}

	// ---- 第一轮：创建并写入 ----
	db, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	app := service.NewApp(db)

	bID, err := app.Batches.Create(&model.Batch{
		CaseRef: "CASE-SMOKE-001", Title: "交叉笔顺自检批次",
		Description: "验证 A→B→C 书写顺序的交叉覆盖证据闭环",
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("create batch: %w", err)
	}
	b, err := app.Batches.Get(bID)
	if err != nil {
		db.Close()
		return fmt.Errorf("get batch: %w", err)
	}

	// 扫描层：基准层 L1 + 对照层 L2
	l1ID, err := app.Layers.Create(&model.Layer{
		BatchID: b.ID, Name: "L1-基准", ScanRef: "scan-a.tif",
		Width: 1000, Height: 800, IsBase: true,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("create layer1: %w", err)
	}
	l1, err := app.Layers.Get(l1ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("get layer1: %w", err)
	}
	l2ID, err := app.Layers.Create(&model.Layer{
		BatchID: b.ID, Name: "L2-对照", ScanRef: "scan-b.tif",
		Width: 1000, Height: 800, IsBase: false,
		Scale: 1.05, OffsetX: 12, OffsetY: -8,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("create layer2: %w", err)
	}
	l2, err := app.Layers.Get(l2ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("get layer2: %w", err)
	}
	_ = l2

	// 片段 A、B、C（L1 基准层，坐标即基准坐标）
	fAID, err := app.Fragments.Create(&model.Fragment{
		BatchID: b.ID, LayerID: l1.ID, Label: "A",
		StartX: 100, StartY: 100, EndX: 300, EndY: 100, Pressure: 0.30,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("create fragment A: %w", err)
	}
	fA, err := app.Fragments.Get(fAID)
	if err != nil {
		db.Close()
		return fmt.Errorf("get fragment A: %w", err)
	}
	fBID, err := app.Fragments.Create(&model.Fragment{
		BatchID: b.ID, LayerID: l1.ID, Label: "B",
		StartX: 200, StartY: 40, EndX: 200, EndY: 200, Pressure: 0.62,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("create fragment B: %w", err)
	}
	fB, err := app.Fragments.Get(fBID)
	if err != nil {
		db.Close()
		return fmt.Errorf("get fragment B: %w", err)
	}
	fCID, err := app.Fragments.Create(&model.Fragment{
		BatchID: b.ID, LayerID: l1.ID, Label: "C",
		StartX: 250, StartY: 60, EndX: 250, EndY: 220, Pressure: 0.55,
	})
	if err != nil {
		db.Close()
		return fmt.Errorf("create fragment C: %w", err)
	}
	fC, err := app.Fragments.Get(fCID)
	if err != nil {
		db.Close()
		return fmt.Errorf("get fragment C: %w", err)
	}

	// 校正（基准层直接复制坐标）
	if err := app.Fragments.UpdateCalibration(fA.ID, fA.StartX, fA.StartY, fA.EndX, fA.EndY); err != nil {
		db.Close()
		return fmt.Errorf("calibrate A: %w", err)
	}
	if err := app.Fragments.UpdateCalibration(fB.ID, fB.StartX, fB.StartY, fB.EndX, fB.EndY); err != nil {
		db.Close()
		return fmt.Errorf("calibrate B: %w", err)
	}
	if err := app.Fragments.UpdateCalibration(fC.ID, fC.StartX, fC.StartY, fC.EndX, fC.EndY); err != nil {
		db.Close()
		return fmt.Errorf("calibrate C: %w", err)
	}

	// 交叉覆盖证据：A 先于 B（B 覆盖 A）、B 先于 C（C 覆盖 B）→ A→B→C
	if _, err := app.Crossings.Create(&model.Crossing{
		BatchID: b.ID, LayerID: l1.ID,
		FirstFragmentID: fA.ID, SecondFragmentID: fB.ID,
		X: 200, Y: 100, Confidence: 0.85, Evidence: "B 覆盖 A 交叉处",
	}); err != nil {
		db.Close()
		return fmt.Errorf("create crossing A<B: %w", err)
	}
	if _, err := app.Crossings.Create(&model.Crossing{
		BatchID: b.ID, LayerID: l1.ID,
		FirstFragmentID: fB.ID, SecondFragmentID: fC.ID,
		X: 250, Y: 140, Confidence: 0.80, Evidence: "C 覆盖 B 交叉处",
	}); err != nil {
		db.Close()
		return fmt.Errorf("create crossing B<C: %w", err)
	}

	// 停顿观测：C 终点附近有停顿，辅助证据
	if _, err := app.Observations.Create(&model.Observation{
		BatchID: b.ID, FragmentID: fC.ID,
		Kind: model.ObsPause, X: 250, Y: 220, Note: "末笔停顿",
	}); err != nil {
		db.Close()
		return fmt.Errorf("create observation: %w", err)
	}

	// 笔顺重建：批次转入待重建 → 生成候选
	if err := app.Batches.UpdateStatusDirect(b.ID, model.BatchPendingRebuild); err != nil {
		db.Close()
		return fmt.Errorf("batch to pending_rebuild: %w", err)
	}
	ctx := context.Background()
	orderSvc := service.NewOrderService(app)
	cand, err := orderSvc.RebuildCandidate(ctx, b.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("rebuild candidate: %w", err)
	}
	edges, err := app.Candidates.ListEdges(cand.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("list edges: %w", err)
	}
	if len(edges) == 0 {
		db.Close()
		return fmt.Errorf("candidate has no edges")
	}
	if cand.Status == model.CandConflict {
		db.Close()
		return fmt.Errorf("unexpected conflict: %s", cand.ConflictReason)
	}

	// 确认候选，批次转入待复核
	confirmed, err := orderSvc.ConfirmCandidate(ctx, cand.ID)
	if err != nil {
		db.Close()
		return fmt.Errorf("confirm candidate: %w", err)
	}
	if confirmed.Status != model.CandConfirmed {
		db.Close()
		return fmt.Errorf("candidate not confirmed")
	}
	batchSvc := service.NewBatchService(app)
	if _, err := batchSvc.ToReview(ctx, b.ID); err != nil {
		db.Close()
		return fmt.Errorf("batch to review: %w", err)
	}

	// 创建并冻结快照，发布并封存批次
	snapSvc := service.NewSnapshotService(app)
	snap, err := snapSvc.CreateDraft(ctx, b.ID, cand.ID, "自检快照")
	if err != nil {
		db.Close()
		return fmt.Errorf("create snapshot: %w", err)
	}
	if _, err := snapSvc.Share(snap.ID); err != nil {
		db.Close()
		return fmt.Errorf("share snapshot: %w", err)
	}
	if _, err := snapSvc.Freeze(ctx, snap.ID); err != nil {
		db.Close()
		return fmt.Errorf("freeze snapshot: %w", err)
	}

	if _, err := batchSvc.Publish(ctx, b.ID); err != nil {
		db.Close()
		return fmt.Errorf("batch publish: %w", err)
	}
	if _, err := batchSvc.Seal(ctx, b.ID); err != nil {
		db.Close()
		return fmt.Errorf("batch seal: %w", err)
	}

	// 记录用于重启恢复验证的基线
	baseline := struct {
		BatchID     int64
		CandidateID int64
		SnapshotID  int64
	}{b.ID, cand.ID, snap.ID}

	db.Close()

	// ---- 第二轮：重启恢复验证 ----
	db2, err := store.Open(dbPath)
	if err != nil {
		return fmt.Errorf("reopen db: %w", err)
	}
	defer db2.Close()
	app2 := service.NewApp(db2)

	b2, err := app2.Batches.Get(baseline.BatchID)
	if err != nil {
		return fmt.Errorf("recover batch: %w", err)
	}
	if b2.Status != model.BatchSealed {
		return fmt.Errorf("batch status not recovered: %s", b2.Status)
	}
	c2, err := app2.Candidates.Get(baseline.CandidateID)
	if err != nil {
		return fmt.Errorf("recover candidate: %w", err)
	}
	if c2.Status != model.CandConfirmed {
		return fmt.Errorf("candidate status not recovered: %s", c2.Status)
	}
	s2, err := app2.Snapshots.Get(baseline.SnapshotID)
	if err != nil {
		return fmt.Errorf("recover snapshot: %w", err)
	}
	if s2.Status != model.SnapFrozen {
		return fmt.Errorf("snapshot status not recovered: %s", s2.Status)
	}
	fs2, err := app2.Fragments.ListByBatch(baseline.BatchID)
	if err != nil {
		return fmt.Errorf("recover fragments: %w", err)
	}
	if len(fs2) != 3 {
		return fmt.Errorf("fragments not recovered: got %d want 3", len(fs2))
	}

	// 几何判定交叉点校验：A 与 B 的交叉点应在 (200,100) 附近
	judge := geometry.NewCrossJudge()
	fa := fragmentByID(fs2, fA.ID)
	fb := fragmentByID(fs2, fB.ID)
	if fa == nil || fb == nil {
		return fmt.Errorf("recovered fragments missing A/B")
	}
	ev := judge.Judge(fa, fb)
	if !ev.Intersect {
		return fmt.Errorf("geometry judge: expected intersection after recovery")
	}
	if geometry.Distance(geometry.Point{X: ev.X, Y: ev.Y}, geometry.Point{X: 200, Y: 100}) > 2 {
		return fmt.Errorf("geometry judge: intersection at (%.1f,%.1f), want near (200,100)", ev.X, ev.Y)
	}
	return nil
}

func fragmentByID(fs []model.Fragment, id int64) *model.Fragment {
	for i := range fs {
		if fs[i].ID == id {
			return &fs[i]
		}
	}
	return nil
}
