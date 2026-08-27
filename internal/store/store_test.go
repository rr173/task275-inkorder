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

// TestCandidateCAS 验证乐观锁裁决。
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

// TestFragmentListByBatchIndependentAcrossBatches 复现"标伪影后重建笔顺，
// 列其它批次片段时列表被截短或串到上一批"的缺陷：
// scanFragments 曾复用包级缓冲，第二次 List 会覆盖第一次仍被持有的切片底层数组。
func TestFragmentListByBatchIndependentAcrossBatches(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	fs := NewFragmentStore(db)
	ls := NewLayerStore(db)

	// 批次 A：3 个片段；批次 B：1 个片段（更少，恰好暴露"截短/串数据"）。
	bA, err := NewBatchStore(db).Create(&model.Batch{CaseRef: "A", Title: "TA"})
	if err != nil {
		t.Fatalf("create batch A: %v", err)
	}
	lA, err := ls.Create(&model.Layer{BatchID: bA, Name: "LA", Width: 10, Height: 10, IsBase: true})
	if err != nil {
		t.Fatalf("create layer A: %v", err)
	}
	for i, label := range []string{"A1", "A2", "A3"} {
		if _, err := fs.Create(&model.Fragment{
			BatchID: bA, LayerID: lA, Label: label,
			StartX: float64(i), StartY: 0, EndX: float64(i), EndY: 1, Pressure: 0.5,
		}); err != nil {
			t.Fatalf("create %s: %v", label, err)
		}
	}
	bB, err := NewBatchStore(db).Create(&model.Batch{CaseRef: "B", Title: "TB"})
	if err != nil {
		t.Fatalf("create batch B: %v", err)
	}
	lB, err := ls.Create(&model.Layer{BatchID: bB, Name: "LB", Width: 10, Height: 10, IsBase: true})
	if err != nil {
		t.Fatalf("create layer B: %v", err)
	}
	if _, err := fs.Create(&model.Fragment{
		BatchID: bB, LayerID: lB, Label: "B1",
		StartX: 9, StartY: 0, EndX: 9, EndY: 1, Pressure: 0.5,
	}); err != nil {
		t.Fatalf("create B1: %v", err)
	}

	// 先取批次 A 的片段（仍持有），再取批次 B（更短）。
	// 若共享底层数组，批次 A 的切片会被批次 B 的数据覆盖，尾部残留旧值。
	aFrag, err := fs.ListByBatch(bA)
	if err != nil {
		t.Fatalf("list A: %v", err)
	}
	if len(aFrag) != 3 {
		t.Fatalf("list A: got %d want 3", len(aFrag))
	}
	bFrag, err := fs.ListByBatch(bB)
	if err != nil {
		t.Fatalf("list B: %v", err)
	}
	if len(bFrag) != 1 {
		t.Fatalf("list B: got %d want 1", len(bFrag))
	}

	// 批次 A 的结果不得被批次 B 的查询污染。
	if len(aFrag) != 3 {
		t.Fatalf("list A corrupted after list B: len=%d", len(aFrag))
	}
	want := []string{"A1", "A2", "A3"}
	for i, f := range aFrag {
		if f.Label != want[i] {
			t.Errorf("list A[%d].Label = %q, want %q (串到批次 B 的数据)", i, f.Label, want[i])
		}
		if f.BatchID != bA {
			t.Errorf("list A[%d].BatchID = %d, want %d", i, f.BatchID, bA)
		}
	}
	if bFrag[0].Label != "B1" {
		t.Errorf("list B[0].Label = %q, want B1", bFrag[0].Label)
	}
}
