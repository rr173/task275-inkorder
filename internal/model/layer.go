package model

import "time"

// Layer 是同一文书的一份扫描层。多份扫描件经标尺校正后统一到同一坐标基准。
type Layer struct {
	ID      int64   `json:"id"`
	BatchID int64   `json:"batch_id"`
	Name    string  `json:"name"`     // 扫描层名，如 "正面彩色扫描"
	ScanRef string  `json:"scan_ref"` // 扫描件引用
	Width   float64 `json:"width"`    // 原始像素宽度
	Height  float64 `json:"height"`   // 原始像素高度
	IsBase  bool    `json:"is_base"`  // 是否基准层（其余层向其对齐）
	Scale   float64 `json:"scale"`    // 标尺比例（基准层为 1）
	OffsetX float64 `json:"offset_x"` // 标尺偏移
	OffsetY float64 `json:"offset_y"`
	CreatedAt time.Time `json:"created_at"`
}

// Ruler 描述一个标尺校正变换：raw = scale * base + offset（从基准坐标到本层坐标）。
type Ruler struct {
	Scale   float64
	OffsetX float64
	OffsetY float64
}

// ApplyBaseToLayer 把基准层坐标变换到本扫描层坐标。
func (r *Ruler) ApplyBaseToLayer(bx, by float64) (float64, float64) {
	return bx*r.Scale + r.OffsetX, by*r.Scale + r.OffsetY
}

// ApplyLayerToBase 把本扫描层坐标变换回基准层坐标（逆向）。
func (r *Ruler) ApplyLayerToBase(lx, ly float64) (float64, float64) {
	if r.Scale == 0 {
		return 0, 0
	}
	return (lx - r.OffsetX) / r.Scale, (ly - r.OffsetY) / r.Scale
}

// IdentityRuler 返回基准层自身的恒等标尺。
func IdentityRuler() Ruler { return Ruler{Scale: 1, OffsetX: 0, OffsetY: 0} }

// FromLayer 从层实体构造标尺。
func FromLayer(l *Layer) Ruler {
	return Ruler{Scale: l.Scale, OffsetX: l.OffsetX, OffsetY: l.OffsetY}
}
