package main

import (
	"context"
	"log"
	"os"
	"time"

	"github.com/vartanbeno/go-reddit/v2/reddit"
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
	credentials := reddit.Credentials{
		ID:       os.Getenv("REDDIT_CLIENT_ID"),
		Secret:   os.Getenv("REDDIT_CLIENT_SECRET"),
		Username: os.Getenv("REDDIT_USERNAME"),
		Password: os.Getenv("REDDIT_PASSWORD"),
	}

	redditClient, err := reddit.NewClient(credentials)
	if err != nil {
		log.Fatal("Failed to create Reddit client:", err)
	}

	dynalist := &DynalistClient{
		apiKey:  os.Getenv("DYNALIST_API_KEY"),
		baseURL: "https://dynalist.io/api/v1",
	}

	ticker := time.NewTicker(5 * time.Second)
	for range ticker.C {
		saved, _, _, err := redditClient.User.Saved(context.Background(), &reddit.ListUserOverviewOptions{Sort: "new", Time: "day"})
		if err != nil {
			log.Printf("Error fetching saved posts: %v", err)
			continue
		}

		for _, post := range saved {
			log.Printf("Saved post: %v", post)

			content := post.Title + " - https://reddit.com" + post.Permalink
			err = dynalist.CreateItem("Reddit", content)
			if err != nil {
				log.Printf("Error creating Dynalist item: %v", err)
			}
		}
	}
}
