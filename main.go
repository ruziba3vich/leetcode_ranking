package main

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/andybalholm/brotli"
)

// GraphQL request structure
type GraphQLRequest struct {
	Query     string                 `json:"query"`
	Variables map[string]interface{} `json:"variables"`
}

// Response structures matching the LeetCode API
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

const (
	leetcodeURL = "https://leetcode.com/graphql"
)

// LeetCodeClient handles API requests
type LeetCodeClient struct {
	httpClient *http.Client
	debug      bool
}

// NewLeetCodeClient creates a new client
func NewLeetCodeClient(debug bool) *LeetCodeClient {
	return &LeetCodeClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		debug: debug,
	}
}

// decompressResponse handles gzip and brotli decompression
func decompressResponse(resp *http.Response) ([]byte, error) {
	var reader io.Reader = resp.Body

	switch resp.Header.Get("Content-Encoding") {
	case "gzip":
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	case "br":
		reader = brotli.NewReader(resp.Body)
	case "deflate":
		// Go's http client automatically handles deflate
	}

	return io.ReadAll(reader)
}

// FetchGlobalRanking fetches ranking data for a specific page
func (c *LeetCodeClient) FetchGlobalRanking(page int) (*Response, error) {
	if c.debug {
		log.Printf("DEBUG: Starting to fetch page %d", page)
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
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if c.debug {
		log.Printf("DEBUG: Request payload: %s", string(jsonData))
	}

	req, err := http.NewRequest("POST", leetcodeURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers similar to the network log
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

	if c.debug {
		log.Printf("DEBUG: Request URL: %s", req.URL.String())
		log.Printf("DEBUG: Request Method: %s", req.Method)
		log.Printf("DEBUG: Request Headers:")
		for name, values := range req.Header {
			log.Printf("  %s: %s", name, strings.Join(values, ", "))
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if c.debug {
			log.Printf("DEBUG: HTTP request failed: %v", err)
		}
		return nil, fmt.Errorf("failed to make request: %w", err)
	}
	defer resp.Body.Close()

	if c.debug {
		log.Printf("DEBUG: Response Status: %s (%d)", resp.Status, resp.StatusCode)
		log.Printf("DEBUG: Response Headers:")
		for name, values := range resp.Header {
			log.Printf("  %s: %s", name, strings.Join(values, ", "))
		}
	}

	// Handle compressed response
	body, err := decompressResponse(resp)
	if err != nil {
		if c.debug {
			log.Printf("DEBUG: Failed to decompress response: %v", err)
		}
		return nil, fmt.Errorf("failed to decompress response: %w", err)
	}

	if c.debug {
		log.Printf("DEBUG: Response Body Length: %d bytes", len(body))
		// Only show first 500 chars to avoid overwhelming output
		bodyStr := string(body)
		if len(bodyStr) > 500 {
			log.Printf("DEBUG: Response Body (first 500 chars): %s...", bodyStr[:500])
		} else {
			log.Printf("DEBUG: Response Body: %s", bodyStr)
		}
	}

	if resp.StatusCode != http.StatusOK {
		// Try to parse the error response
		var errorResp map[string]interface{}
		if err := json.Unmarshal(body, &errorResp); err == nil {
			if c.debug {
				log.Printf("DEBUG: Parsed error response: %+v", errorResp)
			}
		}
		return nil, fmt.Errorf("unexpected status code: %d, response: %s", resp.StatusCode, string(body))
	}

	var response Response
	if err := json.Unmarshal(body, &response); err != nil {
		if c.debug {
			log.Printf("DEBUG: JSON unmarshal error: %v", err)
			log.Printf("DEBUG: Raw response for debugging: %s", string(body))
		}
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if c.debug {
		log.Printf("DEBUG: Successfully parsed response")
		if len(response.Errors) > 0 {
			log.Printf("DEBUG: GraphQL errors found: %+v", response.Errors)
		}
	}

	// Check for GraphQL errors
	if len(response.Errors) > 0 {
		return nil, fmt.Errorf("GraphQL errors: %+v", response.Errors)
	}

	return &response, nil
}

// FetchMultiplePages fetches multiple pages of ranking data
func (c *LeetCodeClient) FetchMultiplePages(startPage, numPages int) ([]RankingNode, error) {
	var allRankings []RankingNode

	for page := startPage; page < startPage+numPages; page++ {
		fmt.Printf("Fetching page %d...\n", page)

		resp, err := c.FetchGlobalRanking(page)
		if err != nil {
			fmt.Printf("Failed to fetch page %d: %v\n", page, err)
			continue
		}

		allRankings = append(allRankings, resp.Data.GlobalRanking.RankingNodes...)

		// Be respectful with API calls
		time.Sleep(1 * time.Second)
	}

	return allRankings, nil
}

// DisplayRankingData prints the ranking data in a readable format
func DisplayRankingData(resp *Response) {
	ranking := resp.Data.GlobalRanking

	fmt.Printf("Total Users: %s\n", formatNumber(ranking.TotalUsers))
	fmt.Printf("Total Pages: %s\n", formatNumber(ranking.TotalPages))
	fmt.Printf("Users per Page: %d\n", ranking.UserPerPage)
	fmt.Println(strings.Repeat("-", 80))

	for _, node := range ranking.RankingNodes {
		fmt.Printf("Rank: %d\n", node.CurrentGlobalRank)
		fmt.Printf("Username: %s\n", node.User.Username)

		realName := "N/A"
		if node.User.Profile.RealName != "" {
			realName = node.User.Profile.RealName
		}
		fmt.Printf("Real Name: %s\n", realName)

		fmt.Printf("Rating: %s\n", node.CurrentRating)

		countryName := "N/A"
		countryCode := "N/A"
		if node.User.Profile.CountryName != "" {
			countryName = node.User.Profile.CountryName
		}
		if node.User.Profile.CountryCode != "" {
			countryCode = node.User.Profile.CountryCode
		}
		fmt.Printf("Country: %s (%s)\n", countryName, countryCode)
		fmt.Printf("Region: %s\n", node.DataRegion)

		if node.User.ActiveBadge != nil {
			fmt.Printf("Badge: %s\n", node.User.ActiveBadge.DisplayName)
		}

		fmt.Println(strings.Repeat("-", 40))
	}
}

// SaveToJSON saves the response data to a JSON file
func SaveToJSON(data interface{}, filename string) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	return os.WriteFile(filename, jsonData, 0644)
}

// formatNumber adds comma separators to numbers
func formatNumber(n int) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	var result []rune
	for i, digit := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, digit)
	}
	return string(result)
}

// testSimpleQuery tests with a minimal GraphQL query first
func testSimpleQuery(client *LeetCodeClient) error {
	log.Println("DEBUG: Testing with a simple query first...")

	simpleQuery := `{
		globalRanking(page: 1) {
			totalUsers
		}
	}`

	reqBody := GraphQLRequest{
		Query:     simpleQuery,
		Variables: map[string]interface{}{},
	}

	jsonData, _ := json.Marshal(reqBody)
	log.Printf("DEBUG: Simple query payload: %s", string(jsonData))

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

	// Handle compressed response for simple query too
	body, err := decompressResponse(resp)
	if err != nil {
		return fmt.Errorf("failed to decompress simple query response: %w", err)
	}

	log.Printf("DEBUG: Simple query response (%d): %s", resp.StatusCode, string(body))

	return nil
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	client := NewLeetCodeClient(true) // Enable debug mode

	fmt.Println("Fetching LeetCode Global Rankings with DEBUG enabled...")

	// Test with a simple query first
	if err := testSimpleQuery(client); err != nil {
		log.Printf("Simple query test failed: %v", err)
	}

	// Try the full query
	resp, err := client.FetchGlobalRanking(1)
	if err != nil {
		log.Fatalf("Failed to fetch ranking data: %v", err)
	}

	log.Println("DEBUG: Successfully fetched data!")
	DisplayRankingData(resp)

	// Save to file
	if err := SaveToJSON(resp, "leetcode_rankings.json"); err != nil {
		log.Printf("Failed to save to file: %v", err)
	} else {
		fmt.Println("\nData saved to leetcode_rankings.json")
	}
}
