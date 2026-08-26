package store

import (
	"database/sql"
	"time"

	"task275-inkorder/internal/model"
)

// CrossingStore 提供交叉覆盖证据的持久化。
type CrossingStore struct {
	db *sql.DB
}

func NewCrossingStore(db *sql.DB) *CrossingStore { return &CrossingStore{db: db} }

// Create 登记一条交叉覆盖证据。
func (s *CrossingStore) Create(c *model.Crossing) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`INSERT INTO crossings(batch_id,layer_id,first_fragment_id,second_fragment_id,x,y,confidence,evidence,is_artifact,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		c.BatchID, c.LayerID, c.FirstFragmentID, c.SecondFragmentID,
		c.X, c.Y, c.Confidence, c.Evidence, boolInt(c.IsArtifact), now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	c.ID = id
	c.CreatedAt = parseTime(now)
	return id, nil
}

// Get 按 ID 取交叉证据。
func (s *CrossingStore) Get(id int64) (*model.Crossing, error) {
	var ct timeText
	row := s.db.QueryRow(
		`SELECT id,batch_id,layer_id,first_fragment_id,second_fragment_id,x,y,confidence,evidence,is_artifact,created_at
		 FROM crossings WHERE id=?`, id)
	c := &model.Crossing{}
	err := row.Scan(&c.ID, &c.BatchID, &c.LayerID, &c.FirstFragmentID, &c.SecondFragmentID,
		&c.X, &c.Y, &c.Confidence, &c.Evidence, &c.IsArtifact, &ct)
	if err == sql.ErrNoRows {
		return nil, model.NewError(model.ErrCodeNotFound, "crossing %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	c.CreatedAt = ct.time()
	return c, nil
}

// ListByBatch 列出批次全部交叉证据（含伪影标记）。
func (s *CrossingStore) ListByBatch(batchID int64) ([]model.Crossing, error) {
	rows, err := s.db.Query(
		`SELECT id,batch_id,layer_id,first_fragment_id,second_fragment_id,x,y,confidence,evidence,is_artifact,created_at
		 FROM crossings WHERE batch_id=? ORDER BY id`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Crossing
	for rows.Next() {
		var c model.Crossing
		var ct timeText
		if err := rows.Scan(&c.ID, &c.BatchID, &c.LayerID, &c.FirstFragmentID, &c.SecondFragmentID,
			&c.X, &c.Y, &c.Confidence, &c.Evidence, &c.IsArtifact, &ct); err != nil {
			return nil, err
		}
		c.CreatedAt = ct.time()
		out = append(out, c)
	}
	return out, rows.Err()
}

// MarkArtifact 标记交叉证据为扫描伪影（不参与笔顺重建）。
func (s *CrossingStore) MarkArtifact(id int64, artifact bool) error {
	res, err := s.db.Exec(`UPDATE crossings SET is_artifact=? WHERE id=?`, boolInt(artifact), id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.NewError(model.ErrCodeNotFound, "crossing %d not found", id)
	}
	return nil
}
