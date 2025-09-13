package ytsr

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

func parseBody(body string, opts *Options) (*ParsedData, error) {
	patterns := []string{
		`var ytInitialData = (.+?)};`,
		`window\["ytInitialData"\] = (.+?)};`,
		`var ytInitialData = (.+?);</script>`,
		`window\["ytInitialData"\] = (.+?);</script>`,
	}

	var jsonData map[string]interface{}
	var jsonStr string

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		match := re.FindStringSubmatch(body)
		if len(match) > 1 {
			jsonStr = match[1]
			if strings.HasSuffix(pattern, "};") {
				jsonStr += "}"
			}
			err := json.Unmarshal([]byte(jsonStr), &jsonData)
			if err == nil {
				break
			}
		}
	}

	if jsonData == nil {
		return nil, fmt.Errorf("could not extract JSON data")
	}

	clientVersion, _ := extractClientVersion(jsonData, body)
	context := buildPostContext(clientVersion, opts)

	return &ParsedData{
		JSON:    jsonData,
		Context: context,
		Body:    body,
	}, nil
}

func parseResponse(parsed *ParsedData, opts *Options) (*SearchResult, error) {
	result := &SearchResult{
		Query: opts.Query,
		Items: []SearchItem{},
	}

	var twoCol map[string]interface{}
	if contents, ok := parsed.JSON["contents"].(map[string]interface{}); ok {
		if tc, ok := contents["twoColumnSearchResultsRenderer"].(map[string]interface{}); ok {
			twoCol = tc
		}
	}
	if twoCol == nil {
		if tc, ok := findTwoColumnSearchResultsRenderer(parsed.JSON); ok {
			twoCol = tc
		}
	}

	if twoCol == nil {
		if contents, ok := parsed.JSON["contents"].(map[string]interface{}); ok {
			if sectionList, ok := contents["sectionListRenderer"].(map[string]interface{}); ok {
				twoCol = map[string]interface{}{
					"primaryContents": map[string]interface{}{
						"sectionListRenderer": sectionList,
					},
				}
			}
		}
	}

	if twoCol == nil {
		return nil, fmt.Errorf("invalid response format")
	}

	primaryContents, ok := twoCol["primaryContents"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response format")
	}

	rawItems, _ := parseWrapper(primaryContents)

	for i, item := range rawItems {
		if i >= opts.Limit {
			break
		}

		parsedItem := parseItem(item)
		if parsedItem != nil && parsedItem.Type == opts.Type {
			result.Items = append(result.Items, *parsedItem)
		}
	}

	if estimatedResults, ok := parsed.JSON["estimatedResults"]; ok {
		if results, ok := estimatedResults.(string); ok {
			if num, err := strconv.Atoi(results); err == nil {
				result.Results = num
			}
		}
	}

	return result, nil
}

func parseWrapper(primaryContents map[string]interface{}) ([]interface{}, interface{}) {
	var rawItems []interface{}
	var continuation interface{}

	if sectionList, ok := primaryContents["sectionListRenderer"].(map[string]interface{}); ok {
		if contents, ok := sectionList["contents"].([]interface{}); ok {
			for _, content := range contents {
				if contentMap, ok := content.(map[string]interface{}); ok {
					if itemSection, ok := contentMap["itemSectionRenderer"].(map[string]interface{}); ok {
						if items, ok := itemSection["contents"].([]interface{}); ok {
							rawItems = items
							break
						}
					}
					if _, ok := contentMap["continuationItemRenderer"]; ok {
						continuation = content
					}
				}
			}
		}
	}

	return rawItems, continuation
}

func parseItem(item interface{}) *SearchItem {
	itemMap, ok := item.(map[string]interface{})
	if !ok {
		return nil
	}

	for key, value := range itemMap {
		switch key {
		case "videoRenderer":
			return parseVideo(value.(map[string]interface{}))
		case "playlistRenderer":
			return parsePlaylist(value.(map[string]interface{}))
		case "gridVideoRenderer":
			return parseVideo(value.(map[string]interface{}))
		case "channelRenderer":
			return nil
		case "lockupViewModel":
			return parseLockupViewModel(value.(map[string]interface{}))
		case "gridShelfViewModel":
			return nil
		}
	}

	return nil
}

func parseLockupViewModel(obj map[string]interface{}) *SearchItem {
	if contentType, ok := obj["contentType"].(string); ok && contentType == "LOCKUP_CONTENT_TYPE_PLAYLIST" {
		item := &SearchItem{
			Type: "playlist",
		}

		if contentId, ok := obj["contentId"].(string); ok {
			item.ID = contentId
			item.URL = "https://www.youtube.com/playlist?list=" + contentId
		}

		if metadata, ok := obj["metadata"].(map[string]interface{}); ok {
			if lockupMetadata, ok := metadata["lockupMetadataViewModel"].(map[string]interface{}); ok {
				if title, ok := lockupMetadata["title"]; ok {
					item.Name = parseText(title)
				}
			}
		}

		return item
	}

	return nil
}

func parseVideo(obj map[string]interface{}) *SearchItem {
	item := &SearchItem{
		Type: "video",
	}

	if videoId, ok := obj["videoId"].(string); ok {
		item.ID = videoId
		item.URL = BaseVideoURL + videoId
	}

	if title, ok := obj["title"]; ok {
		item.Name = parseText(title)
	}

	if thumbnail, ok := obj["thumbnail"].(map[string]interface{}); ok {
		if thumbnails, ok := thumbnail["thumbnails"].([]interface{}); ok {
			item.Thumbnails = prepareThumbnails(thumbnails)
			if len(item.Thumbnails) > 0 {
				item.Thumbnail = item.Thumbnails[0].URL
			}
		}
	}

	if desc, ok := obj["descriptionSnippet"]; ok {
		item.Description = parseText(desc)
	} else if detailedSnippets, ok := obj["detailedMetadataSnippets"].([]interface{}); ok && len(detailedSnippets) > 0 {
		if snippet, ok := detailedSnippets[0].(map[string]interface{}); ok {
			if snippetText, ok := snippet["snippetText"]; ok {
				item.Description = parseText(snippetText)
			}
		}
	} else if richSnippet, ok := obj["richSnippet"]; ok {
		if snippetText, ok := richSnippet.(map[string]interface{})["snippetText"]; ok {
			item.Description = parseText(snippetText)
		}
	}

	if viewCount, ok := obj["viewCountText"]; ok {
		if views := parseIntegerFromText(viewCount); views > 0 {
			item.Views = &views
		}
	}

	if lengthText, ok := obj["lengthText"]; ok {
		item.Duration = parseText(lengthText)
	}

	if publishedTime, ok := obj["publishedTimeText"]; ok {
		item.UploadedAt = parseText(publishedTime)
	}

	item.Author = parseAuthor(obj)

	if badges, ok := obj["badges"].([]interface{}); ok {
		for _, badge := range badges {
			if badgeMap, ok := badge.(map[string]interface{}); ok {
				if renderer, ok := badgeMap["metadataBadgeRenderer"].(map[string]interface{}); ok {
					if label, ok := renderer["label"].(string); ok {
						item.Badges = append(item.Badges, label)
					}
				}
			}
		}
	}

	for _, badge := range item.Badges {
		if badge == "LIVE NOW" || badge == "LIVE" {
			item.IsLive = true
			break
		}
	}

	return item
}

func parsePlaylist(obj map[string]interface{}) *SearchItem {
	item := &SearchItem{
		Type: "playlist",
	}

	if playlistId, ok := obj["playlistId"].(string); ok {
		item.ID = playlistId
		item.URL = "https://www.youtube.com/playlist?list=" + playlistId
	}

	if title, ok := obj["title"]; ok {
		item.Name = parseText(title)
	}

	item.Owner = parseOwner(obj)

	return item
}

func parseAuthor(obj map[string]interface{}) *Author {
	if ownerText, ok := obj["ownerText"].(map[string]interface{}); ok {
		if runs, ok := ownerText["runs"].([]interface{}); ok && len(runs) > 0 {
			if run, ok := runs[0].(map[string]interface{}); ok {
				author := &Author{}

				if text, ok := run["text"].(string); ok {
					author.Name = text
				}

				if navEndpoint, ok := run["navigationEndpoint"].(map[string]interface{}); ok {
					if browseEndpoint, ok := navEndpoint["browseEndpoint"].(map[string]interface{}); ok {
						if browseId, ok := browseEndpoint["browseId"].(string); ok {
							author.ChannelID = browseId
						}
						if canonicalUrl, ok := browseEndpoint["canonicalBaseUrl"].(string); ok {
							if u, err := url.Parse(BaseURL); err == nil {
								if fullUrl, err := u.Parse(canonicalUrl); err == nil {
									author.URL = fullUrl.String()
								}
							}
						}
					}
				}

				if ctsr, ok := obj["channelThumbnailSupportedRenderers"].(map[string]interface{}); ok {
					if renderer, ok := ctsr["channelThumbnailWithLinkRenderer"].(map[string]interface{}); ok {
						if thumbnail, ok := renderer["thumbnail"].(map[string]interface{}); ok {
							if thumbnails, ok := thumbnail["thumbnails"].([]interface{}); ok {
								author.Avatars = prepareThumbnails(thumbnails)
								if len(author.Avatars) > 0 {
									author.BestAvatar = &author.Avatars[0]
								}
							}
						}
					}
				}

				if ownerBadges, ok := obj["ownerBadges"].([]interface{}); ok {
					for _, badge := range ownerBadges {
						if badgeMap, ok := badge.(map[string]interface{}); ok {
							if renderer, ok := badgeMap["metadataBadgeRenderer"].(map[string]interface{}); ok {
								if tooltip, ok := renderer["tooltip"].(string); ok {
									author.Badges = append(author.Badges, tooltip)
									if strings.Contains(tooltip, "VERIFIED") || strings.Contains(tooltip, "OFFICIAL") {
										author.Verified = true
									}
								}
							}
						}
					}
				}

				return author
			}
		}
	}
	return nil
}

func parseOwner(obj map[string]interface{}) *Owner {
	var ownerRuns []interface{}

	if shortByline, ok := obj["shortBylineText"].(map[string]interface{}); ok {
		if runs, ok := shortByline["runs"].([]interface{}); ok {
			ownerRuns = runs
		}
	} else if longByline, ok := obj["longBylineText"].(map[string]interface{}); ok {
		if runs, ok := longByline["runs"].([]interface{}); ok {
			ownerRuns = runs
		}
	}

	if len(ownerRuns) <= 0 {
		return nil
	}

	if run, ok := ownerRuns[0].(map[string]interface{}); ok {
		owner := &Owner{}

		if text, ok := run["text"].(string); ok {
			owner.Name = text
		}

		if navEndpoint, ok := run["navigationEndpoint"].(map[string]interface{}); ok {
			if browseEndpoint, ok := navEndpoint["browseEndpoint"].(map[string]interface{}); ok {
				if browseId, ok := browseEndpoint["browseId"].(string); ok {
					owner.ChannelID = browseId
				}
				if canonicalUrl, ok := browseEndpoint["canonicalBaseUrl"].(string); ok {
					if u, err := url.Parse(BaseURL); err == nil {
						if fullUrl, err := u.Parse(canonicalUrl); err == nil {
							owner.URL = fullUrl.String()
						}
					}
				}
			}
		}

		if ownerBadges, ok := obj["ownerBadges"].([]interface{}); ok {
			for _, badge := range ownerBadges {
				if badgeMap, ok := badge.(map[string]interface{}); ok {
					if renderer, ok := badgeMap["metadataBadgeRenderer"].(map[string]interface{}); ok {
						if tooltip, ok := renderer["tooltip"].(string); ok {
							owner.Badges = append(owner.Badges, tooltip)
							if strings.Contains(strings.ToUpper(tooltip), "VERIFIED") ||
								strings.Contains(strings.ToUpper(tooltip), "OFFICIAL") ||
								strings.Contains(strings.ToUpper(tooltip), "ARTIST") {
								owner.Verified = true
							}
						}
					} else if renderer, ok := badgeMap["metadataBadgeRenderer"].(map[string]interface{}); ok {
						if style, ok := renderer["style"].(string); ok {
							if style == "BADGE_STYLE_TYPE_VERIFIED" || style == "BADGE_STYLE_TYPE_VERIFIED_ARTIST" {
								owner.Verified = true
							}
						}
					}
				}
			}
		}

		return owner
	}

	return nil
}

func parseText(text interface{}) string {
	if text == nil {
		return ""
	}

	switch t := text.(type) {
	case string:
		return t
	case map[string]interface{}:
		if content, ok := t["content"].(string); ok {
			return content
		}
		if simpleText, ok := t["simpleText"].(string); ok {
			return simpleText
		}
		if runs, ok := t["runs"].([]interface{}); ok {
			var result strings.Builder
			for _, run := range runs {
				if runMap, ok := run.(map[string]interface{}); ok {
					if runText, ok := runMap["text"].(string); ok {
						result.WriteString(runText)
					}
				}
			}
			return result.String()
		}
	}
	return ""
}

func parseIntegerFromText(text interface{}) int {
	textStr := parseText(text)
	if textStr == "" {
		return 0
	}

	re := regexp.MustCompile(`\D+`)
	numStr := re.ReplaceAllString(textStr, "")

	if num, err := strconv.Atoi(numStr); err == nil {
		return num
	}
	return 0
}
