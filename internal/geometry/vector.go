package geometry

import "math"

// Point 是二维坐标点。
type Point struct {
	X float64
	Y float64
}

// Segment 是二维线段。
type Segment struct {
	A Point
	B Point
}

// Distance 计算两点欧氏距离。
func Distance(p, q Point) float64 {
	dx, dy := p.X-q.X, p.Y-q.Y
	return math.Hypot(dx, dy)
}

// Length 返回线段长度。
func (s Segment) Length() float64 { return Distance(s.A, s.B) }

// Project 返回点 p 在线段 s 上的投影参数 t（0~1）与投影点。
func (s Segment) Project(p Point) (float64, Point) {
	dx, dy := s.B.X-s.A.X, s.B.Y-s.A.Y
	len2 := dx*dx + dy*dy
	if len2 == 0 {
		return 0, s.A
	}
	t := ((p.X-s.A.X)*dx + (p.Y-s.A.Y)*dy) / len2
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	return t, Point{X: s.A.X + t*dx, Y: s.A.Y + t*dy}
}

// DistanceToSegment 返回点到线段的距离。
func (s Segment) DistanceToSegment(p Point) float64 {
	_, proj := s.Project(p)
	return Distance(p, proj)
}

// CrossProduct 计算向量 ab 与 ac 的叉积（z 分量），用于判断方向与相交。
func CrossProduct(a, b, c Point) float64 {
	return (b.X-a.X)*(c.Y-a.Y) - (b.Y-a.Y)*(c.X-a.X)
}

// Intersection 计算两条线段交点；不相交返回 (Point, false)。
// 采用标准 CCW 求交算法。
func Intersection(s1, s2 Segment) (Point, bool) {
	o1 := CrossProduct(s1.A, s1.B, s2.A)
	o2 := CrossProduct(s1.A, s1.B, s2.B)
	o3 := CrossProduct(s2.A, s2.B, s1.A)
	o4 := CrossProduct(s2.A, s2.B, s1.B)

	// 一般相交（严格跨越）
	if o1*o2 < 0 && o3*o4 < 0 {
		d := (s2.B.Y-s2.A.Y)*(s1.B.X-s1.A.X) - (s2.B.X-s2.A.X)*(s1.B.Y-s1.A.Y)
		if d == 0 {
			return Point{}, false
		}
		ua := ((s2.B.X-s2.A.X)*(s1.A.Y-s2.A.Y) - (s2.B.Y-s2.A.Y)*(s1.A.X-s2.A.X)) / d
		x := s1.A.X + ua*(s1.B.X-s1.A.X)
		y := s1.A.Y + ua*(s1.B.Y-s1.A.Y)
		return Point{X: x, Y: y}, true
	}
	// 端点接触（共线或端点重合）视为不相交（墨迹交叉需严格跨越）
	return Point{}, false
}

// Midpoint 返回线段中点。
func (s Segment) Midpoint() Point {
	return Point{X: (s.A.X + s.B.X) / 2, Y: (s.A.Y + s.B.Y) / 2}
}

// DirectionAngle 返回线段方向角（弧度，0~2π）。
func (s Segment) DirectionAngle() float64 {
	return math.Atan2(s.B.Y-s.A.Y, s.B.X-s.A.X)
}

// AngleDiff 返回两角度最小差值（弧度）。
func AngleDiff(a, b float64) float64 {
	d := math.Mod(a-b+math.Pi, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	return d - math.Pi
}
