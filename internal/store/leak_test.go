package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"task275-inkorder/internal/model"
)

// TestFailedTxDoesNotLeakWriteLock 复现"一次失败的批次流转之后，紧接着注册扫描层会写不进去"。
//
// WriteGuard.WithTx 在回调失败时若不 Rollback，事务悬挂，SQLite RESERVED 写锁泄漏。
// 随后绕过 guard 的直写（LayerStore.Create 等）拿不到写锁，busy_timeout 到期后返回
// SQLITE_BUSY，表现即为"写不进去"。本测试在失败事务之后立即直写一个扫描层，
// 要求它及时成功；修复前会超时或报错。
func TestFailedTxDoesNotLeakWriteLock(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	g := NewWriteGuard(db)
	bs := NewBatchStore(db)
	ls := NewLayerStore(db)

	bID, err := bs.Create(&model.Batch{CaseRef: "C1", Title: "T"})
	if err != nil {
		t.Fatalf("create batch: %v", err)
	}

	// 模拟一次失败的批次流转：回调返回 gate 拒绝错误（如 BatchService.Transition 的
	// "批次无活跃片段"）。事务被 Begin + 探测 UPDATE 后即失败返回。
	if err := g.WithTx(context.Background(), func(tx *sql.Tx) error {
		return model.NewError(model.ErrCodeBadState, "模拟批次流转 gate 拒绝")
	}); err == nil {
		t.Fatalf("expected failed transition error")
	}

	// 紧接着直写注册一个扫描层（不经 guard）。
	// 修复前：悬挂事务持有 RESERVED 写锁，LayerStore.Create 拿不到写锁而 SQLITE_BUSY。
	done := make(chan error, 1)
	go func() {
		_, e := ls.Create(&model.Layer{
			BatchID: bID, Name: "L-after-fail", Width: 100, Height: 80, IsBase: true,
		})
		done <- e
	}()
	select {
	case e := <-done:
		if e != nil {
			t.Fatalf("create layer after failed tx should succeed, got: %v", e)
		}
	case <-time.After(10 * time.Second):
		t.Fatalf("create layer after failed tx timed out — write lock leaked")
	}
}
