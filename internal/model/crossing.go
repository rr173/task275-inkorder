package model

import "time"

// Crossing 是一条交叉覆盖证据：在交叉点 (X,Y) 处，SecondFragment 覆盖/后写于 FirstFragment。
// 即 FirstFragment 先写、SecondFragment 后写（Second 在上）。
type Crossing struct {
	ID               int64     `json:"id"`
	BatchID          int64     `json:"batch_id"`
	LayerID          int64     `json:"layer_id"`
	FirstFragmentID  int64     `json:"first_fragment_id"`  // 先写的笔画（被覆盖，在下）
	SecondFragmentID int64     `json:"second_fragment_id"` // 后写的笔画（覆盖，在上）
	X                float64   `json:"x"`                  // 交叉点坐标（基准层）
	Y                float64   `json:"y"`
	Confidence       float64   `json:"confidence"` // 覆盖关系置信度 0~1
	Evidence         string    `json:"evidence"`   // 证据描述，如"第二笔压过第一笔末端"
	IsArtifact       bool      `json:"is_artifact"` // 是否被判定为扫描伪影
	CreatedAt        time.Time `json:"created_at"`
}

// Direction 返回有序对的方向语义：First → Second 表示 First 先于 Second 书写。
func (c *Crossing) Direction() string {
	return "first_before_second"
}

// Reversed 返回反向交叉（Second 先于 First），用于一致性比对时翻转证据。
func (c *Crossing) Reversed() Crossing {
	return Crossing{
		BatchID:          c.BatchID,
		LayerID:          c.LayerID,
		FirstFragmentID:  c.SecondFragmentID,
		SecondFragmentID: c.FirstFragmentID,
		X:                c.X,
		Y:                c.Y,
		Confidence:       c.Confidence,
		Evidence:         c.Evidence,
		IsArtifact:       c.IsArtifact,
	}
}
