package model

import "time"

// FragmentStatus 是笔迹片段的状态机取值。
//
//	raw → calibrated → artifact（伪影）→ excluded（排除）
//	  └──────────────────────────┘
type FragmentStatus string

const (
	FragmentRaw        FragmentStatus = "raw"
	FragmentCalibrated FragmentStatus = "calibrated"
	FragmentArtifact   FragmentStatus = "artifact"
	FragmentExcluded   FragmentStatus = "excluded"
)

// Fragment 是一条笔迹笔画片段（扫描件上的一段墨迹）。
// 坐标使用所属扫描层的原始像素坐标，校正后写入校正坐标。
type Fragment struct {
	ID         int64          `json:"id"`
	BatchID    int64          `json:"batch_id"`
	LayerID    int64          `json:"layer_id"`
	Label      string         `json:"label"`       // 笔画编号，如 "A1"
	Status     FragmentStatus `json:"status"`
	StartX     float64        `json:"start_x"`     // 起点原始坐标
	StartY     float64        `json:"start_y"`
	EndX       float64        `json:"end_x"`       // 终点原始坐标
	EndY       float64        `json:"end_y"`
	Pressure   float64        `json:"pressure"`    // 平均笔压（0~1），停顿处偏低
	CalibStartX float64       `json:"calib_start_x"` // 校正后起点
	CalibStartY float64       `json:"calib_start_y"`
	CalibEndX   float64       `json:"calib_end_x"`   // 校正后终点
	CalibEndY   float64       `json:"calib_end_y"`
	CreatedAt  time.Time      `json:"created_at"`
}

// FragmentTransitions 定义片段状态流转表。
var FragmentTransitions = map[FragmentStatus][]FragmentStatus{
	FragmentRaw:        {FragmentCalibrated, FragmentArtifact, FragmentExcluded},
	FragmentCalibrated: {FragmentArtifact, FragmentExcluded},
	FragmentArtifact:   {FragmentExcluded},
	FragmentExcluded:   {},
}

// CanTransitionFragment 判断片段状态流转是否合法。
func CanTransitionFragment(from, to FragmentStatus) bool {
	for _, next := range FragmentTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// Active 判断片段是否参与笔顺重建（未排除、未标记伪影）。
func (f *Fragment) Active() bool {
	return f.Status != FragmentExcluded && f.Status != FragmentArtifact
}
