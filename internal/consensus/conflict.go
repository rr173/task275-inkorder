package consensus

import (
	"fmt"
	"sort"

	"task275-inkorder/internal/model"
)

// ConflictPair 是一对互相矛盾的证据（同一片段对、方向相反）。
type ConflictPair struct {
	FirstFragmentID  int64
	SecondFragmentID int64
	ForwardIDs       []int64 // 支持 A 先于 B 的证据 ID
	ReverseIDs       []int64 // 支持 B 先于 A 的证据 ID
	Resolution       string  // 裁决结论（人工填写）
}

// ConflictDetector 检测整批交叉证据中的方向冲突。
type ConflictDetector struct{}

// Detect 按片段对聚合证据，找出方向矛盾的证据组。
func (ConflictDetector) Detect(crossings []model.Crossing) []ConflictPair {
	// 方向归一化：以 (minID,maxID) 为键，记录正向/反向证据
	type rec struct {
		forward []int64
		reverse []int64
	}
	groups := make(map[PairKey]*rec)
	for _, c := range crossings {
		if c.IsArtifact {
			continue
		}
		key := NewPairKey(c.FirstFragmentID, c.SecondFragmentID)
		r := groups[key]
		if r == nil {
			r = &rec{}
			groups[key] = r
		}
		if c.FirstFragmentID == key.A {
			r.forward = append(r.forward, c.ID)
		} else {
			r.reverse = append(r.reverse, c.ID)
		}
	}

	keys := make([]PairKey, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].A != keys[j].A {
			return keys[i].A < keys[j].A
		}
		return keys[i].B < keys[j].B
	})

	var out []ConflictPair
	for _, k := range keys {
		r := groups[k]
		if len(r.forward) > 0 && len(r.reverse) > 0 {
			out = append(out, ConflictPair{
				FirstFragmentID:  k.A,
				SecondFragmentID: k.B,
				ForwardIDs:       r.forward,
				ReverseIDs:       r.reverse,
				Resolution:       "",
			})
		}
	}
	return out
}

// Describe 生成冲突描述文本。
func (p *ConflictPair) Describe() string {
	return fmt.Sprintf("片段 %d 与 %d 的交叉证据方向矛盾（%d 条支持 A 先写，%d 条支持 B 先写）",
		p.FirstFragmentID, p.SecondFragmentID, len(p.ForwardIDs), len(p.ReverseIDs))
}

var _ = model.ErrConflict
