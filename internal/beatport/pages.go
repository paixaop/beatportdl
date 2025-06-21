package beatport

import (
	"fmt"
	"log"
	"strings"
)

// Common Beatport page URLs
const (
	BeatportMyCollection = "/library"
	BeatportTop100       = "/top-100"
	BeatportHype         = "/hype"
	BeatportReleases     = "/releases"
	BeatportCharts       = "/charts"
)

// PageType represents different types of Beatport pages
type PageType string

const (
	PageTypeCollection PageType = "collection"
	PageTypeTop100     PageType = "top100"
	PageTypeHype       PageType = "hype"
	PageTypeReleases   PageType = "releases"
	PageTypeCharts     PageType = "charts"
	PageTypeCustom     PageType = "custom"
)

// BeatportPageInfo contains information about a Beatport page
type BeatportPageInfo struct {
	URL      string
	PageType PageType
	Title    string
}

// GetPageInfo determines the page type and information from a URL
func GetPageInfo(url string) BeatportPageInfo {
	info := BeatportPageInfo{
		URL:      url,
		PageType: PageTypeCustom,
		Title:    "Custom Page",
	}

	// Normalize URL to check paths
	urlLower := strings.ToLower(url)

	switch {
	case strings.Contains(urlLower, BeatportMyCollection):
		info.PageType = PageTypeCollection
		info.Title = "My Collection"
	case strings.Contains(urlLower, BeatportTop100):
		info.PageType = PageTypeTop100
		info.Title = "Top 100"
	case strings.Contains(urlLower, BeatportHype):
		info.PageType = PageTypeHype
		info.Title = "Hype"
	case strings.Contains(urlLower, BeatportReleases):
		info.PageType = PageTypeReleases
		info.Title = "Releases"
	case strings.Contains(urlLower, BeatportCharts):
		info.PageType = PageTypeCharts
		info.Title = "Charts"
	}

	return info
}

// DownloadCollectionTracks downloads tracks from user's collection
func (b *Beatport) DownloadCollectionTracks(config interface{}, configPath string) ([]string, error) {
	collectionURL := BeatportMainUrl + BeatportMyCollection
	log.Printf("Downloading collection tracks from: %s", collectionURL)
	return b.DownloadPageWithAuth(collectionURL, config, configPath)
}

// DownloadTop100Tracks downloads tracks from Top 100 page
func (b *Beatport) DownloadTop100Tracks(config interface{}, configPath string) ([]string, error) {
	top100URL := BeatportMainUrl + BeatportTop100
	log.Printf("Downloading Top 100 tracks from: %s", top100URL)
	return b.DownloadPageWithAuth(top100URL, config, configPath)
}

// DownloadHypeTracks downloads tracks from Hype page
func (b *Beatport) DownloadHypeTracks(config interface{}, configPath string) ([]string, error) {
	hypeURL := BeatportMainUrl + BeatportHype
	log.Printf("Downloading Hype tracks from: %s", hypeURL)
	return b.DownloadPageWithAuth(hypeURL, config, configPath)
}

// DownloadTracksFromURL downloads tracks from any Beatport URL with authentication
func (b *Beatport) DownloadTracksFromURL(url string, config interface{}, configPath string) ([]string, error) {
	pageInfo := GetPageInfo(url)
	log.Printf("Downloading tracks from %s: %s", pageInfo.Title, url)

	// Ensure URL is absolute
	if strings.HasPrefix(url, "/") {
		url = BeatportMainUrl + url
	}

	tracks, err := b.DownloadPageWithAuth(url, config, configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to download tracks from %s: %w", pageInfo.Title, err)
	}

	log.Printf("Successfully extracted %d track links from %s", len(tracks), pageInfo.Title)
	return tracks, nil
}

// PrintTrackLinks prints the extracted track links in a readable format
func PrintTrackLinks(tracks []string, pageTitle string) {
	fmt.Printf("\n=== Track Links from %s ===\n", pageTitle)
	for i, track := range tracks {
		fmt.Printf("%d. %s\n", i+1, track)
	}
	fmt.Printf("\nTotal tracks found: %d\n\n", len(tracks))
}
