package service

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	"github.com/k0kubun/pp"
	"github.com/ruziba3vich/leetcode_ranking/internal/dto"
	"github.com/ruziba3vich/leetcode_ranking/internal/errors_"
	"github.com/ruziba3vich/leetcode_ranking/internal/models"
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

// Consolidated response types
type GraphQLError struct {
	Message    string                 `json:"message"`
	Locations  []GraphQLErrorLocation `json:"locations,omitempty"`
	Path       []string               `json:"path,omitempty"`
	Extensions map[string]interface{} `json:"extensions,omitempty"`
}

type GraphQLErrorLocation struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

// User-related types
type ACStat struct {
	Difficulty  string `json:"difficulty"`
	Count       int    `json:"count"`
	Submissions int    `json:"submissions"`
}

type SubmitStats struct {
	ACSubmissionNum    []ACStat `json:"acSubmissionNum"`
	TotalSubmissionNum []ACStat `json:"totalSubmissionNum"`
}

type Profile struct {
	UserSlug    string `json:"userSlug"`
	UserAvatar  string `json:"userAvatar"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"countryName"`
	RealName    string `json:"realName"`
	Typename    string `json:"__typename"`
}

type Badge struct {
	DisplayName string `json:"displayName"`
	Icon        string `json:"icon"`
	Typename    string `json:"__typename"`
}

type User struct {
	Username    string  `json:"username"`
	NameColor   *string `json:"nameColor"`
	ActiveBadge *Badge  `json:"activeBadge"`
	Profile     Profile `json:"profile"`
	Typename    string  `json:"__typename"`
}

type MatchedUser struct {
	SubmitStats SubmitStats `json:"submitStats"`
	Profile     Profile     `json:"profile"`
}

// Ranking-related types
type RankingNode struct {
	Ranking           string `json:"ranking"`
	CurrentRating     string `json:"currentRating"`
	CurrentGlobalRank int    `json:"currentGlobalRanking"`
	DataRegion        string `json:"dataRegion"`
	User              User   `json:"user"`
	Typename          string `json:"__typename"`
}

type GlobalRanking struct {
	TotalUsers   int           `json:"totalUsers"`
	TotalPages   int           `json:"totalPages"`
	UserPerPage  int           `json:"userPerPage"`
	RankingNodes []RankingNode `json:"rankingNodes"`
	Typename     string        `json:"__typename"`
}

// Unified response types
type ResponseUser struct {
	Data struct {
		AllQuestionsCount []struct {
			Difficulty string `json:"difficulty"`
			Count      int    `json:"count"`
		} `json:"allQuestionsCount"`
		MatchedUser *MatchedUser `json:"matchedUser"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

type ResponseGlobal struct {
	Data struct {
		GlobalRanking GlobalRanking `json:"globalRanking"`
	} `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

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
	BatchSize int           // users to process in each batch
}

// OPTIMIZED: Single method that handles both fetching and converting user data
func (s *userService) fetchAndConvertUser(username string) (*models.StageUserDataParams, error) {
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, fmt.Errorf("username is required")
	}

	var out ResponseUser
	if err := s.leetCodeClient.doGraphQL(queryMatchedUser, map[string]interface{}{"username": username}, &out); err != nil {
		return nil, fmt.Errorf("leetcode fetch failed for %q: %w", username, err)
	}

	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors for user %q: %+v", username, out.Errors)
	}

	if out.Data.MatchedUser == nil {
		return nil, errors_.ErrUserNotAvailable
	}

	// Find AC stats for "All" difficulty
	var acAll *ACStat
	for i := range out.Data.MatchedUser.SubmitStats.ACSubmissionNum {
		if out.Data.MatchedUser.SubmitStats.ACSubmissionNum[i].Difficulty == "All" {
			acAll = &out.Data.MatchedUser.SubmitStats.ACSubmissionNum[i]
			break
		}
	}

	if acAll == nil {
		return nil, fmt.Errorf("missing AC 'All' statistics for user %q", username)
	}

	profile := out.Data.MatchedUser.Profile

	// Optional logging
	s.logger.Infof("Fetched user=%s solved=%d submissions=%d country=%s",
		username, acAll.Count, acAll.Submissions, profile.CountryName)

	return &models.StageUserDataParams{
		Username:            username,
		UserSlug:            profile.UserSlug,
		UserAvatar:          profile.UserAvatar,
		CountryCode:         profile.CountryCode,
		CountryName:         profile.CountryName,
		RealName:            profile.RealName,
		Typename:            profile.Typename,
		TotalProblemsSolved: int32(acAll.Count),
		TotalSubmissions:    int32(acAll.Submissions),
	}, nil
}

// OPTIMIZED: Concurrent user processing with worker pools
func (s *userService) processUsersConcurrently(ctx context.Context, usernames []string, workers int, delay time.Duration) ([]*models.StageUserDataParams, error) {
	if workers <= 0 {
		workers = 1
	}

	jobs := make(chan string, len(usernames))
	results := make(chan *models.StageUserDataParams, len(usernames))
	errors := make(chan error, len(usernames))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for username := range jobs {
				// select {
				// case <-ctx.Done():
				// 	errors <- ctx.Err()
				// 	return
				// default:
				// }

				user, err := s.fetchAndConvertUser(username)
				if err != nil {
					s.logger.Error("failed to fetch user", map[string]any{"username": username, "error": err})
					errors <- err
				} else {
					results <- user
				}

				// Polite delay between requests
				if delay > 0 {
					time.Sleep(delay)
				}
			}
		}()
	}

	// Send jobs
	go func() {
		defer close(jobs)
		for _, username := range usernames {
			select {
			// case <-ctx.Done():
			// 	return
			case jobs <- username:
			}
		}
	}()

	// Wait for workers to complete
	go func() {
		wg.Wait()
		close(results)
		close(errors)
	}()

	// Collect results
	var users []*models.StageUserDataParams
	var errs []error

	for {
		select {
		case user, ok := <-results:
			if !ok {
				results = nil
			} else {
				users = append(users, user)
			}
		case err, ok := <-errors:
			if !ok {
				errors = nil
			} else {
				errs = append(errs, err)
			}
			// case <-ctx.Done():
			// 	return nil, ctx.Err()
		}

		if results == nil && errors == nil {
			break
		}
	}

	if len(errs) > 0 {
		s.logger.Warnf("encountered %d errors while processing %d users", len(errs), len(usernames))
	}

	return users, nil
}

// OPTIMIZED: Main sync method with improved batching and concurrency
func (s *userService) SyncLeaderboard(ctx context.Context, opts SyncOptions) error {
	pp.Println("------------------ starting synchronization -----------------")

	// Set defaults
	if opts.StartPage < 1 {
		opts.StartPage = 1
	}
	if opts.Delay <= 0 {
		opts.Delay = 800 * time.Millisecond
	}
	if opts.Workers <= 0 {
		opts.Workers = 3 // Slightly more aggressive default
	}
	if opts.BatchSize <= 0 {
		opts.BatchSize = 100 // Process users in batches
	}

	pp.Printf("sync: starting page-by-page sync from page %d, delay=%s, workers=%d, batch_size=%d\n",
		opts.StartPage, opts.Delay, opts.Workers, opts.BatchSize)

	// Get first page to determine total pages
	firstPage, err := s.fetchRankingPage(opts.StartPage)
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

	// Process pages in batches
	for currentPage := opts.StartPage; s.sync && currentPage <= endPage; currentPage++ {
		s.syncingPage = currentPage
		// select {
		// case <-ctx.Done():
		// 	s.logger.Errorf("sync: context canceled at page %d", currentPage)
		// 	return ctx.Err()
		// default:
		// }

		pp.Printf("sync: processing page %d/%d\n", currentPage, endPage)

		// Fetch current page (reuse first page data if it's the start page)
		var pageResp *ResponseGlobal
		if currentPage == opts.StartPage && firstPage != nil {
			pageResp = firstPage
		} else {
			pageResp, err = s.fetchRankingPage(currentPage)
			if err != nil {
				s.logger.Errorf("sync: failed to fetch page %d: %v", currentPage, err)
				continue // Skip this page and continue with next
			}
		}

		// Extract usernames from current page
		usernames := s.extractUsernamesFromPage(pageResp)
		pp.Printf("sync: page %d contains %d users\n", currentPage, len(usernames))

		// Process users concurrently
		users, err := s.processUsersConcurrently(ctx, usernames, opts.Workers, opts.Delay)
		if err != nil {
			s.logger.Errorf("sync: failed to process users on page %d: %v", currentPage, err)
			continue
		}

		// Batch insert users
		if len(users) > 0 {
			err := s.dbStorage.UpsertUserData(ctx, users)
			if err != nil {
				s.logger.Error("failed to sync users", map[string]any{"page": currentPage, "count": len(users)})
			} else {
				totalProcessedUsers += len(users)
				s.logger.Infof("sync: completed page %d/%d - processed %d users (total: %d)",
					currentPage, endPage, len(users), totalProcessedUsers)
			}
		}

		// Optional: delay between pages
		if currentPage < endPage {
			time.Sleep(opts.Delay)
		}
	}

	s.logger.Infof("sync: completed all pages. Total processed users: %d", totalProcessedUsers)
	pp.Println("------------------ synchronization completed -----------------")
	return nil
}

func (s *userService) GetSyncStatus() *dto.GetSyncStatusResponse {
	return &dto.GetSyncStatusResponse{
		IsOn: s.sync,
		Page: s.syncingPage,
	}
}

// OPTIMIZED: Simplified page fetching
func (s *userService) fetchRankingPage(page int) (*ResponseGlobal, error) {
	var out ResponseGlobal
	if err := s.leetCodeClient.doGraphQL(queryGlobalRanking, map[string]interface{}{"page": page}, &out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %+v", out.Errors)
	}
	return &out, nil
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

// SIMPLIFIED: Single method for external API calls (replaces FetchLeetCodeUser)
func (s *userService) GetUserData(ctx context.Context, username string) (*models.StageUserDataParams, error) {
	return s.fetchAndConvertUser(username)
}

// Core GraphQL execution method (unchanged but renamed for clarity)
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
