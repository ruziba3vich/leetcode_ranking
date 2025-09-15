package tests

import (
	"context"
	"testing"
	"time"

	"github.com/ruziba3vich/leetcode_ranking/internal/service"
)

func TestSyncUsers(t *testing.T) {
	srv := GetUserService()
	ctx, cancel := context.WithTimeout(context.TODO(), time.Minute*120)
	defer cancel()
	opts := service.SyncOptions{
		StartPage: 12,
		Pages:     30164,
	}

	err := srv.SyncLeaderboard(ctx, opts)

	if err != nil {
		t.Errorf("got an error: %s", err.Error())
	}
}
