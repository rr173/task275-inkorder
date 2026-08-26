package store

import (
	"database/sql"
	"time"

	"task275-inkorder/internal/model"
)

// LayerStore 提供扫描层与标尺的持久化。
type LayerStore struct {
	db *sql.DB
}

func NewLayerStore(db *sql.DB) *LayerStore { return &LayerStore{db: db} }

// Create 注册扫描层。首个注册的层自动成为基准层。
func (s *LayerStore) Create(l *model.Layer) (int64, error) {
	// 判断批次是否已有层
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM layers WHERE batch_id=?`, l.BatchID).Scan(&count); err != nil {
		return 0, err
	}
	isBase := count == 0 || l.IsBase
	if isBase {
		l.Scale = 1
		l.OffsetX = 0
		l.OffsetY = 0
	} else if l.Scale == 0 {
		l.Scale = 1
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`INSERT INTO layers(batch_id,name,scan_ref,width,height,is_base,scale,offset_x,offset_y,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		l.BatchID, l.Name, l.ScanRef, l.Width, l.Height, boolInt(isBase), l.Scale, l.OffsetX, l.OffsetY, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	l.ID = id
	l.IsBase = isBase
	l.CreatedAt = parseTime(now)
	return id, nil
}

// Get 按 ID 取扫描层。
func (s *LayerStore) Get(id int64) (*model.Layer, error) {
	var ct timeText
	row := s.db.QueryRow(
		`SELECT id,batch_id,name,scan_ref,width,height,is_base,scale,offset_x,offset_y,created_at FROM layers WHERE id=?`, id)
	l := &model.Layer{}
	err := row.Scan(&l.ID, &l.BatchID, &l.Name, &l.ScanRef, &l.Width, &l.Height,
		&l.IsBase, &l.Scale, &l.OffsetX, &l.OffsetY, &ct)
	if err == sql.ErrNoRows {
		return nil, model.NewError(model.ErrCodeNotFound, "layer %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	l.CreatedAt = ct.time()
	return l, nil
}

// ListByBatch 列出批次的全部扫描层。
func (s *LayerStore) ListByBatch(batchID int64) ([]model.Layer, error) {
	rows, err := s.db.Query(
		`SELECT id,batch_id,name,scan_ref,width,height,is_base,scale,offset_x,offset_y,created_at
		 FROM layers WHERE batch_id=? ORDER BY id`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Layer
	for rows.Next() {
		var l model.Layer
		var ct timeText
		if err := rows.Scan(&l.ID, &l.BatchID, &l.Name, &l.ScanRef, &l.Width, &l.Height,
			&l.IsBase, &l.Scale, &l.OffsetX, &l.OffsetY, &ct); err != nil {
			return nil, err
		}
		l.CreatedAt = ct.time()
		out = append(out, l)
	}
	return out, rows.Err()
}

// BaseLayer 返回批次的基准层（不存在返回 nil）。
func (s *LayerStore) BaseLayer(batchID int64) (*model.Layer, error) {
	var ct timeText
	row := s.db.QueryRow(
		`SELECT id,batch_id,name,scan_ref,width,height,is_base,scale,offset_x,offset_y,created_at
		 FROM layers WHERE batch_id=? AND is_base=1 ORDER BY id LIMIT 1`, batchID)
	l := &model.Layer{}
	err := row.Scan(&l.ID, &l.BatchID, &l.Name, &l.ScanRef, &l.Width, &l.Height,
		&l.IsBase, &l.Scale, &l.OffsetX, &l.OffsetY, &ct)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	l.CreatedAt = ct.time()
	return l, nil
}

// UpdateRuler 更新非基准层的标尺（scale/offset）。
func (s *LayerStore) UpdateRuler(id int64, scale, ox, oy float64) error {
	res, err := s.db.Exec(`UPDATE layers SET scale=?, offset_x=?, offset_y=? WHERE id=? AND is_base=0`,
		scale, ox, oy, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.NewError(model.ErrCodeConflict, "layer %d is base layer or not found", id)
	}
	return nil
}

func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
