package service

import (
	"testing"

	"task275-inkorder/internal/store"
)

func TestCalibrateThenListShowsCalibratedCoords(t *testing.T) {
	db, err := store.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	app := NewApp(db)
	b, err := NewBatchService(app).Create("C", "t", "")
	if err != nil {
		t.Fatal(err)
	}
	frag := NewFragmentService(app)
	l, err := frag.AddLayer(b.ID, "L1", "s.tif", 1000, 800, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := frag.AddFragment(b.ID, l.ID, "A", 100, 80, 300, 80, 0.4); err != nil {
		t.Fatal(err)
	}
	if n, err := frag.CalibrateBatch(b.ID); err != nil || n != 1 {
		t.Fatalf("calibrate n=%d err=%v", n, err)
	}
	fs, err := app.Fragments.ListByBatch(b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 {
		t.Fatalf("len=%d", len(fs))
	}
	if fs[0].CalibStartX != 100 || fs[0].CalibEndX != 300 {
		t.Fatalf("stale calib coords: start=%.1f end=%.1f", fs[0].CalibStartX, fs[0].CalibEndX)
	}
}
