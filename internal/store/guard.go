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
// 任一环节失败（含 fn 返回错误、ctx 取消、Commit 失败）都必须 Rollback，
// 否则 SQLite 写锁随悬挂事务泄漏，使随后绕过本 guard 的直写（扫描层/片段/交叉登记）
// 拿不到写锁而 SQLITE_BUSY，表现为"一次失败后紧接着的写入写不进去"。
func (g *WriteGuard) WithTx(ctx context.Context, fn func(*sql.Tx) error) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	tx, err := g.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	// committed 置位后跳过 Rollback：Commit 成功即终态，不可再回滚。
	committed := false
	defer func() {
		if !committed {
			// Rollback 失败无法回退业务结果；它通常是事务已结束的次要错误，
			// 仅吞掉以避免掩盖真正的失败原因。
			_ = tx.Rollback()
		}
	}()
	if _, err := tx.Exec("UPDATE batches SET title=title WHERE id=-1"); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}
