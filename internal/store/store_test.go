package store

import (
	"context"
	"path/filepath"
	"testing"

	"task275-inkorder/internal/model"
)

// TestPersistAndRecover 验证 SQLite 持久化与重启恢复。
func TestPersistAndRecover(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	bID, err := NewBatchStore(db).Create(&model.Batch{CaseRef: "C1", Title: "T"})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}
	lID, err := NewLayerStore(db).Create(&model.Layer{
		BatchID: bID, Name: "L1", Width: 100, Height: 80, IsBase: true,
	})
	if err != nil {
		t.Fatalf("create layer: %v", err)
	}
	fID, err := NewFragmentStore(db).Create(&model.Fragment{
		BatchID: bID, LayerID: lID, Label: "A",
		StartX: 1, StartY: 2, EndX: 3, EndY: 4, Pressure: 0.5,
	})
	if err != nil {
		t.Fatalf("create fragment: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	// 重启
	db2, err := Open(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer db2.Close()

	b, err := NewBatchStore(db2).Get(bID)
	if err != nil {
		t.Fatalf("recover batch: %v", err)
	}
	if b.Status != model.BatchImporting {
		t.Errorf("batch status = %s", b.Status)
	}
	f, err := NewFragmentStore(db2).Get(fID)
	if err != nil {
		t.Fatalf("recover fragment: %v", err)
	}
	if f.Label != "A" || f.StartX != 1 {
		t.Errorf("fragment not recovered: %+v", f)
	}
	ls, err := NewLayerStore(db2).ListByBatch(bID)
	if err != nil || len(ls) != 1 || !ls[0].IsBase {
		t.Errorf("layers not recovered: %v %v", ls, err)
	}
}

// TestFragmentListCacheInvalidation 校正/状态变更后 ListByBatch 必须返回新值，
// 而非缓存中的旧坐标（回归：UpdateCalibration/UpdateStatus 未失效批次缓存）。
func TestFragmentListCacheInvalidation(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	bID, _ := NewBatchStore(db).Create(&model.Batch{CaseRef: "C", Title: "T"})
	lID, _ := NewLayerStore(db).Create(&model.Layer{BatchID: bID, Name: "L1", Width: 100, Height: 80, IsBase: true})
	fs := NewFragmentStore(db)
	fID, _ := fs.Create(&model.Fragment{BatchID: bID, LayerID: lID, Label: "A", StartX: 1, StartY: 2, EndX: 3, EndY: 4, Pressure: 0.5})

	// 首次列表，填充缓存，校正坐标应为零值
	if list, _ := fs.ListByBatch(bID); len(list) != 1 || list[0].CalibEndX != 0 || list[0].Status != model.FragmentRaw {
		t.Fatalf("pre-calib list = %+v", list)
	}

	// 校正后再次列表，必须反映校正坐标与状态推进
	if err := fs.UpdateCalibration(fID, 10, 20, 30, 40); err != nil {
		t.Fatalf("update calibration: %v", err)
	}
	list, err := fs.ListByBatch(bID)
	if err != nil || len(list) != 1 {
		t.Fatalf("list after calib: %v %v", list, err)
	}
	got := list[0]
	if got.CalibStartX != 10 || got.CalibStartY != 20 || got.CalibEndX != 30 || got.CalibEndY != 40 {
		t.Errorf("calib coords stale after calibration: %+v", got)
	}
	if got.Status != model.FragmentCalibrated {
		t.Errorf("status stale after calibration: %s", got.Status)
	}

	// 状态变更（伪影）后列表也应反映新状态
	if err := fs.UpdateStatus(fID, model.FragmentArtifact); err != nil {
		t.Fatalf("update status: %v", err)
	}
	list2, _ := fs.ListByBatch(bID)
	if list2[0].Status != model.FragmentArtifact {
		t.Errorf("status stale after update status: %s", list2[0].Status)
	}
}
func TestCandidateCAS(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	bs := NewBatchStore(db)
	bID, _ := bs.Create(&model.Batch{CaseRef: "C", Title: "T"})
	cs := NewCandidateStore(NewWriteGuard(db))
	cID, err := cs.Create(context.Background(), &model.OrderCandidate{BatchID: bID, Status: model.CandConsistent, Score: 0.9}, nil)
	if err != nil {
		t.Fatalf("create candidate: %v", err)
	}
	ok, err := cs.UpdateStatusCAS(context.Background(), cID, 1, model.CandConfirmed)
	if err != nil || !ok {
		t.Fatalf("cas update: ok=%v err=%v", ok, err)
	}
	// 版本已变，再用旧版本应失败
	ok, err = cs.UpdateStatusCAS(context.Background(), cID, 1, model.CandRejected)
	if err != nil {
		t.Fatalf("second cas err: %v", err)
	}
	if ok {
		t.Error("stale version should fail CAS")
	}
	c, _ := cs.Get(cID)
	if c.Status != model.CandConfirmed {
		t.Errorf("status = %s", c.Status)
	}
}
