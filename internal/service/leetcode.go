package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/k0kubun/pp"
	"github.com/ruziba3vich/leetcode_ranking/db/users_storage"
	"github.com/ruziba3vich/leetcode_ranking/internal/errors_"
	"github.com/ruziba3vich/leetcode_ranking/internal/pkg/config"
)

const leetcodeURL = "https://leetcode.com/graphql"

type LeetCodeClient struct {
	httpClient *http.Client
	debug      bool
	delay      time.Duration
	headers    http.Header
}

var queryGlobalRanking = `query globalRanking($page: Int) {
	globalRanking(page: $page) {
	  totalUsers
	  totalPages
	  userPerPage
	  rankingNodes {
		ranking
		currentRating
		currentGlobalRanking
		dataRegion
		user {
		  username
		  nameColor
		  activeBadge { displayName icon __typename }
		  profile {
			userSlug
			userAvatar
			countryCode
			countryName
			realName
			__typename
		  }
		  __typename
		}
		__typename
	  }
	  __typename
	}
  }`

var queryMatchedUser = `query userProfilePublicProfile($username: String!) {
	allQuestionsCount {
	  difficulty
	  count
	}
	matchedUser(username: $username) {
	  submitStats {
		acSubmissionNum {
		  difficulty
		  count
		  submissions
		}
		totalSubmissionNum {
		  difficulty
		  count
		  submissions
		}
	  }
	  profile {
		userSlug
		userAvatar
		countryCode
		countryName
		realName
		__typename
	  }
	}
  }`

func NewLeetCodeClient(cfg *config.Config) *LeetCodeClient {
	h := make(http.Header)
	h.Set("Content-Type", "application/json")
	h.Set("Accept", "*/*")
	h.Set("Accept-Language", "en-US,en;q=0.9")
	h.Set("Accept-Encoding", "gzip, deflate, br")
	h.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	h.Set("Origin", "https://leetcode.com")
	h.Set("Referer", "https://leetcode.com/contest/globalranking/")
	h.Set("Sec-Fetch-Dest", "empty")
	h.Set("Sec-Fetch-Mode", "cors")
	h.Set("Sec-Fetch-Site", "same-origin")
	// CSRF/cookies typically not required for these two queries; add if your session needs them.

	return &LeetCodeClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		debug:      cfg.Debug,
		delay:      cfg.Delay,
		headers:    h,
	}
}

// SyncOptions controls pagination & concurrency
type SyncOptions struct {
	StartPage int           // 1-based
	Pages     int           // <=0 to fetch all pages
	Workers   int           // goroutines for per-user fetch+upsert
	Delay     time.Duration // polite delay between page requests
}

// SyncLeaderboard pulls from LeetCode and upserts into DB.
// This is your single entrypoint for the daily cron.
// SyncLeaderboard pulls from LeetCode and upserts into DB page by page.
// This is your single entrypoint for the daily cron.
func (s *userService) SyncLeaderboard(ctx context.Context, opts SyncOptions) error {
	pp.Println("------------------ starting synchronization -----------------")

	// defaults
	if opts.StartPage < 1 {
		opts.StartPage = 1
	}
	if opts.Delay <= 0 {
		opts.Delay = 800 * time.Millisecond
	}
	if opts.Workers <= 0 {
		opts.Workers = 1 // Sequential processing by default
	}

	pp.Printf("sync: starting page-by-page sync from page %d, delay=%s, workers=%d\n",
		opts.StartPage, opts.Delay, opts.Workers)

	// Get first page to determine total pages
	firstPage, err := s.FetchRankingPage(opts.StartPage)
	if err != nil {
		s.logger.Errorf("sync: failed to fetch first page %d: %v", opts.StartPage, err)
		return fmt.Errorf("fetch first page: %w", err)
	}

	totalPages := firstPage.Data.GlobalRanking.TotalPages
	pp.Printf("sync: total pages available: %d\n", totalPages)

	// Determine end page
	endPage := totalPages
	if opts.Pages > 0 {
		if calculatedEnd := opts.StartPage + opts.Pages - 1; calculatedEnd < endPage {
			endPage = calculatedEnd
		}
	}

	pp.Printf("sync: will process pages %d to %d\n", opts.StartPage, endPage)

	totalProcessedUsers := 0

	// Process each page completely before moving to the next
	for currentPage := opts.StartPage; currentPage <= endPage; currentPage++ {
		select {
		case <-ctx.Done():
			s.logger.Errorf("sync: context canceled at page %d", currentPage)
			return ctx.Err()
		default:
		}

		pp.Printf("sync: processing page %d/%d\n", currentPage, endPage)

		// Fetch current page (reuse first page data if it's the start page)
		var pageResp *ResponseGlobal
		if currentPage == opts.StartPage && firstPage != nil {
			pageResp = firstPage
		} else {
			pageResp, err = s.FetchRankingPage(currentPage)
			if err != nil {
				s.logger.Errorf("sync: failed to fetch page %d: %v", currentPage, err)
				continue // Skip this page and continue with next
			}
		}

		// Extract usernames from current page
		usernames := s.extractUsernamesFromPage(pageResp)
		go pp.Printf("sync: page %d contains %d users\n", currentPage, len(usernames))

		// Process all users from current page
		pageProcessedUsers := 0
		for _, username := range usernames {
			select {
			case <-ctx.Done():
				s.logger.Errorf("sync: context canceled while processing user %s on page %d", username, currentPage)
				return ctx.Err()
			default:
			}

			if err := s.processUser(ctx, username); err != nil {
				s.logger.Errorf("sync: failed to process user %s from page %d: %v", username, currentPage, err)
				continue
			}

			pageProcessedUsers++
			totalProcessedUsers++
			go fmt.Println("--------------------------------", currentPage, "-------------------------- curr page")

			// Polite delay between user requests
			// time.Sleep(opts.Delay)
		}

		s.logger.Infof("sync: completed page %d/%d - processed %d users (total: %d)",
			currentPage, endPage, pageProcessedUsers, totalProcessedUsers)

		// Optional: delay between pages (could be different from user delay)
		if currentPage < endPage {
			time.Sleep(opts.Delay)
		}
	}

	s.logger.Infof("sync: completed all pages. Total processed users: %d", totalProcessedUsers)
	pp.Println("------------------ synchronization completed -----------------")
	return nil
}

// extractUsernamesFromPage extracts unique usernames from a page response
func (s *userService) extractUsernamesFromPage(pageResp *ResponseGlobal) []string {
	seen := make(map[string]struct{})
	var usernames []string

	for _, node := range pageResp.Data.GlobalRanking.RankingNodes {
		username := strings.TrimSpace(node.User.Username)
		if username == "" {
			continue
		}

		if _, exists := seen[username]; !exists {
			seen[username] = struct{}{}
			usernames = append(usernames, username)
		}
	}

	// Sort usernames for consistent processing order
	sort.Strings(usernames)
	return usernames
}

// processUser handles fetching and upserting a single user
func (s *userService) processUser(ctx context.Context, username string) error {
	// Fetch user profile and stats
	resp, err := s.FetchUser(username)
	if err != nil {
		return fmt.Errorf("fetch user failed: %w", err)
	}

	if resp.Data.MatchedUser == nil {
		return fmt.Errorf("user not found or unavailable")
	}

	// Find AC stats for "All" difficulty
	var acAll *ACStat
	for i := range resp.Data.MatchedUser.SubmitStats.ACSubmissionNum {
		if resp.Data.MatchedUser.SubmitStats.ACSubmissionNum[i].Difficulty == "All" {
			acAll = &resp.Data.MatchedUser.SubmitStats.ACSubmissionNum[i]
			break
		}
	}

	if acAll == nil {
		return fmt.Errorf("missing AC 'All' statistics")
	}

	profile := resp.Data.MatchedUser.Profile

	// Upsert user into database
	user, err := s.storage.UpsertUser(ctx, users_storage.UpsertUserParams{
		Username: username,
		UserSlug: profile.UserSlug,
		UserAvatar: sql.NullString{
			String: profile.UserAvatar,
			Valid:  true,
		},
		CountryCode: sql.NullString{
			String: profile.CountryCode,
			Valid:  true,
		},
		CountryName: sql.NullString{
			String: profile.CountryName,
			Valid:  true,
		},
		RealName: sql.NullString{
			String: profile.RealName,
			Valid:  true,
		},
		Typename: sql.NullString{
			String: profile.Typename,
			Valid:  true,
		},
		TotalProblemsSolved: int32(acAll.Count),
		TotalSubmissions:    int32(acAll.Submissions),
	})

	pp.Println(user)

	if err != nil {
		return fmt.Errorf("database upsert failed: %w", err)
	}

	s.logger.Infof("sync: successfully processed user=%s solved=%d submissions=%d country=%s",
		username, acAll.Count, acAll.Submissions, profile.CountryCode)

	pp.Printf("✓ Processed user: %s\n", username)
	return nil
}

// Remove the old CollectUsernames method as it's no longer needed
// The page-by-page approach eliminates the need to collect all usernames upfront

type ResponseUser struct {
	Data   DataUser       `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

type DataUser struct {
	AllQuestionsCount []struct {
		Difficulty string `json:"difficulty"`
		Count      int    `json:"count"`
	} `json:"allQuestionsCount"`
	MatchedUser *MatchedUser `json:"matchedUser"`
}

type SubmitStats struct {
	ACSubmissionNum    []ACStat `json:"acSubmissionNum"`
	TotalSubmissionNum []ACStat `json:"totalSubmissionNum"`
}

type ProfileFull struct {
	UserSlug    string `json:"userSlug"`
	UserAvatar  string `json:"userAvatar"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"countryName"`
	RealName    string `json:"realName"`
	Typename    string `json:"__typename"`
}

type OutputUser struct {
	User struct {
		Username string `json:"username"`
		Profile  struct {
			UserSlug            string `json:"userSlug"`
			UserAvatar          string `json:"userAvatar"`
			CountryCode         string `json:"countryCode"`
			CountryName         string `json:"countryName"`
			RealName            string `json:"realName"`
			Typename            string `json:"__typename"`
			TotalProblemsSolved int    `json:"totalProblemsSolved"`
			TotalSubmissions    int    `json:"totalSubmissions"`
		} `json:"profile"`
	} `json:"user"`
}

type MatchedUser struct {
	SubmitStats SubmitStats `json:"submitStats"`
	Profile     ProfileFull `json:"profile"`
}

type GraphQLError struct {
	Message    string                 `json:"message"`
	Locations  []GraphQLErrorLocation `json:"locations,omitempty"`
	Path       []string               `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

type Badge struct {
	DisplayName string `json:"displayName"`
	Icon        string `json:"icon"`
	Typename    string `json:"__typename"`
}

type ProfileLite struct {
	UserSlug    string `json:"userSlug"`
	UserAvatar  string `json:"userAvatar"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"countryName"`
	RealName    string `json:"realName"`
	Typename    string `json:"__typename"`
}

type UserLite struct {
	Username    string      `json:"username"`
	NameColor   *string     `json:"nameColor"`
	ActiveBadge *Badge      `json:"activeBadge"`
	Profile     ProfileLite `json:"profile"`
	Typename    string      `json:"__typename"`
}

type RankingNode struct {
	Ranking           string   `json:"ranking"`
	CurrentRating     string   `json:"currentRating"`
	CurrentGlobalRank int      `json:"currentGlobalRanking"`
	DataRegion        string   `json:"dataRegion"`
	User              UserLite `json:"user"`
	Typename          string   `json:"__typename"`
}

type GlobalRanking struct {
	TotalUsers   int           `json:"totalUsers"`
	TotalPages   int           `json:"totalPages"`
	UserPerPage  int           `json:"userPerPage"`
	RankingNodes []RankingNode `json:"rankingNodes"`
	Typename     string        `json:"__typename"`
}

type DataGlobal struct {
	GlobalRanking GlobalRanking `json:"globalRanking"`
}

type ResponseGlobal struct {
	Data   DataGlobal     `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type GraphQLErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type ACStat struct {
	Difficulty  string `json:"difficulty"`
	Count       int    `json:"count"`
	Submissions int    `json:"submissions"`
}

func (s *userService) FetchUser(username string) (*ResponseUser, error) {
	var out ResponseUser
	if err := s.leetCodeClient.doGraphQL(queryMatchedUser, map[string]interface{}{"username": username}, &out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %+v", out.Errors)
	}
	return &out, nil
}

// Fetch usernames from ranking pages: start..end inclusive
func (s *userService) CollectUsernames(startPage, maxPages int) ([]string, int, error) {
	if startPage < 1 {
		startPage = 1
	}

	first, err := s.FetchRankingPage(startPage)
	if err != nil {
		return nil, 0, fmt.Errorf("fetch first page: %w", err)
	}
	totalPages := first.Data.GlobalRanking.TotalPages

	// Decide end page
	endPage := totalPages
	if maxPages > 0 {
		if e := startPage + maxPages - 1; e < endPage {
			endPage = e
		}
	}

	seen := make(map[string]struct{})
	var users []string

	// Add first page users
	for _, n := range first.Data.GlobalRanking.RankingNodes {
		u := strings.TrimSpace(n.User.Username)
		if u == "" {
			continue
		}
		if _, ok := seen[u]; !ok {
			seen[u] = struct{}{}
			users = append(users, u)
		}
	}

	// Remaining pages
	for p := startPage + 1; p <= endPage; p++ {
		fmt.Printf("Fetching rankings page %d/%d...\n", p, endPage)
		resp, err := s.FetchRankingPage(p)
		if err != nil {
			log.Printf("WARN: page %d failed: %v", p, err)
			continue
		}
		for _, n := range resp.Data.GlobalRanking.RankingNodes {
			u := strings.TrimSpace(n.User.Username)
			if u == "" {
				continue
			}
			if _, ok := seen[u]; !ok {
				seen[u] = struct{}{}
				users = append(users, u)
			}
		}
		time.Sleep(s.leetCodeClient.delay)
	}

	sort.Strings(users)
	return users, endPage, nil
}

func (s *userService) FetchRankingPage(page int) (*ResponseGlobal, error) {
	var out ResponseGlobal
	if err := s.leetCodeClient.doGraphQL(queryGlobalRanking, map[string]interface{}{"page": page}, &out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %+v", out.Errors)
	}
	return &out, nil
}

func (c *LeetCodeClient) doGraphQL(query string, variables map[string]interface{}, out interface{}) error {
	reqBody := GraphQLRequest{Query: query, Variables: variables}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest("POST", leetcodeURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("new request: %w", err)
	}
	req.Header = c.headers.Clone()

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	body, err := decompressResponse(resp)
	if err != nil {
		return fmt.Errorf("decompress: %w", err)
	}

	if c.debug {
		log.Printf("DEBUG: %s status=%d body=%s", req.URL.Path, resp.StatusCode, truncate(string(body), 800))
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("non-200: %d body: %s", resp.StatusCode, truncate(string(body), 400))
	}

	if err := json.Unmarshal(body, &out); err != nil {
		return fmt.Errorf("unmarshal: %w", err)
	}
	return nil
}

func (s *userService) FetchLeetCodeUser(ctx context.Context, username string) (OutputUser, error) {
	var out OutputUser

	username = strings.TrimSpace(username)
	if username == "" {
		return out, fmt.Errorf("username is required")
	}

	resp, err := s.FetchUser(username)
	if err != nil {
		return out, fmt.Errorf("leetcode fetch failed for %q: %w", username, err)
	}
	if resp.Data.MatchedUser == nil {
		return out, errors_.ErrUserNotAvailable
	}

	// pick AC "All"
	var acAll *ACStat
	for i := range resp.Data.MatchedUser.SubmitStats.ACSubmissionNum {
		if resp.Data.MatchedUser.SubmitStats.ACSubmissionNum[i].Difficulty == "All" {
			acAll = &resp.Data.MatchedUser.SubmitStats.ACSubmissionNum[i]
			break
		}
	}
	if acAll == nil {
		return out, fmt.Errorf("missing AC 'All' stat for %q", username)
	}

	p := resp.Data.MatchedUser.Profile

	// map to OutputUser (your requested shape)
	out.User.Username = username
	out.User.Profile.UserSlug = p.UserSlug
	out.User.Profile.UserAvatar = p.UserAvatar
	out.User.Profile.CountryCode = p.CountryCode
	out.User.Profile.CountryName = p.CountryName
	out.User.Profile.RealName = p.RealName
	out.User.Profile.Typename = p.Typename
	out.User.Profile.TotalProblemsSolved = acAll.Count
	out.User.Profile.TotalSubmissions = acAll.Submissions // accepted submissions

	// optional logging
	s.logger.Infof("Fetched user=%s solved=%d submissions=%d country=%s",
		username, acAll.Count, acAll.Submissions, p.CountryCode)

	return out, nil
}

func decompressResponse(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer gr.Close()
		reader = gr
	case "br":
		reader = brotli.NewReader(resp.Body)
	}
	return io.ReadAll(reader)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
