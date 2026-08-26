package model

import "time"

// BatchStatus 是鉴定批次的状态机取值。
//
//	importing → pending_rebuild → pending_review → published → sealed
//	    └──────────────┘                 └──────────────┘
//	      重建前可退回修正          复核后可直接封存或继续发布
type BatchStatus string

const (
	BatchImporting     BatchStatus = "importing"      // 导入中：可录入片段/扫描层/观测点
	BatchPendingRebuild BatchStatus = "pending_rebuild" // 待重建：片段齐备，等待生成笔顺候选
	BatchPendingReview BatchStatus = "pending_review"  // 待复核：候选已生成，研究者裁决
	BatchPublished     BatchStatus = "published"      // 已发布：鉴定快照已发布
	BatchSealed        BatchStatus = "sealed"         // 封存：不可再修改
)

// Batch 是一个鉴定批次，聚合一批笔迹片段、扫描层与笔顺候选。
type Batch struct {
	ID          int64       `json:"id"`
	CaseRef     string      `json:"case_ref"`     // 案件/文书编号
	Title       string      `json:"title"`        // 批次标题
	Description string      `json:"description"`  // 鉴定说明
	Status      BatchStatus `json:"status"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Transitions 定义合法状态流转表。
var Transitions = map[BatchStatus][]BatchStatus{
	BatchImporting:      {BatchPendingRebuild},
	BatchPendingRebuild: {BatchImporting, BatchPendingReview},
	BatchPendingReview:  {BatchPendingRebuild, BatchPublished},
	BatchPublished:      {BatchPendingReview, BatchSealed},
	BatchSealed:         {},
}

// CanTransition 判断 from → to 是否合法。
func CanTransition(from, to BatchStatus) bool {
	for _, next := range Transitions[from] {
		if next == to {
			return true
		}
	}
	return false
}

// Terminal 判断状态是否为终态（不可再流转）。
func (s BatchStatus) Terminal() bool { return s == BatchSealed }

// ValidBatchStatus 校验字符串是否为合法状态值。
func ValidBatchStatus(v string) (BatchStatus, bool) {
	s := BatchStatus(v)
	switch s {
	case BatchImporting, BatchPendingRebuild, BatchPendingReview, BatchPublished, BatchSealed:
		return s, true
	}
	return "", false
}
