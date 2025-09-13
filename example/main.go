package main

import (
	"fmt"
	"log"

	"ytpl-ytsr-go/pkg/ytpl"
)

const playlistURL string = "https://www.youtube.com/playlist?list=PLpas7-jjCh0Z7-t9yqhg3ibXTgeZhtH-G"
const playlistID string = "PLpas7-jjCh0Z7-t9yqhg3ibXTgeZhtH-G"
const channelURL string = "https://www.youtube.com/channel/UCXuqSBlHAE6Xw-yeJA0Tunw"

func main() {
	fmt.Println("=== Example 1: Basic Playlist Parsing ===")

	playlist, err := ytpl.GetPlaylist(playlistURL, nil)
	if err != nil {
		log.Fatalf("Error parsing playlist: %v", err)
	}

	fmt.Printf("Playlist: %s\n", playlist.Title)
	fmt.Printf("Description: %s\n", playlist.Description)
	fmt.Printf("Total Items: %d\n", playlist.TotalItems)
	fmt.Printf("Views: %d\n", playlist.Views)
	fmt.Printf("Found %d items\n", len(playlist.Items))
	fmt.Println()

	fmt.Println("=== Example 2: Limited Playlist Parsing ===")
	options := &ytpl.Options{
		Limit: 10, // Limit to 10 items
	}

	playlist2, err := ytpl.GetPlaylist(playlistURL, options)
	if err != nil {
		log.Fatalf("Error parsing playlist with options: %v", err)
	}

	fmt.Printf("Retrieved %d items (limited to %d):\n", len(playlist2.Items), options.Limit)
	for i, item := range playlist2.Items {
		fmt.Printf("%d. %s\n", i+1, item.Title)
		fmt.Printf("   ID: %s\n", item.ID)
		fmt.Printf("   URL: %s\n", item.URL)
		fmt.Printf("   Duration: %s\n", item.Duration)
		fmt.Printf("   Author: %s\n", item.Author)
		fmt.Println()
	}

	fmt.Println("=== Example 3: ID Validation ===")
	testURLs := []string{
		playlistID,
		playlistURL,
		channelURL,
		"invalid-id",
	}

	for _, testURL := range testURLs {
		valid := ytpl.ValidateID(testURL)
		fmt.Printf("'%s' is valid: %t\n", testURL, valid)
	}
	fmt.Println()

	fmt.Println("=== Example 4: Playlist ID Extraction ===")
	testIDs := []string{
		playlistID,
		playlistURL,
		channelURL,
	}

	for _, testID := range testIDs {
		extractedID, err := ytpl.GetPlaylistID(testID)
		if err != nil {
			fmt.Printf("'%s' -> Error: %v\n", testID, err)
		} else {
			fmt.Printf("'%s' -> '%s'\n", testID, extractedID)
		}
	}
}
