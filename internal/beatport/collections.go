package beatport

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"
)

type Collection struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	TrackCount  int       `json:"track_count"`
	UpdatedDate time.Time `json:"updated_date"`
}

type CollectionItem struct {
	ID    int64 `json:"id"`
	Track Track `json:"track"`
}

func (c *Collection) DirectoryName(n NamingPreferences) string {
	templateValues := map[string]string{
		"id":           strconv.Itoa(int(c.ID)),
		"name":         SanitizeForPath(c.Name, n.AsciiOnly),
		"track_count":  NumberWithPadding(c.TrackCount, c.TrackCount, n.TrackNumberPadding),
		"updated_date": c.UpdatedDate.Format("2006-01-02"),
		"first_artist": "",
	}
	directoryName := ParseTemplate(n.Template, templateValues)
	return SanitizePath(directoryName, n.Whitespace, n.AsciiOnly)
}

func (b *Beatport) GetCollection() (*Collection, error) {
	res, err := b.fetch(
		"GET",
		BeatportMyCollection+"/",
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	// The collection endpoint might return a different structure
	// Let's first try to understand the response structure
	var response map[string]interface{}
	if err = json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}

	collection := &Collection{
		Name:        "My Collection",
		UpdatedDate: time.Now(),
	}

	// Extract collection metadata if available
	if meta, ok := response["collection"].(map[string]interface{}); ok {
		if id, ok := meta["id"].(float64); ok {
			collection.ID = int64(id)
		}
		if name, ok := meta["name"].(string); ok {
			collection.Name = name
		}
		if updatedDate, ok := meta["updated_date"].(string); ok {
			if parsedTime, err := time.Parse("2006-01-02T15:04:05Z", updatedDate); err == nil {
				collection.UpdatedDate = parsedTime
			}
		}
	}

	// Count tracks if available
	if tracks, ok := response["tracks"].([]interface{}); ok {
		collection.TrackCount = len(tracks)
	} else if count, ok := response["count"].(float64); ok {
		collection.TrackCount = int(count)
	}

	return collection, nil
}

func (b *Beatport) GetCollectionItems(id int64, page int, params string) (*Paginated[CollectionItem], error) {
	// id parameter is ignored for collection items since it's user-specific
	res, err := b.fetch(
		"GET",
		fmt.Sprintf("%s/?page=%d&%s", BeatportMyCollection, page, params),
		nil,
		"",
	)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	var response Paginated[CollectionItem]
	if err = json.NewDecoder(res.Body).Decode(&response); err != nil {
		return nil, err
	}

	for i := range response.Results {
		response.Results[i].Track.Store = b.store
	}

	return &response, nil
}
