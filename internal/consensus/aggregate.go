package consensus

import (
	"sort"

	"task275-inkorder/internal/model"
)

// PairKey 标识一对片段（归一化：first < second）。
type PairKey struct {
	A int64
	B int64
}

// NewPairKey 归一化片段对（按 ID 排序，保证跨层可比）。
func NewPairKey(x, y int64) PairKey {
	if x < y {
		return PairKey{A: x, B: y}
	}
	return PairKey{A: y, B: x}
}

// LayerVote 是单个扫描层对某片段对先后关系的投票。
type LayerVote struct {
	LayerID       int64
	FirstBefore   bool // true：层证据认为 A 先于 B
	Confidence    float64
	Evidence      string
}

// Aggregate 聚合多个扫描层对同一片段对（A,B）的投票。
// 返回：一致方向（FirstBefore）、支持度、冲突标记、不一致层数。
type AggregateResult struct {
	FirstBefore      bool
	Support          float64 // 加权支持占比 0~1
	Conflict         bool    // 支持与反对势均力敌（无法判定）
	ConsistentLayers int
	TotalLayers      int
	Reason           string
}

// Aggregate 聚合投票。
func Aggregate(key PairKey, votes []LayerVote) AggregateResult {
	res := AggregateResult{TotalLayers: len(votes)}
	if len(votes) == 0 {
		res.Reason = "无跨层证据"
		return res
	}
	agree := 0.0
	against := 0.0
	direction := votes[0].FirstBefore
	for _, v := range votes {
		if v.FirstBefore == direction {
			agree += v.Confidence
		} else {
			against += v.Confidence
		}
	}
	total := agree + against
	if total == 0 {
		res.Reason = "全部层置信度为 0"
		return res
	}
	res.FirstBefore = direction
	res.Support = agree / total
	// 冲突判定：支持率接近 0.5 视为冲突（阈值 0.4~0.6）
	if res.Support > 0.4 && res.Support < 0.6 {
		res.Conflict = true
		res.Reason = "跨层证据方向分歧，无法确定先后"
		return res
	}
	// 统计一致层数
	firstDir := 0
	for _, v := range votes {
		if v.FirstBefore == direction {
			firstDir++
		}
	}
	res.ConsistentLayers = firstDir
	if res.Support >= 0.6 {
		res.Reason = "跨层证据一致支持该方向"
	}
	return res
}

// SortVotesByConfidence 按置信度降序排票（供裁决展示）。
func SortVotesByConfidence(votes []LayerVote) {
	sort.SliceStable(votes, func(i, j int) bool {
		return votes[i].Confidence > votes[j].Confidence
	})
}

var _ = model.Crossing{}
