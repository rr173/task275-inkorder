package store

import (
	"database/sql"
	"time"

	"task275-inkorder/internal/model"
)

// ObjectionStore 提供异议的持久化。
type ObjectionStore struct {
	db *sql.DB
}

func NewObjectionStore(db *sql.DB) *ObjectionStore { return &ObjectionStore{db: db} }

// Create 登记异议。
func (s *ObjectionStore) Create(o *model.Objection) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`INSERT INTO objections(candidate_id,fragment_id,kind,reason,created_at)
		 VALUES(?,?,?,?,?)`,
		o.CandidateID, o.FragmentID, o.Kind, o.Reason, now)
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

// ListByCandidate 列出候选的异议。
func (s *ObjectionStore) ListByCandidate(candidateID int64) ([]model.Objection, error) {
	rows, err := s.db.Query(
		`SELECT id,candidate_id,fragment_id,kind,reason,created_at FROM objections WHERE candidate_id=? ORDER BY id`,
		candidateID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Objection
	for rows.Next() {
		var o model.Objection
		var ct timeText
		if err := rows.Scan(&o.ID, &o.CandidateID, &o.FragmentID, &o.Kind, &o.Reason, &ct); err != nil {
			return nil, err
		}
		o.CreatedAt = ct.time()
		out = append(out, o)
	}
	return out, rows.Err()
}

// CountByCandidate 统计候选的异议数（供裁决参考）。
func (s *ObjectionStore) CountByCandidate(candidateID int64) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM objections WHERE candidate_id=?`, candidateID).Scan(&n)
	return n, err
}
