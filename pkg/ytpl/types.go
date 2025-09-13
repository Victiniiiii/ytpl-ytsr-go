package ytpl

import "net/http"

type PlaylistItem struct {
	ID         string `json:"id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	Duration   string `json:"duration"`
	Thumbnail  string `json:"thumbnail"`
	Author     string `json:"author"`
	AuthorURL  string `json:"author_url"`
	IsLiveNow  bool   `json:"is_live_now"`
	IsUpcoming bool   `json:"is_upcoming"`
	IsPremiere bool   `json:"is_premiere"`
}

type Thumbnail struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}

type PlaylistInfo struct {
	ID          string         `json:"id"`
	Thumbnail   Thumbnail      `json:"thumbnail"`
	URL         string         `json:"url"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	TotalItems  int            `json:"total_items"`
	Views       int            `json:"views"`
	Items       []PlaylistItem `json:"items"`
}

type Options struct {
	Limit          int
	RequestOptions *http.Client
	Query          map[string]string
}

type Context struct {
	Client struct {
		ClientName    string `json:"clientName"`
		ClientVersion string `json:"clientVersion"`
	} `json:"client"`
}

type ParsedResponse struct {
	JSON    map[string]interface{}
	APIKey  string
	Context Context
}
