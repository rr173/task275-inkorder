package order

import (
	"fmt"

	"task275-inkorder/internal/model"
)

// Validator 校验候选的一致性不变量。
type Validator struct{}

// ValidateResult 是校验输出。
type ValidateResult struct {
	OK        bool
	Reasons   []string // 不满足的不变量列表
	Cycle     []int64  // 若存在环
	Score     float64
}

// Validate 校验候选是否满足全部不变量：
//  1. 候选状态必须是 generated/consistent/conflict（confirmed 后不可再改）；
//  2. 偏序图无环；
//  3. 边只指向批次内的活跃片段；
//  4. 无重复边（同一对节点只保留一条）；
//  5. 候选的每条边都有证据来源（crossing/pause/direction/manual）。
func (Validator) Validate(c *model.OrderCandidate, edges []model.CandidateEdge, fragments []model.Fragment) ValidateResult {
	res := ValidateResult{OK: true}
	if c.Status != model.CandGenerated && c.Status != model.CandConsistent && c.Status != model.CandConflict {
		res.OK = false
		res.Reasons = append(res.Reasons, fmt.Sprintf("候选状态 %s 不允许再校验", c.Status))
	}

	// 活跃片段集合
	active := make(map[int64]bool)
	for _, f := range fragments {
		if f.Active() {
			active[f.ID] = true
		}
	}

	g := NewDiGraph()
	seen := make(map[[2]int64]bool)
	for _, e := range edges {
		if e.BeforeFragmentID == e.AfterFragmentID {
			res.OK = false
			res.Reasons = append(res.Reasons, fmt.Sprintf("自环边 %d→%d", e.BeforeFragmentID, e.AfterFragmentID))
			continue
		}
		if !active[e.BeforeFragmentID] || !active[e.AfterFragmentID] {
			res.OK = false
			res.Reasons = append(res.Reasons, fmt.Sprintf("边引用非活跃片段 %d→%d", e.BeforeFragmentID, e.AfterFragmentID))
			continue
		}
		key := [2]int64{e.BeforeFragmentID, e.AfterFragmentID}
		if seen[key] {
			res.OK = false
			res.Reasons = append(res.Reasons, fmt.Sprintf("重复偏序边 %d→%d", e.BeforeFragmentID, e.AfterFragmentID))
			continue
		}
		seen[key] = true
		switch e.Source {
		case model.EdgeFromCrossing, model.EdgeFromPause, model.EdgeFromDirection, model.EdgeManual:
		default:
			res.OK = false
			res.Reasons = append(res.Reasons, fmt.Sprintf("未知边证据来源 %q", e.Source))
		}
		g.AddEdge(Edge{Before: e.BeforeFragmentID, After: e.AfterFragmentID, Weight: e.Weight, Source: string(e.Source)})
	}

	if cyc := g.Cycle(); len(cyc) > 0 {
		res.OK = false
		res.Cycle = cyc
		res.Reasons = append(res.Reasons, fmt.Sprintf("偏序图存在环：%v", cyc))
	}

	// 证据加权支持度
	total := 0.0
	support := 0.0
	for _, e := range edges {
		total += e.Weight
		support += e.Weight
	}
	if total > 0 {
		res.Score = support / total
	}
	return res
}

// Consistency 比较两个候选：返回它们顺序冲突的片段对数。
func Consistency(a, b []int64) int {
	posA := make(map[int64]int, len(a))
	for i, id := range a {
		posA[id] = i
	}
	conflicts := 0
	for i := 0; i < len(b); i++ {
		for j := i + 1; j < len(b); j++ {
			pa, okA := posA[b[i]]
			pb, okB := posA[b[j]]
			if okA && okB && pa > pb {
				conflicts++
			}
		}
	}
	return conflicts
}
