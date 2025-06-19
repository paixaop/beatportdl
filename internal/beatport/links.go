package beatport

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type LinkType string
type Store string

var (
	TrackLink      LinkType = "tracks"
	ReleaseLink    LinkType = "releases"
	PlaylistLink   LinkType = "playlists"
	ChartLink      LinkType = "charts"
	LabelLink      LinkType = "labels"
	ArtistLink     LinkType = "artists"
	CollectionLink LinkType = "collection"

	StoreBeatport   Store = "beatport"
	StoreBeatsource Store = "beatsource"
)

type Link struct {
	Original string
	Type     LinkType
	ID       int64
	Params   string
	Store    Store
}

var (
	ErrInvalidUrl = errors.New("invalid url")
)

func (b *Beatport) ParseUrl(inputURL string) (*Link, error) {
	u, err := url.Parse(inputURL)
	if err != nil {
		return nil, err
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	segmentsLength := len(segments)
	link := Link{
		Original: inputURL,
	}

	switch u.Host {
	case BeatportDomain, BeatportAPIDomain:
		link.Store = StoreBeatport
	case BeatsourceDomain, BeatsourceAPIDomain:
		link.Store = StoreBeatsource
	default:
		return nil, ErrInvalidUrl
	}

	if segmentsLength == 0 {
		return nil, ErrInvalidUrl
	}

	if segmentsLength > 1 && len(segments[0]) == 2 {
		segments = segments[1:]
		segmentsLength--

		if segments[0] == "catalog" {
			segments = segments[1:]
			segmentsLength--
		}
	}

	var idSegment int

	switch segments[0] {
	case "track":
		idSegment = 2
		link.Type = TrackLink
	case "release":
		idSegment = 2
		link.Type = ReleaseLink
	case "library":
		// Handle /library URL without ID (for collection)
		if segmentsLength == 1 {
			link.Type = CollectionLink
			link.ID = 0 // No ID required for collection
			link.Params = u.RawQuery
			return &link, nil
		}
		switch segments[1] {
		case "playlists", "playlist":
			idSegment = 2
			link.Type = PlaylistLink
		default:
			return nil, fmt.Errorf("invalid link type: %s/%s", segments[0], segments[1])
		}
	case "playlists":
		idSegment = 2
		link.Type = PlaylistLink
	case "chart", "playlist":
		idSegment = 2
		link.Type = ChartLink
	case "label":
		idSegment = 2
		link.Type = LabelLink
	case "artist":
		idSegment = 2
		link.Type = ArtistLink

	case "tracks":
		idSegment = 1
		link.Type = TrackLink
	case "releases":
		idSegment = 1
		link.Type = ReleaseLink
	default:
		return nil, ErrInvalidUrl
	}

	if idSegment+1 > segmentsLength {
		return nil, ErrInvalidUrl
	}

	link.ID, err = strconv.ParseInt(segments[idSegment], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid id: %v", err)
	}

	link.Params = u.RawQuery

	return &link, nil
}
