package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
)

const leetcodeURL = "https://leetcode.com/graphql"

// ----------------------------
// GraphQL requests
// ----------------------------

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

// ----------------------------
// Global ranking (page) types
// ----------------------------

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

// ----------------------------
// matchedUser types
// ----------------------------

type ACStat struct {
	Difficulty  string `json:"difficulty"`
	Count       int    `json:"count"`
	Submissions int    `json:"submissions"`
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

type MatchedUser struct {
	SubmitStats SubmitStats `json:"submitStats"`
	Profile     ProfileFull `json:"profile"`
}

type DataUser struct {
	AllQuestionsCount []struct {
		Difficulty string `json:"difficulty"`
		Count      int    `json:"count"`
	} `json:"allQuestionsCount"`
	MatchedUser *MatchedUser `json:"matchedUser"`
}

type ResponseUser struct {
	Data   DataUser       `json:"data"`
	Errors []GraphQLError `json:"errors,omitempty"`
}

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

// ----------------------------
// Output shape requested
// ----------------------------

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

// ----------------------------
// Client
// ----------------------------

type LeetCodeClient struct {
	httpClient *http.Client
	debug      bool
	delay      time.Duration
	headers    http.Header
}

func NewLeetCodeClient(debug bool, delay time.Duration) *LeetCodeClient {
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
		debug:      debug,
		delay:      delay,
		headers:    h,
	}
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

// ----------------------------
// Queries
// ----------------------------

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

// ----------------------------
// API methods
// ----------------------------

func (c *LeetCodeClient) FetchRankingPage(page int) (*ResponseGlobal, error) {
	var out ResponseGlobal
	if err := c.doGraphQL(queryGlobalRanking, map[string]interface{}{"page": page}, &out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %+v", out.Errors)
	}
	return &out, nil
}

func (c *LeetCodeClient) FetchUser(username string) (*ResponseUser, error) {
	var out ResponseUser
	if err := c.doGraphQL(queryMatchedUser, map[string]interface{}{"username": username}, &out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %+v", out.Errors)
	}
	return &out, nil
}

// Fetch usernames from ranking pages: start..end inclusive
func (c *LeetCodeClient) CollectUsernames(startPage, maxPages int) ([]string, int, error) {
	if startPage < 1 {
		startPage = 1
	}

	first, err := c.FetchRankingPage(startPage)
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
		resp, err := c.FetchRankingPage(p)
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
		time.Sleep(c.delay)
	}

	sort.Strings(users)
	return users, endPage, nil
}

// ----------------------------
// Decompression helper
// ----------------------------

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
		// deflate handled automatically
	}
	return io.ReadAll(reader)
}

// ----------------------------
// Utilities
// ----------------------------

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func SaveToJSON(data interface{}, file string) error {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(file, b, 0644)
}

// ----------------------------
// Main flow
// ----------------------------

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	start := flag.Int("start", 1, "Start page (1-indexed)")
	pages := flag.Int("pages", 30164, "How many pages to scrape (<=0 = all)")
	out := flag.String("out", "leetcode_users.json", "Output JSON file")
	debug := flag.Bool("debug", true, "Debug logging")
	delayMs := flag.Int("delay_ms", 800, "Delay between page requests (ms)")
	workers := flag.Int("workers", 6, "Parallel workers for per-user fetch")
	flag.Parse()

	client := NewLeetCodeClient(*debug, time.Duration(*delayMs)*time.Millisecond)

	fmt.Printf("Collecting usernames from global ranking (start=%d pages=%d)...\n", *start, *pages)
	usernames, endPage, err := client.CollectUsernames(*start, *pages)
	if err != nil {
		log.Fatalf("collect usernames: %v", err)
	}
	fmt.Printf("Collected %d usernames (through page %d)\n", len(usernames), endPage)

	type job struct {
		Username string
	}
	jobs := make(chan job)
	var mu sync.Mutex
	var results []OutputUser

	// Worker pool
	var wg sync.WaitGroup
	worker := func() {
		defer wg.Done()
		for j := range jobs {
			resp, err := client.FetchUser(j.Username)
			if err != nil || resp.Data.MatchedUser == nil {
				if err != nil {
					log.Printf("WARN: user %s fetch failed: %v", j.Username, err)
				} else {
					log.Printf("WARN: user %s missing matchedUser", j.Username)
				}
				continue
			}

			// Find AC All
			var acAll *ACStat
			for i := range resp.Data.MatchedUser.SubmitStats.ACSubmissionNum {
				if resp.Data.MatchedUser.SubmitStats.ACSubmissionNum[i].Difficulty == "All" {
					acAll = &resp.Data.MatchedUser.SubmitStats.ACSubmissionNum[i]
					break
				}
			}
			if acAll == nil {
				log.Printf("WARN: user %s missing AC 'All' stat", j.Username)
				continue
			}

			// Compose output
			var ou OutputUser
			ou.User.Username = j.Username
			ou.User.Profile.UserSlug = resp.Data.MatchedUser.Profile.UserSlug
			ou.User.Profile.UserAvatar = resp.Data.MatchedUser.Profile.UserAvatar
			ou.User.Profile.CountryCode = resp.Data.MatchedUser.Profile.CountryCode
			ou.User.Profile.CountryName = resp.Data.MatchedUser.Profile.CountryName
			ou.User.Profile.RealName = resp.Data.MatchedUser.Profile.RealName
			ou.User.Profile.Typename = resp.Data.MatchedUser.Profile.Typename
			ou.User.Profile.TotalProblemsSolved = acAll.Count
			ou.User.Profile.TotalSubmissions = acAll.Submissions // NOTE: accepted submissions

			mu.Lock()
			results = append(results, ou)
			mu.Unlock()

			// small jitter between user calls (be polite)
			time.Sleep(150 * time.Millisecond)
		}
	}

	// Start workers
	if *workers < 1 {
		*workers = 1
	}
	wg.Add(*workers)
	for w := 0; w < *workers; w++ {
		go worker()
	}

	// Enqueue jobs
	for _, u := range usernames {
		jobs <- job{Username: u}
	}
	close(jobs)

	wg.Wait()

	// Persist
	if err := SaveToJSON(results, *out); err != nil {
		log.Fatalf("save json: %v", err)
	}
	fmt.Printf("Wrote %d users to %s\n", len(results), *out)
}
