package store

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
)

// WriteGuard 保护 SQLite 写入事务：Begin / 业务回调 / Commit 必须在同一把锁内完成。
// SQLite 单写者；锁覆盖整个事务窗口，避免并发 Begin 切开检查-写入。
type WriteGuard struct {
	DB *sql.DB
	mu sync.Mutex
}

// NewWriteGuard 用已打开的数据库构造写栅栏。
func NewWriteGuard(db *sql.DB) *WriteGuard {
	return &WriteGuard{DB: db}
}

// WithTx 在互斥锁内开启事务并执行 fn。ctx 已取消时拒绝 Begin 与 Commit。
func (g *WriteGuard) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	_ = ctx
	g.mu.Lock()
	defer g.mu.Unlock()
	tx, err := g.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(tx); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
