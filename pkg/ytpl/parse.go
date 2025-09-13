package ytpl

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

func parseText(textObj interface{}) string {
	if textObj == nil {
		return ""
	}

	switch v := textObj.(type) {
	case string:
		return v
	case map[string]interface{}:
		if simpleText, ok := v["simpleText"].(string); ok {
			return simpleText
		}
		if runs, ok := v["runs"].([]interface{}); ok {
			var result strings.Builder
			for _, run := range runs {
				if runMap, ok := run.(map[string]interface{}); ok {
					if text, ok := runMap["text"].(string); ok {
						result.WriteString(text)
					}
				}
			}
			return result.String()
		}
	}
	return ""
}

func parseNumFromText(textObj interface{}) int {
	text := parseText(textObj)
	if text == "" {
		return 0
	}

	numStr := regexp.MustCompile(`[^\d,.]`).ReplaceAllString(text, "")
	numStr = strings.ReplaceAll(numStr, ",", "")

	if num, err := strconv.Atoi(numStr); err == nil {
		return num
	}
	return 0
}

func parseItem(rawItem interface{}) *PlaylistItem {
	itemMap, ok := rawItem.(map[string]interface{})
	if !ok {
		return nil
	}

	var renderer map[string]interface{}
	for key, value := range itemMap {
		if strings.Contains(key, "VideoRenderer") {
			renderer, _ = value.(map[string]interface{})
			break
		}
	}

	if renderer == nil {
		return nil
	}

	item := &PlaylistItem{}

	if videoID, ok := renderer["videoId"].(string); ok {
		item.ID = videoID
		item.URL = fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	}

	item.Title = parseText(renderer["title"])

	if thumbnails, ok := renderer["thumbnail"].(map[string]interface{}); ok {
		if thumbnailList, ok := thumbnails["thumbnails"].([]interface{}); ok && len(thumbnailList) > 0 {
			if thumb, ok := thumbnailList[0].(map[string]interface{}); ok {
				if url, ok := thumb["url"].(string); ok {
					item.Thumbnail = url
				}
			}
		}
	}

	if lengthText, ok := renderer["lengthText"].(map[string]interface{}); ok {
		item.Duration = parseText(lengthText)
	}

	if ownerText, ok := renderer["ownerText"].(map[string]interface{}); ok {
		item.Author = parseText(ownerText)
	}

	return item
}

func parseBody(body string, opts *Options) (*ParsedResponse, error) {
	parsed := &ParsedResponse{}

	apiKeyStart := strings.Index(body, `"INNERTUBE_API_KEY":"`)
	if apiKeyStart != -1 {
		apiKeyStart += len(`"INNERTUBE_API_KEY":"`)
		apiKeyEnd := strings.Index(body[apiKeyStart:], `"`)
		if apiKeyEnd != -1 {
			parsed.APIKey = body[apiKeyStart : apiKeyStart+apiKeyEnd]
		}
	}

	versionStart := strings.Index(body, `"clientVersion":"`)
	if versionStart != -1 {
		versionStart += len(`"clientVersion":"`)
		versionEnd := strings.Index(body[versionStart:], `"`)
		if versionEnd != -1 {
			parsed.Context.Client.ClientVersion = body[versionStart : versionStart+versionEnd]
			parsed.Context.Client.ClientName = "WEB"
		}
	}

	jsonStart := strings.Index(body, `var ytInitialData = `)
	if jsonStart != -1 {
		jsonStart += len(`var ytInitialData = `)
		jsonEnd := strings.Index(body[jsonStart:], `;</script>`)
		if jsonEnd != -1 {
			jsonStr := body[jsonStart : jsonStart+jsonEnd]
			if err := json.Unmarshal([]byte(jsonStr), &parsed.JSON); err == nil {
				return parsed, nil
			}
		}
	}

	return parsed, nil
}

func parsePage2(apiKey string, token string, context Context, opts *Options) ([]PlaylistItem, error) {
	payload := map[string]interface{}{
		"context":      context,
		"continuation": token,
	}

	jsonResp, err := doPost(BaseAPIURL+apiKey, opts.RequestOptions, payload)
	if err != nil {
		return nil, err
	}

	actions, ok := jsonResp["onResponseReceivedActions"].([]interface{})
	if !ok || len(actions) == 0 {
		return []PlaylistItem{}, nil
	}

	action, ok := actions[0].(map[string]interface{})
	if !ok {
		return []PlaylistItem{}, nil
	}

	appendAction, ok := action["appendContinuationItemsAction"].(map[string]interface{})
	if !ok {
		return []PlaylistItem{}, nil
	}

	wrapper, ok := appendAction["continuationItems"].([]interface{})
	if !ok {
		return []PlaylistItem{}, nil
	}

	var parsedItems []PlaylistItem
	for i, item := range wrapper {
		if i >= opts.Limit {
			break
		}
		if parsedItem := parseItem(item); parsedItem != nil {
			parsedItems = append(parsedItems, *parsedItem)
		}
	}

	opts.Limit -= len(parsedItems)

	var nextToken string
	for _, item := range wrapper {
		if itemMap, ok := item.(map[string]interface{}); ok {
			for key := range itemMap {
				if key == "continuationItemRenderer" {
					nextToken = getContinuationToken(itemMap)
					break
				}
			}
			if nextToken != "" {
				break
			}
		}
	}

	if nextToken == "" || opts.Limit < 1 {
		return parsedItems, nil
	}

	nestedResp, err := parsePage2(apiKey, nextToken, context, opts)
	if err != nil {
		return parsedItems, err
	}

	parsedItems = append(parsedItems, nestedResp...)
	return parsedItems, nil
}
