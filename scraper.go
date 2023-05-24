package main

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/huynchu/go-rssagg/internal/database"
)

func startScraping(db *database.Queries, 
	                 concurrency int, 
									 timeBetweenRequest time.Duration,
) {
		log.Printf("Scraping on %v goroutines every %s duration", concurrency, timeBetweenRequest)
		ticker := time.NewTicker(timeBetweenRequest)
		for ; ;<-ticker.C {
			feeds, err := db.GetNextFeedsToFetch(context.Background(), int32(concurrency))
			if err != nil {
				log.Println("error fetching feeds:", err)
				continue
			}

			wg := sync.WaitGroup{}
			for _, feed := range feeds {
				wg.Add(1)
				go scrapeFeed(db, &wg, feed)
			}
			wg.Wait()
		}
 }

 func scrapeFeed(db *database.Queries, wg *sync.WaitGroup, feed database.Feed) {
	defer wg.Done()
	_, err := db.MarkFeedAsFetched(context.Background(), feed.ID)
	if err != nil {
		log.Printf("Couldn't mark feed %s fetched: %v", feed.Name, err)
		return
	}

	feedData, err := urlToFeed(feed.Url)
	if err != nil {
		log.Printf("Couldn't collect feed %s: %v", feed.Name, err)
		return
	}
	for _, item := range feedData.Channel.Item {
		description := sql.NullString{}
		if item.Description != "" {
			description.String = item.Description
			description.Valid = true
		}
		pubAt, err := time.Parse(time.RFC1123Z, item.PubDate)
		if err != nil {
			log.Printf("Couldn't parse date %v with err %v", item.PubDate, err)
			continue
		}

		_ , err = db.CreatePost(context.Background(), database.CreatePostParams{
			ID: uuid.New(),
			CreatedAt: time.Now().UTC(),
			UpdatedAt: time.Now().UTC(),
			Title: item.Title,
			Description: description,
			PublishedAt: pubAt,
			Url: item.Link,
			FeedID: feed.ID,
		})
		if err != nil {
			log.Println("Failed to create post:", err)
		}
	}
	log.Printf("Feed %s collected, %v posts found", feed.Name, len(feedData.Channel.Item))
}