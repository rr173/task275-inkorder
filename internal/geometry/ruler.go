package geometry

import "task275-inkorder/internal/model"

// RulerEstimator 从基准层与本层至少一对同名点估计标尺参数（相似变换）。
type RulerEstimator struct{}

// EstimateRuler 由两对对应点（基准坐标, 本层坐标）求解 scale/offset。
// 返回的 Ruler 满足：layer = base*scale + offset。
// 至少需要 1 对点（此时 scale 取 1，仅估计偏移）；2 对点可解 scale。
func EstimateRuler(basePts, layerPts []Point) model.Ruler {
	r := model.Ruler{Scale: 1, OffsetX: 0, OffsetY: 0}
	if len(basePts) == 0 || len(basePts) != len(layerPts) {
		return r
	}
	// 用首对点估计偏移（scale 先取 1）
	r.OffsetX = layerPts[0].X - basePts[0].X
	r.OffsetY = layerPts[0].Y - basePts[0].Y
	if len(basePts) >= 2 {
		// 用两对点距离比估计 scale（抗单个点噪声）
		dBase := Distance(basePts[0], basePts[1])
		dLayer := Distance(layerPts[0], layerPts[1])
		if dBase > 1e-9 && dLayer > 1e-9 {
			s := dLayer / dBase
			if s > 0.1 && s < 10 { // 合理缩放区间，防病态输入
				r.Scale = s
				// 用第二对点精修偏移
				r.OffsetX = layerPts[1].X - basePts[1].X*r.Scale
				r.OffsetY = layerPts[1].Y - basePts[1].Y*r.Scale
			}
		}
	}
	return r
}

// Residual 返回用标尺把基准点变换到本层后与实测点的平均残差。
func Residual(r model.Ruler, basePts, layerPts []Point) float64 {
	if len(basePts) == 0 {
		return 0
	}
	sum := 0.0
	n := 0
	for i := range basePts {
		lx, ly := r.ApplyBaseToLayer(basePts[i].X, basePts[i].Y)
		sum += Distance(Point{X: lx, Y: ly}, layerPts[i])
		n++
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// AlignFragment 用标尺把片段从本层坐标变换到基准层坐标。
func AlignFragment(r model.Ruler, sx, sy, ex, ey float64) (bsx, bsy, bex, bey float64) {
	bsx, bsy = r.ApplyLayerToBase(sx, sy)
	bex, bey = r.ApplyLayerToBase(ex, ey)
	return
}
