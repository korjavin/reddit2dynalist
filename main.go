package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/turnage/graw/reddit"
)

type DynalistClient struct {
	apiKey  string
	baseURL string
}

func (d *DynalistClient) CreateItem(docID string, content string) error {
	// TODO: Implement Dynalist API call to create item
	return nil
}

func main() {
	// Get Reddit credentials from environment variables
	clientID := os.Getenv("REDDIT_CLIENT_ID")
	clientSecret := os.Getenv("REDDIT_CLIENT_SECRET")
	username := os.Getenv("REDDIT_USERNAME")
	password := os.Getenv("REDDIT_PASSWORD")

	// Check if environment variables are set
	if clientID == "" || clientSecret == "" || username == "" || password == "" {
		log.Fatal("Missing Reddit credentials. Please set REDDIT_CLIENT_ID, REDDIT_CLIENT_SECRET, REDDIT_USERNAME, and REDDIT_PASSWORD environment variables.")
	}

	// Format user agent according to Reddit's guidelines: platform:app-id:version (by /u/username)
	userAgent := "script:sync3dynalist:v1.0 (by /u/" + username + ")"

	// Print debug information (excluding password)
	log.Printf("Using Reddit credentials - Client ID: %s, Username: %s, User Agent: %s",
		clientID, username, userAgent)

	// Create Reddit bot configuration
	botConfig := reddit.BotConfig{
		Agent: userAgent,
		App: reddit.App{
			ID:       clientID,
			Secret:   clientSecret,
			Username: username,
			Password: password,
		},
		Rate: 5 * time.Second, // Rate limit to 5 seconds between requests
	}

	// Create Reddit client
	client, err := reddit.NewBot(botConfig)
	if err != nil {
		log.Fatal("Failed to create Reddit client:", err)
	}

	// Test Reddit API connectivity by fetching a subreddit
	log.Println("Testing Reddit API connectivity...")
	harvest, err := client.Listing("/r/golang", "")
	if err != nil {
		log.Printf("Failed to access Reddit API (public endpoint): %v", err)
		log.Println("This may indicate network issues or Reddit API changes")
	} else if len(harvest.Posts) > 0 {
		log.Printf("Successfully connected to Reddit API, got subreddit: %s", harvest.Posts[0].Subreddit)
	} else {
		log.Println("Successfully connected to Reddit API, but no posts were returned")
	}

	dynalistAPIKey := os.Getenv("DYNALIST_API_KEY")
	if dynalistAPIKey == "" {
		log.Fatal("Missing Dynalist API key. Please set DYNALIST_API_KEY environment variable.")
	}

	dynalist := &DynalistClient{
		apiKey:  dynalistAPIKey,
		baseURL: "https://dynalist.io/api/v1",
	}

	// Set up signal handling for graceful shutdown
	done := make(chan bool)
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigCh
		log.Println("Received shutdown signal, gracefully shutting down...")
		done <- true
	}()

	log.Println("Starting Reddit to Dynalist sync service...")
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			log.Println("Shutting down...")
			return
		case <-ticker.C:
			log.Println("Checking for new saved posts...")

			// Fetch saved posts using the Listing method
			// The path for saved posts is /user/{username}/saved
			savedPath := "/user/" + username + "/saved"
			params := map[string]string{
				"sort": "new",
				"t":    "day",
			}

			harvest, err := client.ListingWithParams(savedPath, params)
			if err != nil {
				log.Printf("Error fetching saved posts: %v", err)
				continue
			}

			if len(harvest.Posts) == 0 {
				log.Println("No new saved posts found")
				continue
			}

			log.Printf("Found %d saved posts", len(harvest.Posts))
			for _, post := range harvest.Posts {
				log.Printf("Processing saved post: %s", post.Title)

				content := post.Title + " - https://reddit.com" + post.Permalink
				err = dynalist.CreateItem("Reddit", content)
				if err != nil {
					log.Printf("Error creating Dynalist item: %v", err)
				} else {
					log.Printf("Successfully added post to Dynalist: %s", post.Title)
				}
			}
		}
	}
}
