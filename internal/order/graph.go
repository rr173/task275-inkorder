package order

import (
	"sort"
)

// Edge 是有向边：Before 先于 After。
type Edge struct {
	Before int64
	After  int64
	Weight float64
	Source string
}

// DiGraph 是片段偏序图，节点为片段 ID。
type DiGraph struct {
	// out 邻接表：节点 → 后继集合（按边权重）
	adj    map[int64][]Edge
	weight map[[2]int64]float64 // 去重边取最大权重
}

// NewDiGraph 构造空图。
func NewDiGraph() *DiGraph {
	return &DiGraph{
		adj:    make(map[int64][]Edge),
		weight: make(map[[2]int64]float64),
	}
}

// Nodes 返回全部节点。
func (g *DiGraph) Nodes() []int64 {
	out := make([]int64, 0, len(g.adj))
	for n := range g.adj {
		out = append(out, n)
	}
	return out
}

// AddEdge 添加边（同对节点重复添加取最大权重）。
func (g *DiGraph) AddEdge(e Edge) {
	key := [2]int64{e.Before, e.After}
	if old, ok := g.weight[key]; ok && old >= e.Weight {
		return
	}
	g.weight[key] = e.Weight
	// 更新邻接表：移除旧边再追加
	list := g.adj[e.Before]
	filtered := list[:0]
	for _, it := range list {
		if it.After != e.After {
			filtered = append(filtered, it)
		}
	}
	g.adj[e.Before] = append(filtered, e)
	// 确保两个节点都在图里（孤立节点也保留）
	if _, ok := g.adj[e.After]; !ok {
		g.adj[e.After] = nil
	}
}

// EdgeCount 返回边数。
func (g *DiGraph) EdgeCount() int { return len(g.weight) }

// Cycle 检测图是否有环；返回环上的一个节点序列（非空表示有环）。
func (g *DiGraph) Cycle() []int64 {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[int64]int)
	var stack []int64
	var dfs func(n int64) bool
	dfs = func(n int64) bool {
		color[n] = gray
		stack = append(stack, n)
		for _, e := range g.adj[n] {
			switch color[e.After] {
			case gray:
				// 找到回边，截取环
				idx := 0
				for i, v := range stack {
					if v == e.After {
						idx = i
						break
					}
				}
				cyc := append([]int64{}, stack[idx:]...)
				cyc = append(cyc, e.After)
				return true
			case white:
				if dfs(e.After) {
					return true
				}
			}
		}
		stack = stack[:len(stack)-1]
		color[n] = black
		return false
	}
	nodes := g.Nodes()
	sort.Slice(nodes, func(i, j int) bool { return nodes[i] < nodes[j] })
	for _, n := range nodes {
		if color[n] == white {
			stack = stack[:0]
			if dfs(n) {
				return stack
			}
		}
	}
	return nil
}

// TopoOrder 返回拓扑序（无环时）。返回的序列为书写先后顺序。
// 使用加权入度选择：优先出度大、权重高的边。
func (g *DiGraph) TopoOrder() ([]int64, bool) {
	if cyc := g.Cycle(); len(cyc) > 0 {
		return nil, false
	}
	indeg := make(map[int64]int)
	for n := range g.adj {
		indeg[n] = 0
	}
	for k := range g.weight {
		indeg[k[1]]++
	}
	// 用简单选择：每次取入度 0 且 id 最小的节点（确定性）
	done := make(map[int64]bool)
	var order []int64
	for len(order) < len(g.adj) {
		var cand int64 = -1
		for n := range g.adj {
			if done[n] {
				continue
			}
			if indeg[n] == 0 && (cand == -1 || n < cand) {
				cand = n
			}
		}
		if cand == -1 {
			return nil, false // 不应发生（无环必有入度0）
		}
		done[cand] = true
		order = append(order, cand)
		for _, e := range g.adj[cand] {
			indeg[e.After]--
		}
	}
	return order, true
}

// Edges 返回全部边（按 Before 排序）。
func (g *DiGraph) Edges() []Edge {
	keys := make([][2]int64, 0, len(g.weight))
	for k := range g.weight {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i][0] != keys[j][0] {
			return keys[i][0] < keys[j][0]
		}
		return keys[i][1] < keys[j][1]
	})
	out := make([]Edge, 0, len(keys))
	for _, k := range keys {
		out = append(out, Edge{Before: k[0], After: k[1], Weight: g.weight[k]})
	}
	return out
}
