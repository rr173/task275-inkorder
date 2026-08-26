package model

import "time"

// CandidateStatus 是笔顺候选的状态机取值。
//
//	generated → consistent / conflict → confirmed / rejected
//	                └─────────────┘
//	            一致或冲突由研究者复核后确认/否决
type CandidateStatus string

const (
	CandGenerated  CandidateStatus = "generated"
	CandConsistent CandidateStatus = "consistent"
	CandConflict   CandidateStatus = "conflict"
	CandConfirmed  CandidateStatus = "confirmed"
	CandRejected   CandidateStatus = "rejected"
)

// CandidateTransitions 定义候选状态流转表。
var CandidateTransitions = map[CandidateStatus][]CandidateStatus{
	CandGenerated:  {CandConsistent, CandConflict},
	CandConsistent: {CandConfirmed, CandRejected},
	CandConflict:   {CandConfirmed, CandRejected},
	CandConfirmed:  {},
	CandRejected:   {},
}

// CanTransitionCandidate 判断候选状态流转是否合法。
func CanTransitionCandidate(from, to CandidateStatus) bool {
	for _, next := range CandidateTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// OrderCandidate 是一个笔顺候选：对一批片段的有序排列。
// Version 用于乐观并发校验（裁决时比对版本号）。
type OrderCandidate struct {
	ID             int64           `json:"id"`
	BatchID        int64           `json:"batch_id"`
	Version        int             `json:"version"`
	Status         CandidateStatus `json:"status"`
	Score          float64         `json:"score"` // 证据加权支持度 0~1
	ConflictReason string          `json:"conflict_reason,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

// EdgeSource 是候选偏序边的证据来源。
type EdgeSource string

const (
	EdgeFromCrossing EdgeSource = "crossing"
	EdgeFromPause    EdgeSource = "pause"
	EdgeFromDirection EdgeSource = "direction"
	EdgeManual       EdgeSource = "manual"
)

// CandidateEdge 是候选内部的一条偏序边：Before 先于 After 书写。
type CandidateEdge struct {
	ID              int64      `json:"id"`
	CandidateID     int64      `json:"candidate_id"`
	BeforeFragmentID int64     `json:"before_fragment_id"`
	AfterFragmentID  int64     `json:"after_fragment_id"`
	Source          EdgeSource `json:"source"`
	Weight          float64    `json:"weight"` // 该边的证据权重
}
