package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"golang.org/x/oauth2"
)

// RedditCredentials contains the credentials needed for Reddit API
type RedditCredentials struct {
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
}

// RedditClient handles interactions with the Reddit API
type RedditClient struct {
	Credentials RedditCredentials
	HTTPClient  *http.Client
	UserAgent   string
}

// RedditPost represents a saved post or comment from Reddit
type RedditPost struct {
	Kind      string `json:"kind"`
	ID        string `json:"id"`
	FullID    string `json:"name"`
	Title     string `json:"title,omitempty"`
	Author    string `json:"author"`
	Permalink string `json:"permalink"`
	URL       string `json:"url,omitempty"`
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

// DynalistClient handles API interactions with Dynalist
type DynalistClient struct {
	APIKey  string
	BaseURL string
	HTTP    *http.Client
}

// DynalistResponse represents the response from Dynalist API
type DynalistResponse struct {
	Code    int             `json:"_code"`
	Message string          `json:"_msg"`
	Data    json.RawMessage `json:"data,omitempty"`
}

// DynalistEditRequest represents the request to edit a document
type DynalistEditRequest struct {
	Token    string          `json:"token"`
	FileID   string          `json:"file_id"`
	Changes  []DynalistChange `json:"changes"`
}

// DynalistChange represents a single change operation in Dynalist
type DynalistChange struct {
	Action    string `json:"action"`
	ParentID  string `json:"parent_id,omitempty"`
	Content   string `json:"content,omitempty"`
	Index     int    `json:"index,omitempty"`
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
	// Create an empty cache in case the file doesn't exist
	cache := &Cache{
		Posts: make(map[string]time.Time),
	}
	
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// Return empty cache if file doesn't exist
			return cache, nil
		}
		return nil, fmt.Errorf("failed to read cache file: %w", err)
	}
	
	if err := json.Unmarshal(data, cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache: %w", err)
	}
	
	return cache, nil
}

// NewRedditClient creates a new Reddit client
func NewRedditClient(credentials RedditCredentials) (*RedditClient, error) {
	// Set up OAuth2 configuration for Reddit
	ctx := context.Background()
	oauth2Config := &oauth2.Config{
		ClientID:     credentials.ClientID,
		ClientSecret: credentials.ClientSecret,
		Endpoint: oauth2.Endpoint{
			TokenURL: "https://www.reddit.com/api/v1/access_token",
			AuthURL:  "https://www.reddit.com/api/v1/authorize",
		},
	}
	
	// Create an OAuth2 token for password credentials
	token, err := oauth2Config.PasswordCredentialsToken(ctx, credentials.Username, credentials.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to get OAuth token: %v", err)
	}
	
	// Create HTTP client with OAuth2
	httpClient := oauth2Config.Client(ctx, token)
	httpClient.Timeout = time.Second * 30
	
	// Create Reddit client with user agent
	userAgent := fmt.Sprintf("script:reddit2dynalist:v1.0 (by /u/%s)", credentials.Username)
	
	return &RedditClient{
		Credentials: credentials,
		HTTPClient:  httpClient,
		UserAgent:   userAgent,
	}, nil
}

// GetSavedPosts retrieves saved posts from Reddit
func (r *RedditClient) GetSavedPosts(ctx context.Context, limit int) ([]RedditPost, error) {
	url := fmt.Sprintf("https://oauth.reddit.com/user/%s/saved?limit=%d&sort=new", 
		r.Credentials.Username, limit)
	
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

// VerifyAuthentication verifies that the client can authenticate with Reddit
func (r *RedditClient) VerifyAuthentication(ctx context.Context) error {
	url := fmt.Sprintf("https://oauth.reddit.com/api/v1/me")
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	req.Header.Set("User-Agent", r.UserAgent)
	
	resp, err := r.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("Reddit API error: %s, Body: %s", resp.Status, string(body))
	}
	
	var user struct {
		Name string `json:"name"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}
	
	if user.Name != r.Credentials.Username {
		return fmt.Errorf("authenticated as %s instead of %s", user.Name, r.Credentials.Username)
	}
	
	return nil
}

// NewDynalistClient creates a new Dynalist client
func NewDynalistClient(apiKey string) *DynalistClient {
	return &DynalistClient{
		APIKey:  apiKey,
		BaseURL: "https://dynalist.io/api/v1",
		HTTP:    &http.Client{Timeout: 10 * time.Second},
	}
}

// CreateItem creates a new item in a Dynalist document
func (d *DynalistClient) CreateItem(documentID string, content string) error {
	// Create a change object to add a new item at the root level
	change := DynalistChange{
		Action:   "insert",
		ParentID: "root", // Add at root level
		Content:  content,
		Index:    0,      // Add at the beginning
	}
	
	req := DynalistEditRequest{
		Token:   d.APIKey,
		FileID:  documentID,
		Changes: []DynalistChange{change},
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/doc/edit", d.BaseURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := d.HTTP.Do(httpReq)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var dynalistResp DynalistResponse
	if err := json.NewDecoder(resp.Body).Decode(&dynalistResp); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if dynalistResp.Code != 0 {
		return fmt.Errorf("dynalist API error: %s", dynalistResp.Message)
	}

	return nil
}

// GetDocumentID fetches the ID of a document by its name
func (d *DynalistClient) GetDocumentID(name string) (string, error) {
	req := map[string]string{
		"token": d.APIKey,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/file/list", d.BaseURL)
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	resp, err := d.HTTP.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var dynalistResp DynalistResponse
	if err := json.NewDecoder(resp.Body).Decode(&dynalistResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	if dynalistResp.Code != 0 {
		return "", fmt.Errorf("dynalist API error: %s", dynalistResp.Message)
	}

	// Parse the list of files and find the one with the given name
	var fileList struct {
		Files []struct {
			ID   string `json:"id"`
			Name string `json:"title"`
			Type string `json:"type"`
		} `json:"files"`
	}

	if err := json.Unmarshal(dynalistResp.Data, &fileList); err != nil {
		return "", fmt.Errorf("failed to unmarshal file list: %w", err)
	}

	for _, file := range fileList.Files {
		if file.Name == name && file.Type == "document" {
			return file.ID, nil
		}
	}

	return "", fmt.Errorf("document '%s' not found", name)
}

func main() {
	// Set up cache file location
	cacheFile := "reddit2dynalist.cache.json"
	
	// Load cache from file or create a new one
	cache, err := LoadCacheFromFile(cacheFile)
	if err != nil {
		log.Printf("Warning: Failed to load cache: %v. Creating a new cache.", err)
		cache = &Cache{
			Posts: make(map[string]time.Time),
		}
	}
	
	log.Printf("Loaded cache with %d previously processed posts", len(cache.Posts))

	// Load credentials from environment variables
	credentials := RedditCredentials{
		ClientID:     os.Getenv("REDDIT_CLIENT_ID"),
		ClientSecret: os.Getenv("REDDIT_CLIENT_SECRET"),
		Username:     os.Getenv("REDDIT_USERNAME"),
		Password:     os.Getenv("REDDIT_PASSWORD"),
	}

	// Validate all required environment variables
	if credentials.ClientID == "" || credentials.ClientSecret == "" || 
	   credentials.Username == "" || credentials.Password == "" ||
	   os.Getenv("DYNALIST_API_KEY") == "" {
		log.Fatal("Missing required environment variables. Please set REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET, REDDIT_USERNAME, REDDIT_PASSWORD, and DYNALIST_API_KEY")
	}

	// Create Reddit client
	redditClient, err := NewRedditClient(credentials)
	if err != nil {
		log.Fatal("Failed to create Reddit client:", err)
	}

	// Verify Reddit authentication
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	if err := redditClient.VerifyAuthentication(ctx); err != nil {
		cancel()
		log.Fatalf("Failed to authenticate with Reddit: %v", err)
	}
	cancel()
	log.Printf("Successfully authenticated as: %s", credentials.Username)

	// Create Dynalist client
	dynalist := NewDynalistClient(os.Getenv("DYNALIST_API_KEY"))

	// Get document ID for "Reddit" document
	documentID, err := dynalist.GetDocumentID("Reddit")
	if err != nil {
		log.Printf("Warning: Could not find 'Reddit' document: %v", err)
		log.Printf("Please create a document named 'Reddit' in your Dynalist account")
		log.Printf("Using a placeholder ID for now...")
		documentID = "your_document_id_here"
	}

	log.Printf("Using Dynalist document ID: %s", documentID)

	// Set up ticker for periodic checking (5 minutes)
	ticker := time.NewTicker(5 * time.Minute)
	log.Printf("Starting to check for new saved posts every 5 minutes...")

	// Process saved posts immediately on startup
	processNewPosts(redditClient, dynalist, documentID, cache, cacheFile)

	// Then process on each tick
	for range ticker.C {
		processNewPosts(redditClient, dynalist, documentID, cache, cacheFile)
	}
}

func processNewPosts(
	redditClient *RedditClient,
	dynalistClient *DynalistClient,
	documentID string,
	cache *Cache,
	cacheFile string,
) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	// Get saved posts
	posts, err := redditClient.GetSavedPosts(ctx, 25)
	if err != nil {
		log.Printf("Error fetching saved posts: %v", err)
		return
	}

	// Track how many new posts we found
	newPosts := 0

	// Process each post
	for _, post := range posts {
		// Skip if we've already processed this post
		if _, exists := cache.Posts[post.FullID]; exists {
			continue
		}

		// Add to cache with current timestamp
		cache.Posts[post.FullID] = time.Now()
		
		// Create content for Dynalist
		var content string
		if post.IsComment {
			content = fmt.Sprintf("Comment by %s - https://reddit.com%s", post.Author, post.Permalink)
		} else if post.Title != "" {
			content = fmt.Sprintf("%s - https://reddit.com%s", post.Title, post.Permalink)
		} else {
			content = fmt.Sprintf("Post by %s - https://reddit.com%s", post.Author, post.Permalink)
		}

		log.Printf("Adding new saved post to Dynalist: %s", content)

		// Create item in Dynalist
		err = dynalistClient.CreateItem(documentID, content)
		if err != nil {
			log.Printf("Error creating Dynalist item: %v", err)
			continue
		}

		newPosts++
	}

	// Cleanup cache - remove entries older than 7 days
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
	
	// Save cache to file for persistence between runs
	if err := cache.SaveToFile(cacheFile); err != nil {
		log.Printf("Warning: Failed to save cache: %v", err)
	}
}