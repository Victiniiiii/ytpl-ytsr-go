package ytsr

import "sync"

type Cache struct {
	mu             sync.RWMutex
	ClientVersion  string
	PlaylistParams string
}

type Options struct {
	Query      string
	Type       string
	Limit      int
	SafeSearch bool
	GL         string
	HL         string
	UTCOffset  int
}

type SearchResult struct {
	Query   string
	Items   []SearchItem
	Results int
}

type SearchItem struct {
	Type        string
	ID          string
	URL         string
	Name        string
	Description string
	Duration    string
	Thumbnail   string
	Thumbnails  []Thumbnail
	UploadedAt  string
	Views       *int
	Author      *Author
	IsLive      bool
	Badges      []string
	Owner       *Owner
}

type Thumbnail struct {
	URL    string
	Width  int
	Height int
}

type Author struct {
	Name       string
	ChannelID  string
	URL        string
	BestAvatar *Thumbnail
	Avatars    []Thumbnail
	Verified   bool
	Badges     []string
}

type Owner struct {
	Name      string
	ChannelID string
	URL       string
	Verified  bool
	Badges    []string
}

type Context struct {
	Client map[string]interface{} `json:"client"`
	User   map[string]interface{} `json:"user"`
}

type ParsedData struct {
	JSON    map[string]interface{}
	Context *Context
	Body    string
}
