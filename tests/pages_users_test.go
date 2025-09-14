package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/ruziba3vich/leetcode_ranking/internal/service"
)

func TestFetchRankingPage_CompareGolden(t *testing.T) {
	// ctx := context.Background()
	srv := GetUserService()

	// act: fetch first page
	resp, err := srv.FetchRankingPage(1)
	if err != nil {
		t.Fatalf("FetchRankingPage(1) failed: %v", err)
	}

	got := resp.Data.GlobalRanking.RankingNodes
	if len(got) == 0 {
		t.Fatal("FetchRankingPage returned no users")
	}

	// load golden file
	path := filepath.Join("./", "fetched_first_page_users.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file: %v", err)
	}

	var wrap struct {
		Data struct {
			GlobalRanking struct {
				RankingNodes []service.RankingNode `json:"rankingNodes"`
			} `json:"globalRanking"`
		} `json:"data"`
	}

	if err := json.Unmarshal(data, &wrap); err != nil {
		t.Fatalf("unmarshal golden: %v", err)
	}
	want := wrap.Data.GlobalRanking.RankingNodes

	// compare lengths
	if len(got) != len(want) {
		t.Errorf("user count mismatch: got %d want %d", len(got), len(want))
	}

	// shallow compare first N (to avoid overwhelming diff)
	n := len(got)
	if len(want) < n {
		n = len(want)
	}
	for i := 0; i < n; i++ {
		if got[i].User.Username != want[i].User.Username {
			t.Errorf("user[%d] username mismatch: got %q want %q", i, got[i].User.Username, want[i].User.Username)
		}
		if got[i].CurrentGlobalRank != want[i].CurrentGlobalRank {
			t.Errorf("user[%d] rank mismatch: got %d want %d", i, got[i].CurrentGlobalRank, want[i].CurrentGlobalRank)
		}
	}
}
