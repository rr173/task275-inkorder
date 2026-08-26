package model

import "time"

// ObjectionKind 是异议类型。
type ObjectionKind string

const (
	ObjArtifact   ObjectionKind = "artifact"   // 证据疑为扫描伪影
	ObjGeometry   ObjectionKind = "geometry"   // 几何位置/校正存疑
	ObjDirection  ObjectionKind = "direction"  // 笔画方向判定存疑
	ObjPressure   ObjectionKind = "pressure"   // 笔压/停顿判读存疑
	ObjOther      ObjectionKind = "other"
)

// Objection 是研究者对某个笔顺候选（或其中片段）登记的异议。
type Objection struct {
	ID          int64         `json:"id"`
	CandidateID int64         `json:"candidate_id"`
	FragmentID  int64         `json:"fragment_id"`
	Kind        ObjectionKind `json:"kind"`
	Reason      string        `json:"reason"`
	CreatedAt   time.Time     `json:"created_at"`
}

// ValidObjectionKind 校验异议类型。
func ValidObjectionKind(v string) (ObjectionKind, bool) {
	switch ObjectionKind(v) {
	case ObjArtifact, ObjGeometry, ObjDirection, ObjPressure, ObjOther:
		return ObjectionKind(v), true
	}
	return "", false
}
