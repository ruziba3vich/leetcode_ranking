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
	"strings"
	"time"

	"github.com/andybalholm/brotli"
)

// ----------------------------
// GraphQL / API data models
// ----------------------------

type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

type Badge struct {
	DisplayName string `json:"displayName"`
	Icon        string `json:"icon"`
	Typename    string `json:"__typename"`
}

type Profile struct {
	UserSlug    string `json:"userSlug"`
	UserAvatar  string `json:"userAvatar"`
	CountryCode string `json:"countryCode"`
	CountryName string `json:"countryName"`
	RealName    string `json:"realName"`
	Typename    string `json:"__typename"`
}

type User struct {
	Username    string  `json:"username"`
	NameColor   *string `json:"nameColor"`
	ActiveBadge *Badge  `json:"activeBadge"`
	Profile     Profile `json:"profile"`
	Typename    string  `json:"__typename"`
}

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

type Data struct {
	GlobalRanking GlobalRanking `json:"globalRanking"`
}

type Response struct {
	Data   Data           `json:"data"`
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

const leetcodeURL = "https://leetcode.com/graphql"

// ----------------------------
// Client
// ----------------------------

type LeetCodeClient struct {
	httpClient *http.Client
	debug      bool
	delay      time.Duration
}

func NewLeetCodeClient(debug bool, delay time.Duration) *LeetCodeClient {
	return &LeetCodeClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		debug:      debug,
		delay:      delay,
	}
}

// ----------------------------
// HTTP helpers
// ----------------------------

func decompressResponse(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body
	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gr, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gr.Close()
		reader = gr
	case "br":
		reader = brotli.NewReader(resp.Body)
		// deflate is handled by http.Client automatically
	}
	return io.ReadAll(reader)
}

// ----------------------------
// API calls
// ----------------------------

func (c *LeetCodeClient) FetchGlobalRanking(page int) (*Response, error) {
	if c.debug {
		log.Printf("DEBUG: Fetching globalRanking page=%d", page)
	}

	query := `query globalRanking($page: Int) {
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
        activeBadge {
          displayName
          icon
          __typename
        }
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

	reqBody := GraphQLRequest{
		Query: query,
		Variables: map[string]interface{}{
			"page": page,
		},
	}
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if c.debug {
		log.Printf("DEBUG: Request payload: %s", truncate(string(jsonData), 800))
	}

	req, err := http.NewRequest("POST", leetcodeURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Headers mimicking browser (helps with br/gzip responses & CORS-y behavior)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://leetcode.com")
	req.Header.Set("Referer", "https://leetcode.com/contest/globalranking/")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "same-origin")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.debug {
			log.Printf("DEBUG: HTTP error: %v", err)
		}
		return nil, fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if c.debug {
		log.Printf("DEBUG: Status: %s", resp.Status)
	}

	body, err := decompressResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("decompress: %w", err)
	}

	if c.debug {
		log.Printf("DEBUG: Resp body (%d bytes): %s", len(body), truncate(string(body), 800))
	}

	if resp.StatusCode != http.StatusOK {
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		return nil, fmt.Errorf("unexpected status %d, body: %s", resp.StatusCode, truncate(string(body), 800))
	}

	var out Response
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}

	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %+v", out.Errors)
	}

	return &out, nil
}

// FetchAllPages pulls page=startPage, learns totalPages, then continues.
// If maxPages <= 0, it fetches ALL from startPage..totalPages.
// Otherwise it fetches up to startPage+maxPages-1 (bounded by totalPages).
func (c *LeetCodeClient) FetchAllPages(startPage, maxPages int) ([]RankingNode, error) {
	if startPage < 1 {
		startPage = 1
	}

	first, err := c.FetchGlobalRanking(startPage)
	if err != nil {
		return nil, fmt.Errorf("first page fetch failed: %w", err)
	}

	totalPages := first.Data.GlobalRanking.TotalPages
	nodes := append([]RankingNode{}, first.Data.GlobalRanking.RankingNodes...)

	// Determine end page
	endPage := totalPages
	if maxPages > 0 {
		if end := startPage + maxPages - 1; end < endPage {
			endPage = end
		}
	}

	for page := startPage + 1; page <= endPage; page++ {
		fmt.Printf("Fetching page %d/%d...\n", page, endPage)
		resp, err := c.FetchGlobalRanking(page)
		if err != nil {
			log.Printf("WARN: Failed to fetch page %d: %v", page, err)
			continue
		}
		nodes = append(nodes, resp.Data.GlobalRanking.RankingNodes...)
		time.Sleep(c.delay)
	}

	return nodes, nil
}

// (Optional) Your earlier multi-page function retained for reference
func (c *LeetCodeClient) FetchMultiplePages(startPage, numPages int) ([]RankingNode, error) {
	var all []RankingNode
	for p := startPage; p < startPage+numPages; p++ {
		fmt.Printf("Fetching page %d...\n", p)
		resp, err := c.FetchGlobalRanking(p)
		if err != nil {
			fmt.Printf("Failed to fetch page %d: %v\n", p, err)
			continue
		}
		all = append(all, resp.Data.GlobalRanking.RankingNodes...)
		time.Sleep(c.delay)
	}
	return all, nil
}

// ----------------------------
// Utilities
// ----------------------------

func SaveToJSON(data interface{}, filename string) error {
	j, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal to json: %w", err)
	}
	return os.WriteFile(filename, j, 0644)
}

func formatNumber(n int) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}
	var b []rune
	for i, d := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			b = append(b, ',')
		}
		b = append(b, d)
	}
	return string(b)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

// Pretty printer for a whole page response (optional).
func DisplayRankingData(resp *Response) {
	r := resp.Data.GlobalRanking
	fmt.Printf("Total Users: %s\n", formatNumber(r.TotalUsers))
	fmt.Printf("Total Pages: %s\n", formatNumber(r.TotalPages))
	fmt.Printf("Users per Page: %d\n", r.UserPerPage)
	fmt.Println(strings.Repeat("-", 80))
	for _, n := range r.RankingNodes {
		printNode(n)
	}
}

func printNode(n RankingNode) {
	fmt.Printf("Rank: %d\n", n.CurrentGlobalRank)
	fmt.Printf("Username: %s\n", n.User.Username)

	real := "N/A"
	if n.User.Profile.RealName != "" {
		real = n.User.Profile.RealName
	}
	fmt.Printf("Real Name: %s\n", real)
	fmt.Printf("Rating: %s\n", n.CurrentRating)

	cn := "N/A"
	cc := "N/A"
	if n.User.Profile.CountryName != "" {
		cn = n.User.Profile.CountryName
	}
	if n.User.Profile.CountryCode != "" {
		cc = n.User.Profile.CountryCode
	}
	fmt.Printf("Country: %s (%s)\n", cn, cc)
	fmt.Printf("Region: %s\n", n.DataRegion)

	if n.User.ActiveBadge != nil {
		fmt.Printf("Badge: %s\n", n.User.ActiveBadge.DisplayName)
	}
	fmt.Println(strings.Repeat("-", 40))
}

// Minimal sanity probe (optional)
func testSimpleQuery(client *LeetCodeClient) error {
	log.Println("DEBUG: Testing with a simple query first...")
	simpleQuery := `{
		globalRanking(page: 1) {
			totalUsers
		}
	}`
	reqBody := GraphQLRequest{Query: simpleQuery, Variables: map[string]interface{}{}}
	jsonData, _ := json.Marshal(reqBody)
	req, _ := http.NewRequest("POST", leetcodeURL, bytes.NewBuffer(jsonData))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "gzip, deflate, br")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36")
	req.Header.Set("Origin", "https://leetcode.com")
	req.Header.Set("Referer", "https://leetcode.com/contest/globalranking/")

	resp, err := client.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("simple query failed: %w", err)
	}
	defer resp.Body.Close()
	body, err := decompressResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to decompress simple query response: %w", err)
	}
	log.Printf("DEBUG: Simple query response (%d): %s", resp.StatusCode, truncate(string(body), 400))
	return nil
}

// ----------------------------
// main
// ----------------------------

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// CLI flags for convenience
	start := flag.Int("start", 1, "Start page (1-indexed)")
	maxPages := flag.Int("pages", 4, "Number of pages to fetch (<=0 to fetch all)")
	out := flag.String("out", "leetcode_rankings_pages.json", "Output JSON file")
	debug := flag.Bool("debug", true, "Enable debug logging")
	delayMs := flag.Int("delay_ms", 1000, "Delay between page requests (milliseconds)")
	runProbe := flag.Bool("probe", false, "Run a simple query probe first")
	flag.Parse()

	client := NewLeetCodeClient(*debug, time.Duration(*delayMs)*time.Millisecond)

	fmt.Printf("Fetching LeetCode Global Rankings (pages starting at %d, count %d)...\n", *start, *maxPages)

	if *runProbe {
		if err := testSimpleQuery(client); err != nil {
			log.Printf("Probe failed: %v", err)
		}
	}

	nodes, err := client.FetchAllPages(*start, *maxPages)
	if err != nil {
		log.Fatalf("Failed to fetch pages: %v", err)
	}

	fmt.Printf("Fetched %d users total\n", len(nodes))

	if err := SaveToJSON(nodes, *out); err != nil {
		log.Printf("Failed to save to file: %v", err)
	} else {
		fmt.Printf("Data saved to %s\n", *out)
	}
}
