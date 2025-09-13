package ytpl

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	BasePlistURL = "https://www.youtube.com/playlist?"
	BaseAPIURL   = "https://www.youtube.com/youtubei/v1/browse?key="
)

var (
	PlaylistRegex      = regexp.MustCompile(`^(FL|PL|UU|LL|RD)[a-zA-Z0-9-_]{16,41}$`)
	AlbumRegex         = regexp.MustCompile(`^OLAK5uy_[a-zA-Z0-9-_]{33}$`)
	ChannelRegex       = regexp.MustCompile(`^UC[a-zA-Z0-9-_]{22,32}$`)
	ChannelOnPageRegex = regexp.MustCompile(`channel_id=UC([\w-]{22,32})"`)
	YTHosts            = []string{"www.youtube.com", "youtube.com", "music.youtube.com"}
)

func GetPlaylistID(linkOrID string) (string, error) {
	if linkOrID == "" {
		return "", errors.New("the linkOrId has to be a non-empty string")
	}

	if PlaylistRegex.MatchString(linkOrID) || AlbumRegex.MatchString(linkOrID) {
		return linkOrID, nil
	}

	if ChannelRegex.MatchString(linkOrID) {
		return "UU" + linkOrID[2:], nil
	}

	parsed, err := url.Parse(linkOrID)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %v", err)
	}

	validHost := false
	for _, host := range YTHosts {
		if parsed.Host == host {
			validHost = true
			break
		}
	}
	if !validHost {
		return "", errors.New("not a known youtube link")
	}

	if parsed.Query().Has("list") {
		listParam := parsed.Query().Get("list")
		if PlaylistRegex.MatchString(listParam) || AlbumRegex.MatchString(listParam) {
			return listParam, nil
		}
		if strings.HasPrefix(listParam, "RD") {
			return "", errors.New("mixes not supported")
		}
		return "", errors.New("invalid or unknown list query in url")
	}

	pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(pathParts) < 2 {
		return "", fmt.Errorf("unable to find a id in \"%s\"", linkOrID)
	}

	maybeType := pathParts[len(pathParts)-2]
	maybeID := pathParts[len(pathParts)-1]

	switch maybeType {
	case "channel":
		if ChannelRegex.MatchString(maybeID) {
			return "UU" + maybeID[2:], nil
		}
	case "user":
		return toChannelList(fmt.Sprintf("https://www.youtube.com/user/%s", maybeID))
	case "c":
		return toChannelList(fmt.Sprintf("https://www.youtube.com/c/%s", maybeID))
	}

	return "", fmt.Errorf("unable to find a id in \"%s\"", linkOrID)
}

func toChannelList(ref string) (string, error) {
	resp, err := http.Get(ref)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	matches := ChannelOnPageRegex.FindStringSubmatch(string(body))
	if len(matches) > 1 {
		return "UU" + matches[1], nil
	}

	return "", fmt.Errorf("unable to resolve the ref: %s", ref)
}

func ValidateID(linkOrID string) bool {
	if linkOrID == "" {
		return false
	}

	if PlaylistRegex.MatchString(linkOrID) || AlbumRegex.MatchString(linkOrID) || ChannelRegex.MatchString(linkOrID) {
		return true
	}

	parsed, err := url.Parse(linkOrID)
	if err != nil {
		return false
	}

	validHost := false
	for _, host := range YTHosts {
		if parsed.Host == host {
			validHost = true
			break
		}
	}
	if !validHost {
		return false
	}

	if parsed.Query().Has("list") {
		listParam := parsed.Query().Get("list")
		if PlaylistRegex.MatchString(listParam) || AlbumRegex.MatchString(listParam) {
			return true
		}
		if strings.HasPrefix(listParam, "RD") {
			return false
		}
		return false
	}

	pathParts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(pathParts) < 2 {
		return false
	}

	maybeType := pathParts[len(pathParts)-2]
	maybeID := pathParts[len(pathParts)-1]

	switch maybeType {
	case "channel":
		return ChannelRegex.MatchString(maybeID)
	case "user", "c":
		return true
	}

	return false
}

func GetPlaylist(linkOrID string, options *Options) (*PlaylistInfo, error) {
	return getPlaylist(linkOrID, options, 3)
}

func getPlaylist(linkOrID string, options *Options, retries int) (*PlaylistInfo, error) {
	plistID, err := GetPlaylistID(linkOrID)
	if err != nil {
		return nil, err
	}

	opts := checkArgs(plistID, options)

	params := url.Values{}
	for k, v := range opts.Query {
		params.Set(k, v)
	}
	refURL := BasePlistURL + params.Encode()

	resp, err := opts.RequestOptions.Get(refURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	parsed, err := parseBody(string(body), opts)
	if err != nil {
		return nil, err
	}

	if parsed.JSON == nil {
		browseID := "VL" + plistID
		if parsed.APIKey == "" || parsed.Context.Client.ClientVersion == "" {
			return nil, errors.New("missing api key or client version")
		}

		payload := map[string]interface{}{
			"context":  parsed.Context,
			"browseId": browseID,
		}

		apiResp, err := doPost(BaseAPIURL+parsed.APIKey, opts.RequestOptions, payload)
		if err == nil {
			parsed.JSON = apiResp
		}
	}

	if parsed.JSON["sidebar"] == nil {
		return nil, errors.New("unknown Playlist")
	}

	if parsed.JSON == nil {
		if retries == 0 {
			logger(string(body))
			return nil, errors.New("unsupported playlist")
		}
		return getPlaylist(linkOrID, opts, retries-1)
	}

	if alerts, ok := parsed.JSON["alerts"]; ok && parsed.JSON["contents"] == nil {
		if alertsList, ok := alerts.([]interface{}); ok {
			for _, alert := range alertsList {
				if alertMap, ok := alert.(map[string]interface{}); ok {
					if alertRenderer, ok := alertMap["alertRenderer"].(map[string]interface{}); ok {
						if alertType, ok := alertRenderer["type"].(string); ok && alertType == "ERROR" {
							errorText := parseText(alertRenderer["text"])
							return nil, errors.New(errorText)
						}
					}
				}
			}
		}
	}

	sidebar, ok := parsed.JSON["sidebar"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid sidebar structure")
	}

	playlistSidebar, ok := sidebar["playlistSidebarRenderer"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid playlist sidebar structure")
	}

	items, ok := playlistSidebar["items"].([]interface{})
	if !ok {
		return nil, errors.New("invalid items structure")
	}

	var info map[string]interface{}
	for _, item := range items {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if primaryInfo, ok := itemMap["playlistSidebarPrimaryInfoRenderer"]; ok {
				info, _ = primaryInfo.(map[string]interface{})
				break
			}
		}
	}

	if info == nil {
		return nil, errors.New("could not find playlist info")
	}

	resp_info := &PlaylistInfo{
		ID:  plistID,
		URL: fmt.Sprintf("%slist=%s", BasePlistURL, plistID),
	}

	resp_info.Title = parseText(info["title"])
	resp_info.Description = parseText(info["description"])

	if thumbnailRenderer, ok := info["thumbnailRenderer"].(map[string]interface{}); ok {
		var thumbnailData map[string]interface{}

		if playlistVideoThumbnail, ok := thumbnailRenderer["playlistVideoThumbnailRenderer"].(map[string]interface{}); ok {
			thumbnailData = playlistVideoThumbnail
		} else if playlistCustomThumbnail, ok := thumbnailRenderer["playlistCustomThumbnailRenderer"].(map[string]interface{}); ok {
			thumbnailData = playlistCustomThumbnail
		}

		if thumbnailData != nil {
			if thumbnail, ok := thumbnailData["thumbnail"].(map[string]interface{}); ok {
				if thumbnails, ok := thumbnail["thumbnails"].([]interface{}); ok && len(thumbnails) > 0 {
					var bestThumbnail map[string]interface{}
					maxWidth := 0
					for _, thumb := range thumbnails {
						if thumbMap, ok := thumb.(map[string]interface{}); ok {
							if width, ok := thumbMap["width"].(float64); ok && int(width) > maxWidth {
								maxWidth = int(width)
								bestThumbnail = thumbMap
							}
						}
					}
					if bestThumbnail != nil {
						resp_info.Thumbnail = Thumbnail{
							URL:    bestThumbnail["url"].(string),
							Width:  int(bestThumbnail["width"].(float64)),
							Height: int(bestThumbnail["height"].(float64)),
						}
					}
				}
			}
		}
	}

	if stats, ok := info["stats"].([]interface{}); ok && len(stats) > 0 {
		resp_info.TotalItems = parseNumFromText(stats[0])
		if len(stats) >= 3 {
			resp_info.Views = parseNumFromText(stats[1])
		}
	}

	contents, ok := parsed.JSON["contents"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid contents structure")
	}

	twoColumnBrowse, ok := contents["twoColumnBrowseResultsRenderer"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid two column browse structure")
	}

	tabs, ok := twoColumnBrowse["tabs"].([]interface{})
	if !ok || len(tabs) == 0 {
		return nil, errors.New("invalid tabs structure")
	}

	firstTab, ok := tabs[0].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid first tab structure")
	}

	tabRenderer, ok := firstTab["tabRenderer"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid tab renderer structure")
	}

	content, ok := tabRenderer["content"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid tab content structure")
	}

	sectionList, ok := content["sectionListRenderer"].(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid section list structure")
	}

	sectionContents, ok := sectionList["contents"].([]interface{})
	if !ok {
		return nil, errors.New("invalid section contents structure")
	}

	var itemSectionRenderer map[string]interface{}
	for _, section := range sectionContents {
		if sectionMap, ok := section.(map[string]interface{}); ok {
			if itemSection, ok := sectionMap["itemSectionRenderer"]; ok {
				itemSectionRenderer, _ = itemSection.(map[string]interface{})
				break
			}
		}
	}

	if itemSectionRenderer == nil {
		return nil, errors.New("empty playlist")
	}

	itemSectionContents, ok := itemSectionRenderer["contents"].([]interface{})
	if !ok {
		return nil, errors.New("invalid item section contents")
	}

	var playlistVideoListRenderer map[string]interface{}
	for _, item := range itemSectionContents {
		if itemMap, ok := item.(map[string]interface{}); ok {
			if playlistVideoList, ok := itemMap["playlistVideoListRenderer"]; ok {
				playlistVideoListRenderer, _ = playlistVideoList.(map[string]interface{})
				break
			}
		}
	}

	if playlistVideoListRenderer == nil {
		return nil, errors.New("empty playlist")
	}

	rawVideoList, ok := playlistVideoListRenderer["contents"].([]interface{})
	if !ok {
		return nil, errors.New("invalid video list")
	}

	for i, rawVideo := range rawVideoList {
		if i >= opts.Limit {
			break
		}
		if item := parseItem(rawVideo); item != nil {
			resp_info.Items = append(resp_info.Items, *item)
		}
	}

	opts.Limit -= len(resp_info.Items)

	var token string
	for _, item := range rawVideoList {
		if itemMap, ok := item.(map[string]interface{}); ok {
			for key := range itemMap {
				if key == "continuationItemRenderer" {
					token = getContinuationToken(itemMap)
					break
				}
			}
			if token != "" {
				break
			}
		}
	}

	if token == "" || opts.Limit < 1 {
		return resp_info, nil
	}

	nestedResp, err := parsePage2(parsed.APIKey, token, parsed.Context, opts)
	if err != nil {
		return resp_info, err
	}

	resp_info.Items = append(resp_info.Items, nestedResp...)
	return resp_info, nil
}

func checkArgs(plistID string, options *Options) *Options {
	if options == nil {
		options = &Options{}
	}
	if options.Limit <= 0 {
		options.Limit = 100
	}
	if options.RequestOptions == nil {
		options.RequestOptions = &http.Client{Timeout: 30 * time.Second}
	}
	if options.Query == nil {
		options.Query = make(map[string]string)
	}
	options.Query["list"] = plistID
	return options
}
