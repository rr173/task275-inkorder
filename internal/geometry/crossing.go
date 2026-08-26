package geometry

import (
	"math"

	"task275-inkorder/internal/model"
)

// CrossEvidence 描述一次交叉观测的自动判定结果。
type CrossEvidence struct {
	Intersect    bool    // 两笔画是否确实相交
	X, Y         float64 // 交叉点（基准层坐标）
	Confidence   float64 // 覆盖关系置信度 0~1
	Suggestion   bool    // 建议方向：true = 片段2覆盖片段1（2后写）
	IsArtifact   bool    // 是否疑似伪影（如仅擦碰、压痕极弱）
	ArtifactWhy  string  // 伪影原因说明
}

// CrossJudge 依据墨迹几何判定交叉覆盖的书写先后。
// 输入两段（基准层坐标）与各自平均笔压；笔压低者更可能被覆盖（先写）。
// 结合方向一致性：若后写笔画方向与"压过"痕迹吻合，置信度提升。
type CrossJudge struct {
	// MinConfidence 低于该值的交叉不建立先后关系。
	MinConfidence float64
}

// NewCrossJudge 构造默认判定器。
func NewCrossJudge() CrossJudge {
	return CrossJudge{MinConfidence: 0.55}
}

// Judge 判定片段1与片段2在交叉处的书写先后。
// 返回证据：Intersect=false 表示不相交。
func (cj CrossJudge) Judge(f1, f2 *model.Fragment) CrossEvidence {
	s1 := Segment{Point{f1.CalibStartX, f1.CalibStartY}, Point{f1.CalibEndX, f1.CalibEndY}}
	s2 := Segment{Point{f2.CalibStartX, f2.CalibStartY}, Point{f2.CalibEndX, f2.CalibEndY}}

	p, ok := Intersection(s1, s2)
	if !ok {
		return CrossEvidence{Intersect: false}
	}
	// 交叉点离任一端过近视为擦碰而非交叉覆盖
	if Distance(p, s1.A) < 3 || Distance(p, s1.B) < 3 ||
		Distance(p, s2.A) < 3 || Distance(p, s2.B) < 3 {
		return CrossEvidence{
			Intersect: true, X: p.X, Y: p.Y,
			Confidence: 0.3, Suggestion: false,
			IsArtifact: true, ArtifactWhy: "交叉点贴近笔画端点，疑似擦碰而非覆盖",
		}
	}

	// 覆盖判断：后写墨迹覆盖先写墨迹，通常后写笔画笔压更高、墨色更浓
	// 以笔压差作为主信号，方向夹角作为次要信号。
	pressureGap := f2.Pressure - f1.Pressure
	ang := math.Abs(AngleDiff(s1.DirectionAngle(), s2.DirectionAngle()))

	// 方向越接近垂直，覆盖关系越可靠；几乎平行则难以判定
	perpendicularity := math.Abs(math.Sin(ang))
	base := 0.5 + 0.5*perpendicularity

	// 笔压差映射：|gap| 大 → 明确，小 → 模糊
	pressureFactor := clamp01(1 - math.Abs(pressureGap)*2) // 差越小因子越高，说明难判
	_ = pressureFactor

	// 组合置信度：垂直度 × 覆盖清晰度
	conf := base * clamp01(0.6+0.8*math.Abs(pressureGap))
	if conf > 0.98 {
		conf = 0.98
	}
	if conf < 0.05 {
		conf = 0.05
	}

	sugg := pressureGap > 0 // 片段2笔压更高 → 覆盖片段1 → 片段2后写

	// 伪影判定：方向几乎平行（无法区分先后）或笔压差几乎为零
	isArtifact := false
	why := ""
	if ang < 0.15 {
		isArtifact = true
		why = "两笔画近似平行，交叉处难以确认覆盖关系"
	} else if math.Abs(pressureGap) < 0.02 {
		isArtifact = true
		why = "两笔画笔压无显著差异，覆盖先后证据不足"
	}

	return CrossEvidence{
		Intersect:   true,
		X:           p.X,
		Y:           p.Y,
		Confidence:  conf,
		Suggestion:  sugg,
		IsArtifact:  isArtifact,
		ArtifactWhy: why,
	}
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
