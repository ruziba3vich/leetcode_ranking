package tests

import (
	"context"
	"log"
	"testing"

	_ "github.com/lib/pq"
	"github.com/ruziba3vich/leetcode_ranking/db/users_storage"
	"github.com/ruziba3vich/leetcode_ranking/internal/pkg/config"
	"github.com/ruziba3vich/leetcode_ranking/internal/pkg/helper"
	"github.com/ruziba3vich/leetcode_ranking/internal/service"
	logger "github.com/ruziba3vich/prodonik_lgger"
)

type Factory struct {
	service service.UserService
}

var (
	factory Factory
)

func GetUserService() service.UserService {
	if factory.service == nil {
		cfg := config.Load()
		leetcodeClient := service.NewLeetCodeClient(cfg)
		lgg, err := logger.NewLogger("app.log")
		if err != nil {
			log.Fatal(err)
		}
		db := helper.NewDB(cfg)
		storage := users_storage.New(db)
		factory.service = service.NewUserService(storage, leetcodeClient, lgg)
	}

	return factory.service
}

// --- fakes/mocks ---

// mock logger that satisfies your logger.Logger (only methods you used)
type mockLogger struct{}

func (m mockLogger) Infof(format string, args ...interface{})  {}
func (m mockLogger) Errorf(format string, args ...interface{}) {}

type mockFetcher struct {
	resp *service.ResponseUser
	err  error
	last string
}

func (m *mockFetcher) FetchUser(username string) (*service.ResponseUser, error) {
	m.last = username
	return m.resp, m.err
}

// helper to make a successful ResponseUser with AC "All"
func makeOKResp(username string, solved, subs int, countryCode, countryName, realName, slug, avatar, typename string) *service.ResponseUser {
	return &service.ResponseUser{
		Data: service.DataUser{
			MatchedUser: &service.MatchedUser{
				SubmitStats: service.SubmitStats{
					ACSubmissionNum: []service.ACStat{
						{Difficulty: "Easy", Count: 1, Submissions: 2},
						{Difficulty: "All", Count: solved, Submissions: subs},
					},
				},
				Profile: service.ProfileFull{
					UserSlug:    slug,
					UserAvatar:  avatar,
					CountryCode: countryCode,
					CountryName: countryName,
					RealName:    realName,
					Typename:    typename,
				},
			},
		},
	}
}

// --- tests ---

func TestFetchLeetCodeUser_Success(t *testing.T) {
	// origFactory := newLCFetcher
	// newLCFetcher = func(debug bool, delay time.Duration) lcFetcher { return mf }
	// t.Cleanup(func() { newLCFetcher = origFactory })

	svc := GetUserService()

	// act
	got, err := svc.FetchLeetCodeUser(context.Background(), "neal_wu")
	if err != nil {
		t.Fatalf("FetchLeetCodeUser error: %v", err)
	}

	// pp.Println(got)

	// assert
	if got.User.Username != "neal_wu" {
		t.Errorf("username = %q, want %q", got.User.Username, "neal_wu")
	}
	p := got.User.Profile
	if p.UserSlug != "neal_wu" {
		t.Errorf("userSlug = %q, want %q", p.UserSlug, "neal_wu")
	}
	if p.UserAvatar != "https://assets.leetcode.com/users/neal_wu/avatar_1737814509.png" {
		t.Errorf("avatar mismatch")
	}
	if p.CountryCode != "US" || p.CountryName != "United States" {
		t.Errorf("country = %s/%s, want US/United States", p.CountryCode, p.CountryName)
	}
	if p.RealName != "Neal Wu" {
		t.Errorf("realName = %q, want Neal Wu", p.RealName)
	}
	if p.Typename != "UserProfileNode" {
		t.Errorf("typename = %q, want UserProfileNode", p.Typename)
	}
	if p.TotalProblemsSolved != 253 {
		t.Errorf("solved = %d, want 253", p.TotalProblemsSolved)
	}
	if p.TotalSubmissions != 440 {
		t.Errorf("accepted submissions = %d, want 440", p.TotalSubmissions)
	}
}

func TestFetchLeetCodeUser_EmptyUsername(t *testing.T) {
	svc := GetUserService()

	_, err := svc.FetchLeetCodeUser(context.Background(), "  ")
	if err == nil {
		t.Fatal("expected error for empty username, got nil")
	}
}

func TestFetchLeetCodeUser_NoMatchedUser(t *testing.T) {
	// mf := &mockFetcher{resp: &service.ResponseUser{Data: service.DataUser{MatchedUser: nil}}, err: nil}
	// origFactory := newLCFetcher
	// newLCFetcher = func(debug bool, delay time.Duration) lcFetcher { return mf }
	// t.Cleanup(func() { newLCFetcher = origFactory })

	svc := GetUserService()

	_, err := svc.FetchLeetCodeUser(context.Background(), "some_not_available_username")
	// pp.Println(resp)
	if err == nil {
		t.Fatal("expected error when matchedUser is nil, got nil")
	}
}

// func TestFetchLeetCodeUser_NoAllStat(t *testing.T) {
// 	// same as OK but remove the "All" difficulty
// 	resp := makeOKResp("x", 10, 20, "US", "United States", "X", "x", "a.png", "UserProfileNode")
// 	resp.Data.MatchedUser.SubmitStats.ACSubmissionNum = []service.ACStat{
// 		{Difficulty: "Easy", Count: 1, Submissions: 2},
// 	}

// 	// mf := &mockFetcher{resp: resp}
// 	// origFactory := newLCFetcher
// 	// newLCFetcher = func(debug bool, delay time.Duration) lcFetcher { return mf }
// 	// t.Cleanup(func() { newLCFetcher = origFactory })

// 	svc := GetUserService()

// 	_, err := svc.FetchLeetCodeUser(context.Background(), "x")
// 	if err == nil {
// 		t.Fatal("expected error when AC 'All' stat is missing, got nil")
// 	}
// }

func TestFetchLeetCodeUser_RemoteError(t *testing.T) {
	// mf := &mockFetcher{resp: nil, err: errors.New("boom")}
	// origFactory := newLCFetcher
	// newLCFetcher = func(debug bool, delay time.Duration) lcFetcher { return mf }
	// t.Cleanup(func() { newLCFetcher = origFactory })

	svc := GetUserService()

	_, err := svc.FetchLeetCodeUser(context.Background(), "__any")

	// pp.Println(resp)
	if err == nil {
		t.Fatal("expected error from fetcher, got nil")
	}
}
