package verify

import (
	"context"
	"path/filepath"
	"testing"
)

func TestBouncedBlocksAndPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bounced.json")
	if err := LoadBounced(path); err != nil {
		t.Fatal(err)
	}
	if n, err := AddBounced([]string{"Dead@Example.com"}); n != 1 || err != nil {
		t.Fatalf("add: n=%d err=%v", n, err)
	}
	if r := Check(context.Background(), "dead@example.com"); r.Valid || r.Code != CodeBounced {
		t.Fatalf("bounced address not blocked: %+v", r)
	}

	bounced = map[string]bool{} // simulate restart
	if err := LoadBounced(path); err != nil || !isBounced("dead@example.com") {
		t.Fatalf("not persisted: err=%v", err)
	}
}
