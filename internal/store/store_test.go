package store

import (
	"context"
	"path/filepath"
	"sync"
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

// TestCandidateEdgesNoCrosstalk 验证 ListEdges 按 candidate_id 返回各自边，
// 不被其他候选的写入串扰（回归：edgeView 共享缓存导致边串到别的写入结果上）。
func TestCandidateEdgesNoCrosstalk(t *testing.T) {
	db, err := Open(":memory:")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	bs := NewBatchStore(db)
	bID, _ := bs.Create(&model.Batch{CaseRef: "C", Title: "T"})
	cs := NewCandidateStore(NewWriteGuard(db))

	// 候选 1 的边：1→2
	cID1, err := cs.Create(context.Background(),
		&model.OrderCandidate{BatchID: bID, Status: model.CandConsistent, Score: 0.9},
		[]model.CandidateEdge{{BeforeFragmentID: 1, AfterFragmentID: 2, Source: model.EdgeFromCrossing, Weight: 0.8}})
	if err != nil {
		t.Fatalf("create candidate1: %v", err)
	}
	// 候选 2 的边：3→4
	cID2, err := cs.Create(context.Background(),
		&model.OrderCandidate{BatchID: bID, Status: model.CandConsistent, Score: 0.5},
		[]model.CandidateEdge{{BeforeFragmentID: 3, AfterFragmentID: 4, Source: model.EdgeManual, Weight: 0.1}})
	if err != nil {
		t.Fatalf("create candidate2: %v", err)
	}

	// 再创建候选 2 之后，查候选 1 的边仍应是 1→2（而非候选 2 的 3→4）。
	e1, err := cs.ListEdges(cID1)
	if err != nil {
		t.Fatalf("list edges1: %v", err)
	}
	if len(e1) != 1 || e1[0].BeforeFragmentID != 1 || e1[0].AfterFragmentID != 2 {
		t.Errorf("candidate1 edges crosstalked: %+v", e1)
	}

	e2, err := cs.ListEdges(cID2)
	if err != nil {
		t.Fatalf("list edges2: %v", err)
	}
	if len(e2) != 1 || e2[0].BeforeFragmentID != 3 || e2[0].AfterFragmentID != 4 {
		t.Errorf("candidate2 edges crosstalked: %+v", e2)
	}

	// 两次返回的切片应互不影响（独立底层数组）。
	e1again, _ := cs.ListEdges(cID1)
	e2again, _ := cs.ListEdges(cID2)
	if &e1again[:1][0] == &e2again[:1][0] {
		t.Error("ListEdges returned shared backing array")
	}
}

// TestCandidateCASConcurrentConfirmReject 验证并发确认/否决同一候选时，
// 至多一方成功（回归：读 v1 的裁决与读 v2 的裁决各自 CAS 命中，导致两次裁决都成功）。
func TestCandidateCASConcurrentConfirmReject(t *testing.T) {
	for iter := 0; iter < 200; iter++ {
		db, err := Open(":memory:")
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		bs := NewBatchStore(db)
		bID, _ := bs.Create(&model.Batch{CaseRef: "C", Title: "T"})
		cs := NewCandidateStore(NewWriteGuard(db))
		cID, err := cs.Create(context.Background(),
			&model.OrderCandidate{BatchID: bID, Status: model.CandConsistent, Score: 0.9}, nil)
		if err != nil {
			t.Fatalf("create candidate: %v", err)
		}

		type res struct {
			ok bool
		}
		results := make([]res, 2)
		var wg sync.WaitGroup
		wg.Add(2)
		for i := 0; i < 2; i++ {
			i := i
			go func() {
				defer wg.Done()
				var to model.CandidateStatus
				if i == 0 {
					to = model.CandConfirmed
				} else {
					to = model.CandRejected
				}
				c, err := cs.Get(cID)
				if err != nil {
					return
				}
				ok, err := cs.UpdateStatusCAS(context.Background(), cID, c.Version, to)
				if err == nil {
					results[i].ok = ok
				}
			}()
		}
		wg.Wait()
		if results[0].ok && results[1].ok {
			t.Fatalf("iter %d: both confirm and reject succeeded (lost update)", iter)
		}
		db.Close()
	}
}
