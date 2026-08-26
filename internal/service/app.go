package service

import (
	"database/sql"

	"task275-inkorder/internal/store"
)

// App 聚合全部 store，作为业务编排的统一入口。
type App struct {
	DB    *sql.DB
	Guard *store.WriteGuard

	Batches      *store.BatchStore
	Layers       *store.LayerStore
	Fragments    *store.FragmentStore
	Observations *store.ObservationStore
	Crossings    *store.CrossingStore
	Candidates   *store.CandidateStore
	Objections   *store.ObjectionStore
	Snapshots    *store.SnapshotStore
}

// NewApp 用已打开的数据库构造编排层。
func NewApp(db *sql.DB) *App {
	g := store.NewWriteGuard(db)
	return &App{
		DB:           db,
		Guard:        g,
		Batches:      store.NewBatchStore(db),
		Layers:       store.NewLayerStore(db),
		Fragments:    store.NewFragmentStore(db),
		Observations: store.NewObservationStore(db),
		Crossings:    store.NewCrossingStore(db),
		Candidates:   store.NewCandidateStore(g),
		Objections:   store.NewObjectionStore(db),
		Snapshots:    store.NewSnapshotStore(g),
	}
}
