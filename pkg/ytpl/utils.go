package ytpl

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func logger(content string) {
	dir := filepath.Join(".", "dumps")
	if err := os.MkdirAll(dir, 0755); err != nil {
		log.Printf("Failed to create dumps directory: %v", err)
		return
	}

	filename := fmt.Sprintf("%s-%d.txt",
		strconv.FormatInt(rand.Int63(), 36)[3:],
		time.Now().Unix())
	filepath := filepath.Join(dir, filename)

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		log.Printf("Failed to write debug file: %v", err)
		return
	}

	log.Printf("\n/%s", strings.Repeat("*", 200))
	log.Printf("Unsupported YouTube Playlist response.")
	log.Printf("Please post the files in %s to DisTube support server. Thanks!", dir)
	log.Printf("%s\\", strings.Repeat("*", 200))
}

func getContinuationToken(item map[string]interface{}) string {
	if item == nil {
		return ""
	}

	continuationItemRenderer, ok := item["continuationItemRenderer"]
	if !ok {
		return ""
	}

	renderer, ok := continuationItemRenderer.(map[string]interface{})
	if !ok {
		return ""
	}

	if continuationEndpoint, ok := renderer["continuationEndpoint"].(map[string]interface{}); ok {
		if continuationCommand, ok := continuationEndpoint["continuationCommand"].(map[string]interface{}); ok {
			if token, ok := continuationCommand["token"].(string); ok {
				return token
			}
		}
	}

	if button, ok := renderer["button"].(map[string]interface{}); ok {
		if buttonRenderer, ok := button["buttonRenderer"].(map[string]interface{}); ok {
			if command, ok := buttonRenderer["command"].(map[string]interface{}); ok {
				if continuationCommand, ok := command["continuationCommand"].(map[string]interface{}); ok {
					if token, ok := continuationCommand["token"].(string); ok {
						return token
					}
				}
			}
		}
	}

	if trigger, ok := renderer["trigger"].(map[string]interface{}); ok {
		if continuationCommand, ok := trigger["continuationCommand"].(map[string]interface{}); ok {
			if token, ok := continuationCommand["token"].(string); ok {
				return token
			}
		}
	}

	token := findTokenRecursively(renderer)
	if token != "" {
		return token
	}

	return ""
}

func findTokenRecursively(obj interface{}) string {
	switch v := obj.(type) {
	case map[string]interface{}:
		if continuationCommand, ok := v["continuationCommand"].(map[string]interface{}); ok {
			if token, ok := continuationCommand["token"].(string); ok {
				return token
			}
		}
		for _, value := range v {
			if result := findTokenRecursively(value); result != "" {
				return result
			}
		}
	case []interface{}:
		for _, item := range v {
			if result := findTokenRecursively(item); result != "" {
				return result
			}
		}
	}
	return ""
}

func doPost(url string, client *http.Client, payload interface{}) (map[string]interface{}, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := client.Post(url, "application/json", strings.NewReader(string(jsonData)))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return result, nil
}
