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

	r, _ := item["continuationItemRenderer"].(map[string]interface{})
	if r == nil {
		return ""
	}

	endpoint, _ := r["continuationEndpoint"].(map[string]interface{})
	if endpoint != nil {
		cmd, _ := endpoint["continuationCommand"].(map[string]interface{})
		if cmd != nil {
			token, _ := cmd["token"].(string)
			if token != "" {
				return token
			}
		}
	}

	button, _ := r["button"].(map[string]interface{})
	br, _ := button["buttonRenderer"].(map[string]interface{})
	cmd, _ := br["command"].(map[string]interface{})
	cc, _ := cmd["continuationCommand"].(map[string]interface{})
	token, _ := cc["token"].(string)
	if token != "" {
		return token
	}

	trigger, _ := r["trigger"].(map[string]interface{})
	cc2, _ := trigger["continuationCommand"].(map[string]interface{})
	token2, _ := cc2["token"].(string)
	if token2 != "" {
		return token2
	}

	token3 := findTokenRecursively(r)
	if token3 != "" {
		return token3
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
