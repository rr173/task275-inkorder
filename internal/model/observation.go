package model

import "time"

// ObsKind 是观测点类型。
type ObsKind string

const (
	ObsCrossing  ObsKind = "crossing"  // 墨迹交叉覆盖点：判断书写先后
	ObsPause     ObsKind = "pause"     // 笔压停顿：起笔/顿笔痕迹
	ObsDirection ObsKind = "direction" // 笔画方向：起笔方向
)

// ValidObsKind 校验观测点类型。
func ValidObsKind(v string) (ObsKind, bool) {
	switch ObsKind(v) {
	case ObsCrossing, ObsPause, ObsDirection:
		return ObsKind(v), true
	}
	return "", false
}

// Observation 是登记在片段上的一个观测点。
type Observation struct {
	ID         int64     `json:"id"`
	BatchID    int64     `json:"batch_id"`
	FragmentID int64     `json:"fragment_id"` // 关联片段
	Kind       ObsKind   `json:"kind"`
	X          float64   `json:"x"` // 观测点坐标（基准层）
	Y          float64   `json:"y"`
	Note       string    `json:"note"` // 观测说明，如"交叉处第二笔覆盖第一笔"
	CreatedAt  time.Time `json:"created_at"`
}
