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
