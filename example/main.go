package main

import (
	"fmt"
	"log"
	"ytpl-ytsr-go/pkg/ytpl"
	"ytpl-ytsr-go/pkg/ytsr"
)

const playlistURL string = "https://www.youtube.com/playlist?list=PLpas7-jjCh0Z7-t9yqhg3ibXTgeZhtH-G"
const playlistID string = "PLpas7-jjCh0Z7-t9yqhg3ibXTgeZhtH-G"
const channelURL string = "https://www.youtube.com/channel/UCXuqSBlHAE6Xw-yeJA0Tunw"

func main() {
	fmt.Println("====================================")
	fmt.Println("        YTPL TESTS")
	fmt.Println("====================================")

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
		Limit: 10,
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

	fmt.Println("\n====================================")
	fmt.Println("        YTSR TESTS")
	fmt.Println("====================================")

	fmt.Println("=== Example 1: Basic Video Search ===")
	searchResults, err := ytsr.Search("golang tutorial", nil)
	if err != nil {
		log.Printf("Error searching videos: %v", err)
	} else {
		fmt.Printf("Search Query: %s\n", searchResults.Query)
		fmt.Printf("Total Results: %d\n", searchResults.Results)
		fmt.Printf("Found %d items\n", len(searchResults.Items))
		fmt.Println()

		for i, item := range searchResults.Items[:5] {
			fmt.Printf("%d. %s\n", i+1, item.Name)
			fmt.Printf("   ID: %s\n", item.ID)
			fmt.Printf("   URL: %s\n", item.URL)
			fmt.Printf("   Duration: %s\n", item.Duration)
			fmt.Printf("   Description: %s\n", truncateString(item.Description, 80))
			if item.Author != nil {
				fmt.Printf("   Author: %s\n", item.Author.Name)
				fmt.Printf("   Channel ID: %s\n", item.Author.ChannelID)
				fmt.Printf("   Verified: %t\n", item.Author.Verified)
			}
			if item.Views != nil {
				fmt.Printf("   Views: %d\n", *item.Views)
			}
			fmt.Printf("   Uploaded: %s\n", item.UploadedAt)
			fmt.Printf("   Is Live: %t\n", item.IsLive)
			fmt.Println()
		}
	}

	fmt.Println("=== Example 2: Limited Video Search ===")
	searchOptions := &ytsr.Options{
		Type:  "video",
		Limit: 3,
		GL:    "US",
		HL:    "en",
	}

	searchResults2, err := ytsr.Search("react js", searchOptions)
	if err != nil {
		log.Printf("Error searching with options: %v", err)
	} else {
		fmt.Printf("Retrieved %d items (limited to %d):\n", len(searchResults2.Items), searchOptions.Limit)
		for i, item := range searchResults2.Items {
			fmt.Printf("%d. %s\n", i+1, item.Name)
			fmt.Printf("   Duration: %s\n", item.Duration)
			if item.Author != nil {
				fmt.Printf("   Channel: %s\n", item.Author.Name)
			}
			fmt.Printf("   URL: %s\n", item.URL)
			fmt.Println()
		}
	}

	fmt.Println("=== Example 3: Playlist Search ===")
	playlistSearchOptions := &ytsr.Options{
		Type:  "playlist",
		Limit: 5,
	}

	playlistResults, err := ytsr.Search("programming tutorials", playlistSearchOptions)
	if err != nil {
		log.Printf("Error searching playlists: %v", err)
	} else {
		fmt.Printf("Found %d playlist results:\n", len(playlistResults.Items))
		for i, item := range playlistResults.Items {
			fmt.Printf("%d. %s\n", i+1, item.Name)
			fmt.Printf("   ID: %s\n", item.ID)
			fmt.Printf("   URL: %s\n", item.URL)
			fmt.Printf("   Video Count: %d\n", item.Length)
			if item.Owner != nil {
				fmt.Printf("   Owner: %s\n", item.Owner.Name)
				fmt.Printf("   Owner Verified: %t\n", item.Owner.Verified)
			}
			fmt.Printf("   Published: %s\n", item.PublishedAt)
			fmt.Println()
		}
	}

	fmt.Println("=== Example 4: Safe Search ===")
	safeSearchOptions := &ytsr.Options{
		Type:       "video",
		Limit:      3,
		SafeSearch: true,
	}

	safeResults, err := ytsr.Search("family friendly content", safeSearchOptions)
	if err != nil {
		log.Printf("Error with safe search: %v", err)
	} else {
		fmt.Printf("Safe search results (%d items):\n", len(safeResults.Items))
		for i, item := range safeResults.Items {
			fmt.Printf("%d. %s\n", i+1, item.Name)
			if item.Author != nil {
				fmt.Printf("   Channel: %s\n", item.Author.Name)
			}
			fmt.Println()
		}
	}

	fmt.Println("=== Example 5: Default Options Test ===")
	defaultOpts := ytsr.DefaultOptions()
	fmt.Printf("Default options:\n")
	fmt.Printf("   Type: %s\n", defaultOpts.Type)
	fmt.Printf("   Limit: %d\n", defaultOpts.Limit)
	fmt.Printf("   SafeSearch: %t\n", defaultOpts.SafeSearch)
	fmt.Printf("   GL: %s\n", defaultOpts.GL)
	fmt.Printf("   HL: %s\n", defaultOpts.HL)
	fmt.Printf("   UTCOffset: %d\n", defaultOpts.UTCOffset)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
