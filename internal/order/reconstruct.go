package order

import (
	"fmt"

	"task275-inkorder/internal/model"
)

// RebuildResult 是一次笔顺重建的输出。
type RebuildResult struct {
	Order         []int64 // 重建的书写先后顺序（片段 ID）
	Score         float64 // 证据加权支持度 0~1
	HasCycle      bool    // 证据偏序是否成环（冲突）
	Cycle         []int64 // 环上的片段序列
	ConflictReason string // 冲突描述
	Edges         []model.CandidateEdge
}

// Builder 从交叉覆盖/停顿/方向证据构建笔顺候选。
type Builder struct{}

// Build 综合三类证据构建偏序图并求解拓扑序。
// fragments 为批次的全部活跃片段；crossings 为有效交叉证据；
// observations 为观测点（pause/direction 提供额外偏序边）。
func (Builder) Build(
	fragments []model.Fragment,
	crossings []model.Crossing,
	observations []model.Observation,
) RebuildResult {

	g := NewDiGraph()

	// 片段 ID 集合
	ids := make(map[int64]bool)
	for _, f := range fragments {
		if !f.Active() {
			continue
		}
		ids[f.ID] = true
	}

	// 1. 交叉覆盖证据（主证据）
	totalWeight := 0.0
	supportWeight := 0.0
	for _, c := range crossings {
		if c.IsArtifact || !ids[c.FirstFragmentID] || !ids[c.SecondFragmentID] {
			continue
		}
		if c.FirstFragmentID == c.SecondFragmentID {
			continue
		}
		g.AddEdge(Edge{Before: c.FirstFragmentID, After: c.SecondFragmentID, Weight: c.Confidence, Source: string(model.EdgeFromCrossing)})
		totalWeight += c.Confidence
		supportWeight += c.Confidence
	}

	// 2. 停顿证据：同一片段上的 pause 若与相邻片段端点接近，暗示停顿后起笔
	//    （简化：把 pause 观测视为"该片段晚于交叉出现"的辅助信号，不产生边；
	//     真正由 service 层把 pause 映射为 direction 边。）

	// 3. 方向证据：direction 观测提示片段延伸方向，用于给孤立片段补充边
	//    （同层方向一致不产生偏序；此处保留扩展位）

	// 4. 人工边：来自异议/人工登记的 edges（service 传入）

	cycle := g.Cycle()
	res := RebuildResult{}
	if len(cycle) > 0 {
		res.HasCycle = true
		res.Cycle = cycle
		res.ConflictReason = fmt.Sprintf("交叉覆盖证据构成偏序环：%v", cycle)
		// 冲突时仍尝试给出尽量一致的排序（删除环上最弱边）
		g = g.removeWeakestCycleEdge()
	}

	order, ok := g.TopoOrder()
	if !ok {
		res.HasCycle = true
		res.ConflictReason = "偏序图存在环且无法通过删边修复"
		return res
	}
	res.Order = order
	if totalWeight > 0 {
		res.Score = supportWeight / totalWeight
	} else {
		res.Score = 1.0
	}
	// 构建候选边
	for _, e := range g.Edges() {
		res.Edges = append(res.Edges, model.CandidateEdge{
			BeforeFragmentID: e.Before,
			AfterFragmentID:  e.After,
			Source:           model.EdgeSource(e.Source),
			Weight:           e.Weight,
		})
	}
	return res
}

// removeWeakestCycleEdge 反复删除环上最弱边直到无环。
func (g *DiGraph) removeWeakestCycleEdge() *DiGraph {
	clone := NewDiGraph()
	for _, e := range g.Edges() {
		clone.AddEdge(e)
	}
	for {
		cyc := clone.Cycle()
		if len(cyc) == 0 {
			return clone
		}
		// 找环上最弱边
		weakest := Edge{Weight: 2}
		found := false
		for i := 0; i+1 < len(cyc); i++ {
			if e, ok := clone.edgeBetween(cyc[i], cyc[i+1]); ok && e.Weight < weakest.Weight {
				weakest = e
				found = true
			}
		}
		if !found {
			return clone
		}
		clone.removeEdge(weakest.Before, weakest.After)
	}
}

func (g *DiGraph) edgeBetween(a, b int64) (Edge, bool) {
	for _, e := range g.adj[a] {
		if e.After == b {
			return e, true
		}
	}
	return Edge{}, false
}

func (g *DiGraph) removeEdge(a, b int64) {
	delete(g.weight, [2]int64{a, b})
	list := g.adj[a]
	filtered := list[:0]
	for _, e := range list {
		if e.After != b {
			filtered = append(filtered, e)
		}
	}
	g.adj[a] = filtered
}
