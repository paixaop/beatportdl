package beatport

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
)

type Top100 struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	URL         string `json:"url"`
	Type        string `json:"type"`
	GenreID     int64  `json:"genre_id,omitempty"`
	GenreName   string `json:"genre_name,omitempty"`
	Tracks      []Track
}

// GetTop100 fetches a top-100 list
func (b *Beatport) GetTop100(ctx context.Context, id int64) (*Top100, error) {
	var endpoint string

	if id == 0 {
		// Main top-100 list
		endpoint = "/catalog/top-100"
	} else {
		// Genre-specific top-100
		endpoint = fmt.Sprintf("/catalog/genres/%d/top-100", id)
	}

	// Create request with context
	var baseUrl string
	switch b.store {
	default:
		baseUrl = BeatportAPIUrl
	case StoreBeatsource:
		baseUrl = BeatsourceAPIUrl
	}

	req, err := http.NewRequestWithContext(ctx, "GET", baseUrl+endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add headers and auth
	for key, value := range b.headers {
		req.Header.Add(key, value)
	}
	if b.auth.tokenPair != nil {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b.auth.tokenPair.AccessToken))
	}

	// Execute request
	resp, err := b.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Debug the actual response structure
	var rawResponse map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&rawResponse); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	// Create the top100 object
	top100 := &Top100{
		GenreID: id,
	}

	// Extract top100 metadata
	if meta, ok := rawResponse["top100"].(map[string]interface{}); ok {
		if id, ok := meta["id"].(float64); ok {
			top100.ID = int64(id)
		}
		if name, ok := meta["name"].(string); ok {
			top100.Name = name
		}
		if desc, ok := meta["description"].(string); ok {
			top100.Description = desc
		}
		if url, ok := meta["url"].(string); ok {
			top100.URL = url
		}
		if typ, ok := meta["type"].(string); ok {
			top100.Type = typ
		}
	}

	// If this is a genre-specific top-100, add the genre info
	if id > 0 {
		// Fetch the genre name using the basic genre struct
		var genre Genre
		genreEndpoint := fmt.Sprintf("/catalog/genres/%d/", id)
		genreReq, err := http.NewRequestWithContext(ctx, "GET", baseUrl+genreEndpoint, nil)
		if err == nil {
			// Add headers and auth
			for key, value := range b.headers {
				genreReq.Header.Add(key, value)
			}
			if b.auth.tokenPair != nil {
				genreReq.Header.Set("Authorization", fmt.Sprintf("Bearer %s", b.auth.tokenPair.AccessToken))
			}

			genreResp, err := b.client.Do(genreReq)
			if err == nil && genreResp.StatusCode == http.StatusOK {
				defer genreResp.Body.Close()
				if err = json.NewDecoder(genreResp.Body).Decode(&genre); err == nil {
					top100.GenreName = genre.Name
				}
			}
		}
	}

	// Extract tracks from the response
	if tracks, ok := rawResponse["tracks"].([]interface{}); ok {
		top100.Tracks = make([]Track, 0, len(tracks))

		for _, item := range tracks {
			if trackData, ok := item.(map[string]interface{}); ok {
				var track Track

				// Extract basic track info
				if id, ok := trackData["id"].(float64); ok {
					track.ID = int64(id)
				}
				if name, ok := trackData["name"].(string); ok {
					track.Name = SanitizedString(name)
				}
				if mixName, ok := trackData["mix_name"].(string); ok {
					track.MixName = SanitizedString(mixName)
				}
				if slug, ok := trackData["slug"].(string); ok {
					track.Slug = slug
				}

				// Get track details
				fullTrack, err := b.GetTrack(track.ID)
				if err == nil && fullTrack != nil {
					track = *fullTrack
				}

				top100.Tracks = append(top100.Tracks, track)
			}
		}
	}

	return top100, nil
}

// Define the Top100Link constant
const Top100Link LinkType = "top100"
// GetTop100FromLink fetches a top-100 list from a Beatport link
func (b *Beatport) GetTop100FromLink(ctx context.Context, link *Link) (*Top100, error) {
	if link.Type != Top100Link {
		return nil, fmt.Errorf("invalid link type: %s", link.Type)
	}

	return b.GetTop100(ctx, link.ID)
}

// Define DownloadOptions struct
type DownloadOptions struct {
	Quality      string
	TrackNumber  string
	Position     int
	TotalTracks  int
	SkipExisting bool
}
// DownloadTop100 downloads all tracks from a top-100 list
func (b *Beatport) DownloadTop100(ctx context.Context, top100 *Top100, options *DownloadOptions) error {
	totalTracks := len(top100.Tracks)
	if totalTracks == 0 {
		return fmt.Errorf("no tracks found in top-100 list")
	}

	// Create a folder for the top-100 list
	var folderName string
	if top100.GenreID > 0 {
		folderName = fmt.Sprintf("Top 100 %s", top100.GenreName)
	} else {
		folderName = "Top 100"
	}

	for i, track := range top100.Tracks {
		position := i + 1

		// Add position to track title - with proper sanitization
		positionPrefix := fmt.Sprintf("%d. ", position)
		track.Name = SanitizedString(positionPrefix + string(track.Name))

		// Add top-100 list to track's release name for better organization
		releaseNameWithTop100 := fmt.Sprintf("%s (%s)", string(track.Release.Name), folderName)
		track.Release.Name = SanitizedString(releaseNameWithTop100)

		// Update download options for this track
		trackOptions := *options
		trackOptions.TrackNumber = strconv.Itoa(position)
		trackOptions.Position = position
		trackOptions.TotalTracks = totalTracks

		// Download track using the track ID and quality
		_, err := b.DownloadTrack(track.ID, trackOptions.Quality)
		if err != nil {
			return err
		}
	}

	return nil
}
