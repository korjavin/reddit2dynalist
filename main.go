package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/vartanbeno/go-reddit/v2/reddit"
)

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

// DynalistCreateItemRequest represents the request to create an item
type DynalistCreateItemRequest struct {
	Token       string `json:"token"`
	DocumentID  string `json:"file_id"`
	ParentID    string `json:"parent_id,omitempty"`
	Content     string `json:"content"`
	InsertAfter string `json:"insert_after,omitempty"`
}

// Cache stores post IDs to avoid duplicates
type Cache struct {
	Posts map[string]time.Time
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
	req := DynalistCreateItemRequest{
		Token:      d.APIKey,
		DocumentID: documentID,
		Content:    content,
	}

	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/doc/children/add", d.BaseURL)
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
	// Initialize cache
	cache := Cache{
		Posts: make(map[string]time.Time),
	}

	// Load Reddit credentials from environment variables
	credentials := reddit.Credentials{
		ID:       os.Getenv("REDDIT_CLIENT_ID"),
		Secret:   os.Getenv("REDDIT_CLIENT_SECRET"),
		Username: os.Getenv("REDDIT_USERNAME"),
		Password: os.Getenv("REDDIT_PASSWORD"),
	}

	// Validate all required environment variables
	if credentials.ID == "" || credentials.Secret == "" || 
	   credentials.Username == "" || credentials.Password == "" ||
	   os.Getenv("DYNALIST_API_KEY") == "" {
		log.Fatal("Missing required environment variables. Please set REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET, REDDIT_USERNAME, REDDIT_PASSWORD, and DYNALIST_API_KEY")
	}

	// Create Reddit client with appropriate user agent
	client, err := reddit.NewClient(
		credentials,
		reddit.WithUserAgent(fmt.Sprintf("script:reddit2dynalist:v1.0 (by /u/%s)", credentials.Username)),
	)
	if err != nil {
		log.Fatal("Failed to create Reddit client:", err)
	}

	// Verify Reddit authentication
	me, _, err := client.User.Get(context.Background(), credentials.Username)
	if err != nil {
		log.Fatal("Failed to authenticate with Reddit:", err)
	}
	log.Printf("Successfully authenticated as: %s", me.Name)

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
	processNewPosts(client, dynalist, documentID, &cache)

	// Then process on each tick
	for range ticker.C {
		processNewPosts(client, dynalist, documentID, &cache)
	}
}

func processNewPosts(
	redditClient *reddit.Client,
	dynalistClient *DynalistClient,
	documentID string,
	cache *Cache,
) {
	ctx := context.Background()
	
	// Get saved posts sorted by new
	saved, _, _, err := redditClient.User.Saved(ctx, &reddit.ListUserOverviewOptions{
		ListOptions: reddit.ListOptions{
			Limit: 25,
		},
		Sort: "new",
		Time: "all", // Get all saved posts, we'll filter with our cache
	})
	
	if err != nil {
		log.Printf("Error fetching saved posts: %v", err)
		return
	}

	// Track how many new posts we found
	newPosts := 0

	// Process each post
	for _, post := range saved {
		// Skip if we've already processed this post
		if _, exists := cache.Posts[post.FullID]; exists {
			continue
		}

		// Add to cache with current timestamp
		cache.Posts[post.FullID] = time.Now()
		
		// Create content for Dynalist
		var content string
		if post.Title != "" {
			content = post.Title + " - https://reddit.com" + post.Permalink
		} else {
			// Handle comments which may not have a title
			content = "Comment by " + post.Author + " - https://reddit.com" + post.Permalink
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
}