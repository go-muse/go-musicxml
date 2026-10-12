package validation

import (
	"context"
	"testing"
)

func TestTimelineFigureCachesRemainBounded(t *testing.T) {
	s := rationalSession(t, Budgets{})
	defer s.cleanup()
	values := make(map[int]int)
	for i := 0; i < 3*timelineFigureCacheEntries+1; i++ {
		if err := timelineFigureCachePut(context.Background(), s, values, i, i); err != nil {
			t.Fatal(err)
		}
		if len(values) > timelineFigureCacheEntries {
			t.Fatalf("unbounded cache: %d", len(values))
		}
		if values[i] != i {
			t.Fatal("latest entry lost")
		}
	}
}
