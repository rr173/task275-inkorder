package store

import (
	"database/sql"
	"time"

	"task275-inkorder/internal/model"
)

// FragmentStore 提供笔迹片段的持久化。
type FragmentStore struct {
	db *sql.DB
}

func NewFragmentStore(db *sql.DB) *FragmentStore { return &FragmentStore{db: db} }

// Create 导入一条片段，状态为 raw。
func (s *FragmentStore) Create(f *model.Fragment) (int64, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	res, err := s.db.Exec(
		`INSERT INTO fragments(batch_id,layer_id,label,status,start_x,start_y,end_x,end_y,pressure,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?,?)`,
		f.BatchID, f.LayerID, f.Label, model.FragmentRaw,
		f.StartX, f.StartY, f.EndX, f.EndY, f.Pressure, now)
	if err != nil {
		return 0, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}
	f.ID = id
	f.Status = model.FragmentRaw
	f.CreatedAt = parseTime(now)
	return id, nil
}

// Get 按 ID 取片段。
func (s *FragmentStore) Get(id int64) (*model.Fragment, error) {
	var ct timeText
	row := s.db.QueryRow(
		`SELECT id,batch_id,layer_id,label,status,start_x,start_y,end_x,end_y,pressure,
		        calib_start_x,calib_start_y,calib_end_x,calib_end_y,created_at
		 FROM fragments WHERE id=?`, id)
	f := &model.Fragment{}
	err := row.Scan(&f.ID, &f.BatchID, &f.LayerID, &f.Label, &f.Status,
		&f.StartX, &f.StartY, &f.EndX, &f.EndY, &f.Pressure,
		&f.CalibStartX, &f.CalibStartY, &f.CalibEndX, &f.CalibEndY, &ct)
	if err == sql.ErrNoRows {
		return nil, model.NewError(model.ErrCodeNotFound, "fragment %d not found", id)
	}
	if err != nil {
		return nil, err
	}
	f.CreatedAt = ct.time()
	return f, nil
}

// ListByBatch 列出批次的片段。
func (s *FragmentStore) ListByBatch(batchID int64) ([]model.Fragment, error) {
	rows, err := s.db.Query(
		`SELECT id,batch_id,layer_id,label,status,start_x,start_y,end_x,end_y,pressure,
		        calib_start_x,calib_start_y,calib_end_x,calib_end_y,created_at
		 FROM fragments WHERE batch_id=? ORDER BY id`, batchID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFragments(rows)
}

// ListByLayer 列出某扫描层的片段。
func (s *FragmentStore) ListByLayer(layerID int64) ([]model.Fragment, error) {
	rows, err := s.db.Query(
		`SELECT id,batch_id,layer_id,label,status,start_x,start_y,end_x,end_y,pressure,
		        calib_start_x,calib_start_y,calib_end_x,calib_end_y,created_at
		 FROM fragments WHERE layer_id=? ORDER BY id`, layerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanFragments(rows)
}

// UpdateCalibration 写入校正坐标并把状态推进到 calibrated。
func (s *FragmentStore) UpdateCalibration(id int64, sx, sy, ex, ey float64) error {
	res, err := s.db.Exec(
		`UPDATE fragments SET calib_start_x=?,calib_start_y=?,calib_end_x=?,calib_end_y=?,status=?
		 WHERE id=?`,
		sx, sy, ex, ey, model.FragmentCalibrated, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.NewError(model.ErrCodeNotFound, "fragment %d not found", id)
	}
	return nil
}

// UpdateStatus 更新片段状态（调用方负责状态机校验）。
func (s *FragmentStore) UpdateStatus(id int64, status model.FragmentStatus) error {
	res, err := s.db.Exec(`UPDATE fragments SET status=? WHERE id=?`, status, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return model.NewError(model.ErrCodeNotFound, "fragment %d not found", id)
	}
	return nil
}

// scanFragments 每次返回独立底层数组的切片。
// 不能复用包级缓冲：多个 ListBy* 结果会被调用方同时持有（如重建笔顺时
// 先取片段再取交叉），若共享底层数组，后一次查询会覆盖前一次的结果，
// 表现为片段列表被截短或串到另一批的数据。
func scanFragments(rows *sql.Rows) ([]model.Fragment, error) {
	var out []model.Fragment
	for rows.Next() {
		var f model.Fragment
		var ct timeText
		if err := rows.Scan(&f.ID, &f.BatchID, &f.LayerID, &f.Label, &f.Status,
			&f.StartX, &f.StartY, &f.EndX, &f.EndY, &f.Pressure,
			&f.CalibStartX, &f.CalibStartY, &f.CalibEndX, &f.CalibEndY, &ct); err != nil {
			return nil, err
		}
		f.CreatedAt = ct.time()
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
