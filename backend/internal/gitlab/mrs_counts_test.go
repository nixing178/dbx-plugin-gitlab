package gitlab

import (
	"context"
	"net/http"
	"sync"
	"testing"
)

// 页签计数：一次调用读四个 state 的 X-Total，用 per_page=1 降低负载。
func TestMRCountsReadsEveryState(t *testing.T) {
	var mu sync.Mutex
	seen := map[string]int{}
	totals := map[string]string{"opened": "0", "merged": "214", "closed": "20", "": "234"}

	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		state := query.Get("state")

		mu.Lock()
		seen[state]++
		mu.Unlock()

		if query.Get("per_page") != "1" {
			t.Errorf("counts must use per_page=1, got %q", query.Get("per_page"))
		}
		if total, ok := totals[state]; ok {
			w.Header().Set("X-Total", total)
		}
		_, _ = w.Write([]byte(`[]`))
	})

	counts, err := client.MRCounts(context.Background(), "demo/project", MROptions{})
	if err != nil {
		t.Fatalf("MRCounts: %v", err)
	}
	if counts.Opened != 0 || counts.Merged != 214 || counts.Closed != 20 || counts.All != 234 {
		t.Fatalf("counts = %+v", counts)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, state := range []string{"opened", "merged", "closed", ""} {
		if seen[state] != 1 {
			t.Fatalf("state %q queried %d times: %v", state, seen[state], seen)
		}
	}
}

// 实例没开 X-Total 或某一项失败时，只把该项标成 -1，不能让整页报错。
func TestMRCountsUnknownTotalFallsBackToMinusOne(t *testing.T) {
	client := newTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") == "merged" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"message":"500 Internal Server Error"}`))
			return
		}
		_, _ = w.Write([]byte(`[]`))
	})

	counts, err := client.MRCounts(context.Background(), "", MROptions{Scope: "all"})
	if err != nil {
		t.Fatalf("MRCounts: %v", err)
	}
	if counts.Opened != -1 || counts.Merged != -1 || counts.Closed != -1 || counts.All != -1 {
		t.Fatalf("counts = %+v, want every field -1", counts)
	}
}
