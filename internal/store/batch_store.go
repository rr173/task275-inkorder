package store

import (
	"database/sql"
	"time"

	"task275-inkorder/internal/model"
)

// BatchStore 提供鉴定批次的持久化读写。
type BatchStore struct {
	db *sql.DB
}

func NewBatchStore(db *sql.DB) *BatchStore { return &BatchStore{db: db} }

// Create 创建批次，状态固定为 importing。
func (s *BatchStore) Create(b *model.Batch) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`INSERT INTO batches(case_ref,title,description,status,created_at,updated_at)
		 VALUES(?,?,?,?,?,?)`,
		b.CaseRef, b.Title, b.Description, model.BatchImporting, now, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	b.ID = id
	b.Status = model.BatchImporting
	b.CreatedAt = parseTime(now)
	b.UpdatedAt = b.CreatedAt
	return id, nil
}

// Get 按 ID 取批次。
func (s *BatchStore) Get(id int64) (*model.Batch, error) {
	var ct, ut timeText
	row := s.db.QueryRow(
		`SELECT id,case_ref,title,description,status,created_at,updated_at FROM batches WHERE id=?`, id)
	b := &model.Batch{}
	err := row.Scan(&b.ID, &b.CaseRef, &b.Title, &b.Description, &b.Status, &ct, &ut)
	if err == sql.ErrNoRows {
		return nil, model.NewError(model.ErrCodeNotFound, "batch %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	b.CreatedAt = ct.time()
	b.UpdatedAt = ut.time()
	return b, nil
}

// List 列出某批次前的全部批次（按 ID 倒序）。
func (s *BatchStore) List(limit int) ([]model.Batch, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := s.db.Query(
		`SELECT id,case_ref,title,description,status,created_at,updated_at FROM batches ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Batch
	for rows.Next() {
		var b model.Batch
		var ct, ut timeText
		if err := rows.Scan(&b.ID, &b.CaseRef, &b.Title, &b.Description, &b.Status, &ct, &ut); err != nil {
			return nil, err
		}
		b.CreatedAt = ct.time()
		b.UpdatedAt = ut.time()
		out = append(out, b)
	}
	return out, rows.Err()
}

// UpdateStatus 在事务内更新批次状态（配合状态机校验使用）。
func (s *BatchStore) UpdateStatus(tx *sql.Tx, id int64, status model.BatchStatus) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := tx.Exec(`UPDATE batches SET status=?, updated_at=? WHERE id=?`, status, now, id)
	return err
}

// UpdateStatusDirect 不带事务更新状态（用于简单路径）。
func (s *BatchStore) UpdateStatusDirect(id int64, status model.BatchStatus) error {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := s.db.Exec(`UPDATE batches SET status=?, updated_at=? WHERE id=?`, status, now, id)
	return err
}

func parseTime(v string) time.Time {
	t, _ := time.Parse(time.RFC3339Nano, v)
	return t
}
