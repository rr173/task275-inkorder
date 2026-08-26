package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// Open 打开（必要时创建）SQLite 数据库并执行建表迁移。
// dsn 为文件路径，":memory:" 表示内存库（仅测试用）。
func Open(dsn string) (*sql.DB, error) {
	if dsn != ":memory:" {
		if dir := filepath.Dir(dsn); dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("create db dir: %w", err)
			}
		}
	}
	// 事务内可能嵌套查询，需 >1 连接；busy_timeout 缓解写锁竞争。
	realDSN := dsn
	if realDSN == ":memory:" {
		// 多连接共享同一内存库（否则每个连接独立库，事务内查不到表）
		realDSN = "file:inkorder-shared?mode=memory&cache=shared"
	}
	if !strings.Contains(realDSN, "_pragma") {
		sep := "?"
		if strings.Contains(realDSN, "?") {
			sep = "&"
		}
		realDSN += sep + "_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", realDSN)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	db.SetMaxOpenConns(4) // SQLite 单写者，读并发用少量连接即可
	if err := migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

// migrate 建立全部表结构。幂等：重复执行不报错。
func migrate(db *sql.DB) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS batches (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			case_ref TEXT NOT NULL,
			title TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			status TEXT NOT NULL,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS layers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES batches(id),
			name TEXT NOT NULL,
			scan_ref TEXT NOT NULL DEFAULT '',
			width REAL NOT NULL,
			height REAL NOT NULL,
			is_base INTEGER NOT NULL DEFAULT 0,
			scale REAL NOT NULL DEFAULT 1,
			offset_x REAL NOT NULL DEFAULT 0,
			offset_y REAL NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS fragments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES batches(id),
			layer_id INTEGER NOT NULL REFERENCES layers(id),
			label TEXT NOT NULL,
			status TEXT NOT NULL,
			start_x REAL NOT NULL,
			start_y REAL NOT NULL,
			end_x REAL NOT NULL,
			end_y REAL NOT NULL,
			pressure REAL NOT NULL,
			calib_start_x REAL NOT NULL DEFAULT 0,
			calib_start_y REAL NOT NULL DEFAULT 0,
			calib_end_x REAL NOT NULL DEFAULT 0,
			calib_end_y REAL NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS observations (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL,
			fragment_id INTEGER NOT NULL REFERENCES fragments(id),
			kind TEXT NOT NULL,
			x REAL NOT NULL,
			y REAL NOT NULL,
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS crossings (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL,
			layer_id INTEGER NOT NULL,
			first_fragment_id INTEGER NOT NULL REFERENCES fragments(id),
			second_fragment_id INTEGER NOT NULL REFERENCES fragments(id),
			x REAL NOT NULL,
			y REAL NOT NULL,
			confidence REAL NOT NULL,
			evidence TEXT NOT NULL DEFAULT '',
			is_artifact INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS order_candidates (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES batches(id),
			version INTEGER NOT NULL DEFAULT 1,
			status TEXT NOT NULL,
			score REAL NOT NULL DEFAULT 0,
			conflict_reason TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS candidate_edges (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			candidate_id INTEGER NOT NULL REFERENCES order_candidates(id),
			before_fragment_id INTEGER NOT NULL,
			after_fragment_id INTEGER NOT NULL,
			source TEXT NOT NULL,
			weight REAL NOT NULL DEFAULT 0
		);`,
		`CREATE TABLE IF NOT EXISTS objections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			candidate_id INTEGER NOT NULL REFERENCES order_candidates(id),
			fragment_id INTEGER NOT NULL,
			kind TEXT NOT NULL,
			reason TEXT NOT NULL,
			created_at TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS snapshots (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			batch_id INTEGER NOT NULL REFERENCES batches(id),
			candidate_id INTEGER NOT NULL REFERENCES order_candidates(id),
			status TEXT NOT NULL,
			ruler_ref TEXT NOT NULL DEFAULT '',
			evidence_json TEXT NOT NULL DEFAULT '',
			note TEXT NOT NULL DEFAULT '',
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);`,
		`CREATE INDEX IF NOT EXISTS idx_fragments_batch ON fragments(batch_id);`,
		`CREATE INDEX IF NOT EXISTS idx_fragments_layer ON fragments(layer_id);`,
		`CREATE INDEX IF NOT EXISTS idx_crossings_batch ON crossings(batch_id);`,
		`CREATE INDEX IF NOT EXISTS idx_edges_candidate ON candidate_edges(candidate_id);`,
		`CREATE INDEX IF NOT EXISTS idx_observations_fragment ON observations(fragment_id);`,
		`CREATE INDEX IF NOT EXISTS idx_objections_candidate ON objections(candidate_id);`,
		`CREATE INDEX IF NOT EXISTS idx_snapshots_batch ON snapshots(batch_id);`,
	}
	for _, s := range stmts {
		if _, err := db.Exec(s); err != nil {
			return err
		}
	}
	return nil
}
