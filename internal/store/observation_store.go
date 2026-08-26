package store

import (
	"database/sql"
	"time"

	"task275-inkorder/internal/model"
)

// ObservationStore 提供观测点（交叉/停顿/方向）的持久化。
type ObservationStore struct {
	db *sql.DB
}

func NewObservationStore(db *sql.DB) *ObservationStore { return &ObservationStore{db: db} }

// Create 登记观测点。
func (s *ObservationStore) Create(o *model.Observation) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`INSERT INTO observations(batch_id,fragment_id,kind,x,y,note,created_at)
		 VALUES(?,?,?,?,?,?,?)`,
		o.BatchID, o.FragmentID, o.Kind, o.X, o.Y, o.Note, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	o.ID = id
	o.CreatedAt = parseTime(now)
	return id, nil
}

// ListByFragment 列出片段的观测点。
func (s *ObservationStore) ListByFragment(fragmentID int64) ([]model.Observation, error) {
	rows, err := s.db.Query(
		`SELECT id,batch_id,fragment_id,kind,x,y,note,created_at FROM observations WHERE fragment_id=? ORDER BY id`,
		fragmentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Observation
	for rows.Next() {
		var o model.Observation
		var ct timeText
		if err := rows.Scan(&o.ID, &o.BatchID, &o.FragmentID, &o.Kind, &o.X, &o.Y, &o.Note, &ct); err != nil {
			return nil, err
		}
		o.CreatedAt = ct.time()
		out = append(out, o)
	}
	return out, rows.Err()
}

// ListByBatch 列出批次的观测点（按片段排序）。
func (s *ObservationStore) ListByBatch(batchID int64) ([]model.Observation, error) {
	rows, err := s.db.Query(
		`SELECT id,batch_id,fragment_id,kind,x,y,note,created_at FROM observations WHERE batch_id=? ORDER BY fragment_id,id`,
		batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Observation
	for rows.Next() {
		var o model.Observation
		var ct timeText
		if err := rows.Scan(&o.ID, &o.BatchID, &o.FragmentID, &o.Kind, &o.X, &o.Y, &o.Note, &ct); err != nil {
			return nil, err
		}
		o.CreatedAt = ct.time()
		out = append(out, o)
	}
	return out, rows.Err()
}
