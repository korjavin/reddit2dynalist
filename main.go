package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
)

const (
	redditRedirectURI = "http://localhost:8080/callback"
	redditTokenFile   = "reddit_refresh_token.txt"
)

// RedditCredentials contains the credentials needed for Reddit API
type RedditCredentials struct {
	ClientID string
}

// RedditClient handles interactions with the Reddit API
type RedditClient struct {
	HTTPClient *http.Client
	UserAgent  string
}

// RedditPost represents a saved post or comment from Reddit
type RedditPost struct {
	Kind      string  `json:"kind"`
	ID        string  `json:"id"`
	FullID    string  `json:"name"`
	Title     string  `json:"title,omitempty"`
	Author    string  `json:"author"`
	Permalink string  `json:"permalink"`
	URL       string  `json:"url,omitempty"`
	Created   float64 `json:"created_utc"`
	IsComment bool    `json:"-"` // Internal field
}

// RedditResponse represents the response from Reddit API
type RedditResponse struct {
	Kind string `json:"kind"`
	Data struct {
		Children []struct {
			Kind string     `json:"kind"`
			Data RedditPost `json:"data"`
		} `json:"children"`
		After string `json:"after"`
	} `json:"data"`
}

// Cache stores post IDs to avoid duplicates
type Cache struct {
	Posts map[string]time.Time
}

// SaveToFile saves the cache to a file
func (c *Cache) SaveToFile(filename string) error {
	data, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %w", err)
	}
	return os.WriteFile(filename, data, 0644)
}

// LoadCacheFromFile loads the cache from a file
func LoadCacheFromFile(filename string) (*Cache, error) {
	cache := &Cache{Posts: make(map[string]time.Time)}
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return cache, nil
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}
	if err := json.Unmarshal(data, cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache: %w", err)
	}
	return cache, nil
}

// NewRedditClient creates a new Reddit client using the installed app flow
func NewRedditClient(clientID, refreshToken string) (*RedditClient, error) {
	ctx := context.Background()
	oauth2Config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: "",
		RedirectURL:  redditRedirectURI,
		Scopes:       []string{"history", "identity", "read", "save"},
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://www.reddit.com/api/v1/access_token",
			AuthURL:  "https://www.reddit.com/api/v1/authorize",
		},
	}
	token := &oauth2.Token{RefreshToken: refreshToken}
	tokenSource := oauth2Config.TokenSource(ctx, token)
	httpClient := oauth2.NewClient(ctx, tokenSource)
	httpClient.Timeout = 30 * time.Second
	userAgent := "script:reddit2dynalist:v1.0 (by /u/yourusername)" // Change to your Reddit username
	return &RedditClient{
		HTTPClient: httpClient,
		UserAgent:  userAgent,
	}, nil
}

// One-time: Run this to get a refresh token
func getRedditRefreshToken(clientID string) (string, error) {
	oauth2Config := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: "",
		RedirectURL:  redditRedirectURI,
		Scopes:       []string{"history", "identity", "read", "save"},
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://www.reddit.com/api/v1/access_token",
			AuthURL:  "https://www.reddit.com/api/v1/authorize",
		},
	}
	fmt.Printf("DEBUG: Using clientID=%q, redirectURI=%q\n", clientID, redditRedirectURI)
	state := fmt.Sprintf("%d", rand.Int())
	authURL := oauth2Config.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	fmt.Println("Go to the following URL in your browser and authorize the app:")
	fmt.Println(authURL)

	codeCh := make(chan string)
	srv := &http.Server{Addr: ":8080"}
	http.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		receivedState := r.URL.Query().Get("state")
		code := r.URL.Query().Get("code")
		fmt.Printf("DEBUG: Received callback with state=%q, code=%q\n", receivedState, code)
		if receivedState != state {
			http.Error(w, "State mismatch", http.StatusBadRequest)
			return
		}
		fmt.Fprintf(w, "Authorization successful! You can close this window.")
		codeCh <- code
		go srv.Shutdown(context.Background())
	})
	go func() { _ = srv.ListenAndServe() }()
	code := <-codeCh

	fmt.Printf("DEBUG: Exchanging code: %q\n", code)
	ctx := context.Background()
	token, err := oauth2Config.Exchange(ctx, code)
	if err != nil {
		fmt.Printf("DEBUG: Exchange error: %v\n", err)
		return "", fmt.Errorf("failed to exchange code for token: %w", err)
	}
	return token.RefreshToken, nil
}

func (r *RedditClient) GetSavedPosts(ctx context.Context, username string, limit int) ([]RedditPost, error) {
	url := fmt.Sprintf("https://oauth.reddit.com/user/%s/saved?limit=%d&sort=new", username, limit)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("User-Agent", r.UserAgent)
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Reddit API error: %s, Body: %s", resp.Status, string(body))
	}
	var redditResp RedditResponse
	if err := json.NewDecoder(resp.Body).Decode(&redditResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	var posts []RedditPost
	for _, child := range redditResp.Data.Children {
		post := child.Data
		post.FullID = child.Kind + "_" + post.ID
		post.IsComment = (child.Kind == "t1")
		posts = append(posts, post)
	}
	return posts, nil
}

func main() {
	authorize := flag.Bool("authorize", false, "Run OAuth2 authorization flow to get refresh token")
	flag.Parse()

	clientID := os.Getenv("REDDIT_CLIENT_ID")
	username := os.Getenv("REDDIT_USERNAME")
	dynalistKey := os.Getenv("DYNALIST_API_KEY")
	if clientID == "" || username == "" || dynalistKey == "" {
		log.Fatal("Missing required environment variables. Please set REDDIT_CLIENT_ID, REDDIT_USERNAME, and DYNALIST_API_KEY")
	}

	if *authorize {
		refreshToken, err := getRedditRefreshToken(clientID)
		if err != nil {
			log.Fatalf("Failed to get refresh token: %v", err)
		}
		fmt.Printf("Your refresh token (save this!):\n%s\n", refreshToken)
		if err := os.WriteFile(redditTokenFile, []byte(refreshToken), 0600); err != nil {
			log.Fatalf("Failed to save refresh token: %v", err)
		}
		fmt.Println("Refresh token saved to", redditTokenFile)
		return
	}

	refreshTokenBytes, err := os.ReadFile(redditTokenFile)
	if err != nil {
		log.Fatalf("Failed to read refresh token file: %v", err)
	}
	refreshToken := string(refreshTokenBytes)

	redditClient, err := NewRedditClient(clientID, refreshToken)
	if err != nil {
		log.Fatal("Failed to create Reddit client:", err)
	}

	cacheFile := "reddit2dynalist.cache.json"
	cache, err := LoadCacheFromFile(cacheFile)
	if err != nil {
		log.Printf("Warning: Failed to load cache: %v. Creating a new cache.", err)
		cache = &Cache{Posts: make(map[string]time.Time)}
	}
	log.Printf("Loaded cache with %d previously processed posts", len(cache.Posts))

	ticker := time.NewTicker(5 * time.Minute)
	log.Printf("Starting to check for new saved posts every 5 minutes...")

	processNewPosts(redditClient, username, dynalistKey, cache, cacheFile)
	for range ticker.C {
		processNewPosts(redditClient, username, dynalistKey, cache, cacheFile)
	}
}

func processNewPosts(
	redditClient *RedditClient,
	username string,
	dynalistKey string,
	cache *Cache,
	cacheFile string,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	posts, err := redditClient.GetSavedPosts(ctx, username, 25)
	if err != nil {
		log.Printf("Error fetching saved posts: %v", err)
		return
	}

	newPosts := 0
	for _, post := range posts {
		if _, exists := cache.Posts[post.FullID]; exists {
			continue
		}
		cache.Posts[post.FullID] = time.Now()
		var content string
		if post.IsComment {
			content = fmt.Sprintf("Comment by %s - https://reddit.com%s", post.Author, post.Permalink)
		} else if post.Title != "" {
			content = fmt.Sprintf("%s - https://reddit.com%s", post.Title, post.Permalink)
		} else {
			content = fmt.Sprintf("Post by %s - https://reddit.com%s", post.Author, post.Permalink)
		}
		title := fmt.Sprintf("Post by %s - https://reddit.com%s", post.Author, post.Permalink)
		log.Printf("Adding new saved post to Dynalist: %s", content)
		err = AddToDynalist(dynalistKey, title, content)
		if err != nil {
			log.Printf("Error creating Dynalist item: %v", err)
			continue
		}
		newPosts++
	}

	now := time.Now()
	for id, timestamp := range cache.Posts {
		if now.Sub(timestamp) > 7*24*time.Hour {
			delete(cache.Posts, id)
		}
	}

	if newPosts > 0 {
		log.Printf("Added %d new posts to Dynalist", newPosts)
	} else {
		log.Printf("No new posts found")
	}

	if err := cache.SaveToFile(cacheFile); err != nil {
		log.Printf("Warning: Failed to save cache: %v", err)
	}
}
