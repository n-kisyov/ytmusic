package youtube

import "ytmusic/internal/config"

// SearchResult holds tracks fetched from a search or continuation request.
type SearchResult struct {
	Tracks            []config.Track `json:"tracks"`
	ContinuationToken string         `json:"continuation_token"`
}
