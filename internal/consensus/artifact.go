package consensus

import (
	"fmt"

	"task275-inkorder/internal/model"
)

// ArtifactJudger 判定交叉证据是否应视为扫描伪影。
// 依据：跨层一致性（唯一一份扫描层出现的交叉且与其它层矛盾时高度可疑）、
// 几何置信度（几何算法给出低置信度）、以及人工异议。
type ArtifactJudger struct {
	// MinLayerCount 少于该层数的孤立证据直接标伪影（需人工复核）。
	MinLayerCount int
}

// NewArtifactJudger 构造默认判定器。
func NewArtifactJudger() ArtifactJudger { return ArtifactJudger{MinLayerCount: 2} }

// Judge 判断一条交叉证据是否为伪影。
// layersWithCrossing：出现该交叉的扫描层数；layersTotal：批次总层数；
// geoConfidence：几何判定置信度；hasObjection：是否有人工伪影异议。
func (aj ArtifactJudger) Judge(layersWithCrossing, layersTotal int, geoConfidence float64, hasObjection bool) (bool, string) {
	if layersTotal < 2 {
		return false, "单层批次不判定伪影"
	}
	if hasObjection {
		return true, "研究者登记伪影异议"
	}
	if layersWithCrossing == 1 && geoConfidence < 0.7 {
		return true, fmt.Sprintf("仅 %d/%d 层观测且几何置信度低(%.2f)", layersWithCrossing, layersTotal, geoConfidence)
	}
	if layersWithCrossing == 1 {
		return false, "仅单层观测但置信度足够，保留待复核"
	}
	return false, "多层一致，非伪影"
}

// ScanArtifact 把整批证据做伪影预筛，返回需排除的证据 ID 列表与原因。
func ScanArtifact(crossings []model.Crossing) map[int64]string {
	// 按片段对聚合各层出现情况
	layerByPair := make(map[PairKey]map[int64]bool)
	confByPair := make(map[PairKey]float64)
	for _, c := range crossings {
		key := NewPairKey(c.FirstFragmentID, c.SecondFragmentID)
		if layerByPair[key] == nil {
			layerByPair[key] = make(map[int64]bool)
		}
		layerByPair[key][c.LayerID] = true
		if c.Confidence > confByPair[key] {
			confByPair[key] = c.Confidence
		}
	}
	out := make(map[int64]string)
	for _, c := range crossings {
		key := NewPairKey(c.FirstFragmentID, c.SecondFragmentID)
		n := len(layerByPair[key])
		if n == 1 && c.Confidence < 0.7 {
			out[c.ID] = fmt.Sprintf("仅单层观测且置信度 %.2f", c.Confidence)
		}
	}
	return out
}

var _ = model.ObsCrossing
