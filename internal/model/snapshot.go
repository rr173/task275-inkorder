package model

import "time"

// SnapshotStatus 是鉴定快照的状态机取值。
//
//	draft → shared → frozen → superseded
type SnapshotStatus string

const (
	SnapDraft      SnapshotStatus = "draft"
	SnapShared     SnapshotStatus = "shared"
	SnapFrozen     SnapshotStatus = "frozen"
	SnapSuperseded SnapshotStatus = "superseded"
)

// SnapshotTransitions 定义快照状态流转表。
var SnapshotTransitions = map[SnapshotStatus][]SnapshotStatus{
	SnapDraft:      {SnapShared},
	SnapShared:     {SnapFrozen},
	SnapFrozen:     {SnapSuperseded},
	SnapSuperseded: {},
}

// CanTransitionSnapshot 判断快照状态流转是否合法。
func CanTransitionSnapshot(from, to SnapshotStatus) bool {
	for _, next := range SnapshotTransitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// Snapshot 是一份鉴定快照：冻结某候选的笔顺结论与扫描基准。
// 冻结后不可修改（含其引用的候选与标尺配置）。EvidenceJSON 在冻结时写入，之后只读。
type Snapshot struct {
	ID           int64          `json:"id"`
	BatchID      int64          `json:"batch_id"`
	CandidateID  int64          `json:"candidate_id"` // 冻结的笔顺候选
	Status       SnapshotStatus `json:"status"`
	RulerRef     string         `json:"ruler_ref"` // 保留的扫描基准（层名+标尺摘要）
	EvidenceJSON string         `json:"evidence_json,omitempty"`
	Note         string         `json:"note"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
}
