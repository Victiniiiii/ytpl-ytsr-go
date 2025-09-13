package ytsr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	BaseSearchURL = "https://www.youtube.com/results"
	BaseAPIURL    = "https://www.youtube.com/youtubei/v1/search"
	BaseVideoURL  = "https://www.youtube.com/watch?v="
	BaseURL       = "https://www.youtube.com/"
	ConsentCookie = "SOCS=CAI"
)

var cache = &Cache{
	ClientVersion:  "2.20240606.06.00",
	PlaylistParams: "EgIQAw%3D%3D",
}

func DefaultOptions() *Options {
	return &Options{
		Type:       "video",
		Limit:      10,
		SafeSearch: false,
		GL:         "US",
		HL:         "en",
		UTCOffset:  -300,
	}
}

func Search(searchString string, options *Options) (*SearchResult, error) {
	return search(searchString, options, 3)
}

func search(searchString string, options *Options, retries int) (*SearchResult, error) {
	if retries == 2 {
		cache.mu.Lock()
		cache.ClientVersion = ""
		cache.PlaylistParams = ""
		cache.mu.Unlock()
	}

	if retries == 0 {
		return nil, fmt.Errorf("unable to find JSON")
	}

	opts := checkArgs(searchString, options)

	var parsed *ParsedData
	var err error

	cache.mu.RLock()
	needsInitialRequest := !opts.SafeSearch || cache.ClientVersion == "" || cache.PlaylistParams == ""
	cache.mu.RUnlock()

	if needsInitialRequest {
		parsed, err = getInitialData(opts)
		if err != nil {
			return nil, err
		}
		saveCache(parsed, opts)
	} else {
		parsed = &ParsedData{
			Context: buildPostContext(cache.ClientVersion, opts),
		}
	}

	if opts.Type == "playlist" {
		parsed.JSON, err = doPost(BaseAPIURL, opts, map[string]interface{}{
			"context": parsed.Context,
			"query":   searchString,
		})
		if err != nil {
			return nil, fmt.Errorf("cannot search for playlist: %v", err)
		}
	} else if opts.SafeSearch || parsed.JSON == nil {
		parsed.JSON, err = doPost(BaseAPIURL, opts, map[string]interface{}{
			"context": parsed.Context,
			"query":   searchString,
		})
		if err != nil && retries == 1 {
			return nil, err
		}
	}

	if parsed.JSON == nil {
		return search(searchString, options, retries-1)
	}

	return parseResponse(parsed, opts)
}

func checkArgs(searchString string, options *Options) *Options {
	if searchString == "" {
		panic("search string is mandatory")
	}

	if options == nil {
		options = DefaultOptions()
	}

	opts := *options

	opts.Query = searchString

	if opts.Type != "video" && opts.Type != "playlist" {
		opts.Type = "video"
	}

	if opts.Limit <= 0 {
		opts.Limit = 10
	}

	if opts.GL == "" {
		opts.GL = "US"
	}

	if opts.HL == "" {
		opts.HL = "en"
	}

	if strings.HasPrefix(searchString, BaseURL) {
		u, err := url.Parse(searchString)
		if err == nil && u.Path == "/results" && u.Query().Get("sp") != "" {
			if u.Query().Get("search_query") == "" {
				panic("filter links have to include a 'search_query' query")
			}
		}
	}

	return &opts
}

func getInitialData(opts *Options) (*ParsedData, error) {
	client := &http.Client{}

	params := url.Values{}
	params.Set("search_query", opts.Query)
	params.Set("gl", opts.GL)
	params.Set("hl", opts.HL)

	req, err := http.NewRequest("GET", BaseSearchURL+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Cookie", ConsentCookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return parseBody(string(body), opts)
}

func extractClientVersion(jsonData map[string]interface{}, body string) (string, error) {
	fallbackVersion := "2.20240606.06.00"

	if respCtx, ok := jsonData["responseContext"].(map[string]interface{}); ok {
		if stp, ok := respCtx["serviceTrackingParams"].([]interface{}); ok {
			for _, service := range stp {
				if serviceMap, ok := service.(map[string]interface{}); ok {
					if params, ok := serviceMap["params"].([]interface{}); ok {
						for _, param := range params {
							if paramMap, ok := param.(map[string]interface{}); ok {
								if key, ok := paramMap["key"].(string); ok && key == "cver" {
									if value, ok := paramMap["value"].(string); ok {
										return value, nil
									}
								}
							}
						}
					}
				}
			}
		}
	}

	patterns := []string{
		`INNERTUBE_CONTEXT_CLIENT_VERSION":"([^"]+)"`,
		`innertube_context_client_version":"([^"]+)"`,
	}

	for _, pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err == nil {
			match := re.FindStringSubmatch(body)
			if len(match) > 1 {
				return match[1], nil
			}
		}
	}

	return fallbackVersion, nil
}

func buildPostContext(clientVersion string, opts *Options) *Context {
	context := &Context{
		Client: map[string]interface{}{
			"utcOffsetMinutes": opts.UTCOffset,
			"gl":               opts.GL,
			"hl":               opts.HL,
			"clientName":       "WEB",
			"clientVersion":    clientVersion,
		},
		User: map[string]interface{}{},
	}

	if opts.SafeSearch {
		context.User["enableSafetyMode"] = true
	}

	return context
}

func saveCache(parsed *ParsedData, opts *Options) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if parsed.Context != nil && parsed.Context.Client != nil {
		if cv, ok := parsed.Context.Client["clientVersion"].(string); ok {
			cache.ClientVersion = cv
		}
	}

	playlistParams := getPlaylistParams(parsed)
	if playlistParams != "" {
		cache.PlaylistParams = playlistParams
	}
}

func getPlaylistParams(parsed *ParsedData) string {
	if parsed.Body != "" {
		re := regexp.MustCompile(`"params":"([^"]+)"},"tooltip":"Search for Playlist"`)
		match := re.FindStringSubmatch(parsed.Body)
		if len(match) > 1 {
			return match[1]
		}
	}
	return ""
}

func doPost(url string, opts *Options, payload map[string]interface{}) (map[string]interface{}, error) {
	client := &http.Client{}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url+"?prettyPrint=false", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Cookie", ConsentCookie)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&result)
	return result, err
}

func findTwoColumnSearchResultsRenderer(m map[string]interface{}) (map[string]interface{}, bool) {
	for k, v := range m {
		if k == "twoColumnSearchResultsRenderer" {
			if mm, ok := v.(map[string]interface{}); ok {
				return mm, true
			}
		}
		switch t := v.(type) {
		case map[string]interface{}:
			if res, ok := findTwoColumnSearchResultsRenderer(t); ok {
				return res, true
			}
		case []interface{}:
			for _, e := range t {
				if em, ok := e.(map[string]interface{}); ok {
					if res, ok := findTwoColumnSearchResultsRenderer(em); ok {
						return res, true
					}
				}
			}
		}
	}
	return nil, false
}

func prepareThumbnails(thumbnails []interface{}) []Thumbnail {
	var result []Thumbnail

	for _, thumb := range thumbnails {
		if thumbMap, ok := thumb.(map[string]interface{}); ok {
			thumbnail := Thumbnail{}

			if urlStr, ok := thumbMap["url"].(string); ok {
				if u, err := url.Parse(BaseURL); err == nil {
					if fullUrl, err := u.Parse(urlStr); err == nil {
						thumbnail.URL = fullUrl.String()
					} else {
						thumbnail.URL = urlStr
					}
				} else {
					thumbnail.URL = urlStr
				}
			}

			if width, ok := thumbMap["width"].(float64); ok {
				thumbnail.Width = int(width)
			}

			if height, ok := thumbMap["height"].(float64); ok {
				thumbnail.Height = int(height)
			}

			result = append(result, thumbnail)
		}
	}

	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[i].Width < result[j].Width {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}
