package service

import (
	"context"
	"testing"

	"task275-inkorder/internal/store"
)

func TestFailedTransitionDoesNotBlockLaterWrites(t *testing.T) {
	dir := t.TempDir()
	db, err := store.Open(dir + "/probe.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := NewApp(db)
	ctx := context.Background()
	b, err := NewBatchService(app).Create("C", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewBatchService(app).Rebuild(ctx, b.ID); err == nil {
		t.Fatal("rebuild without fragments should fail")
	}
	frag := NewFragmentService(app)
	l, err := frag.AddLayer(b.ID, "L1", "s.tif", 1000, 800, true)
	if err != nil {
		t.Fatalf("add layer after failed transition: %v", err)
	}
	if _, err := frag.AddFragment(b.ID, l.ID, "A", 1, 1, 2, 2, 0.4); err != nil {
		t.Fatalf("add fragment after failed transition: %v", err)
	}
}
