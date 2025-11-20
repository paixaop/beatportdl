package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"unspok3n/beatportdl/config"
	"unspok3n/beatportdl/internal/beatport"
)

func Setup(configDir string) (cfg *config.AppConfig, cachePath string, err error) {
	configFilePath, exists, err := FindConfigFile(configDir)
	if err != nil {
		return nil, "", err
	}

	if !exists {
		fmt.Println("Config file not found, creating a new one:", configFilePath)

		fmt.Print("Username: ")
		username := GetLine()
		fmt.Print("Password: ")
		password := GetLine()
		fmt.Print("Downloads directory: ")
		downloadsDir := GetLine()

		cfg := &config.AppConfig{
			Username:           username,
			Password:           password,
			DownloadsDirectory: downloadsDir,
		}

		fmt.Println("1. Lossless (44.1 khz FLAC)\n2. High (256 kbps AAC)\n3. Medium (128 kbps AAC)\n4. Medium HLS (128 kbps AAC)")
		for {
			fmt.Print("Quality: ")
			qualityNumber := GetLine()
			switch qualityNumber {
			case "1":
				cfg.Quality = "lossless"
			case "2":
				cfg.Quality = "high"
			case "3":
				cfg.Quality = "medium"
			case "4":
				cfg.Quality = "medium-hls"
			default:
				fmt.Println("Invalid quality")
				continue
			}
			break
		}

		if err := cfg.Save(configFilePath); err != nil {
			return nil, configFilePath, fmt.Errorf("save config: %w", err)
		}
	}

	parsedConfig, err := config.Parse(configFilePath)
	if err != nil {
		return nil, configFilePath, fmt.Errorf("load config: %w", err)
	}

	cacheFilePath, exists, err := FindCacheFile(configDir)
	if err != nil {
		return nil, configFilePath, fmt.Errorf("get executable path: %w", err)
	}

	return parsedConfig, cacheFilePath, nil
}

func (app *application) mainPrompt() {
	fmt.Print("Enter url, track ID, or search query: ")
	input := GetLine()

	// Check if input is a URL
	if strings.HasPrefix(input, beatport.BeatportMainUrl) || strings.HasPrefix(input, beatport.BeatsourceMainUrl) {
		app.urls = append(app.urls, input)
		return
	}

	// Check if input is a track ID (numeric only)
	if trackID, err := strconv.ParseInt(strings.TrimSpace(input), 10, 64); err == nil && trackID > 0 {
		app.handleTrackID(trackID)
		return
	}

	// Otherwise, treat as search query
	app.search(input)
}

func (app *application) handleTrackID(trackID int64) {
	fmt.Printf("🎵 Downloading track ID: %d\n", trackID)

	// Check if user specified a store preference
	fmt.Print("Which store? (1) Beatport [default] (2) Beatsource: ")
	storeChoice := GetLine()

	var trackURL string
	var storeName string

	switch strings.TrimSpace(storeChoice) {
	case "2", "beatsource", "bs":
		trackURL = fmt.Sprintf("%s/track/track-name/%d", beatport.BeatsourceMainUrl, trackID)
		storeName = "Beatsource"
	default:
		trackURL = fmt.Sprintf("%s/track/track-name/%d", beatport.BeatportMainUrl, trackID)
		storeName = "Beatport"
	}

	// Add to URLs for processing
	app.urls = append(app.urls, trackURL)

	fmt.Printf("✅ Added track ID %d from %s to download queue\n", trackID, storeName)
}

func (app *application) handleTrackIDForStore(trackID int64, store string) string {
	switch strings.ToLower(store) {
	case "beatsource", "bs":
		return fmt.Sprintf("%s/track/track-name/%d", beatport.BeatsourceMainUrl, trackID)
	default:
		return fmt.Sprintf("%s/track/track-name/%d", beatport.BeatportMainUrl, trackID)
>>>>>>> 515bc7c (Initial commit)
	}
}

func (app *application) search(input string) {
	var storeTag string
	var inst *beatport.Beatport
	storeTag, input = extractStoreTag(input)
	switch storeTag {
	default:
		inst = app.bp
	case "beatsource":
		inst = app.bs
	}

	results, err := inst.Search(input)
	if err != nil {
		app.FatalError("beatport", err)
	}
	trackResultsLen := len(results.Tracks)
	releasesResultsLen := len(results.Releases)

	if trackResultsLen+releasesResultsLen == 0 {
		fmt.Println("No results found")
		return
	}

	fmt.Println("Search results:")
	fmt.Println("[ Tracks ]")
	for i, track := range results.Tracks {
		fmt.Printf(
			"%2d. %s - %s (%s) [%s]\n", i+1,
			track.Artists.Display(
				app.config.ArtistsLimit,
				app.config.ArtistsShortForm,
			),
			track.Name.String(),
			track.MixName.String(),
			track.Length,
		)
	}
	fmt.Println("\n[ Releases ]")
	indexOffset := trackResultsLen + 1
	for i, release := range results.Releases {
		fmt.Printf(
			"%2d. %s - %s [%s]\n", i+indexOffset,
			release.Artists.Display(
				app.config.ArtistsLimit,
				app.config.ArtistsShortForm,
			),
			release.Name.String(),
			release.Label.Name,
		)
	}
	fmt.Print("Enter the result number(s): ")
	input = GetLine()
	requestedResults := strings.Split(input, " ")
	for _, result := range requestedResults {
		resultInt, err := strconv.Atoi(result)
		if err != nil {
			fmt.Printf("invalid result number: %s\n", result)
			continue
		}

		if resultInt > releasesResultsLen+trackResultsLen || resultInt == 0 {
			fmt.Printf("invalid result number: %d\n", resultInt)
			continue
		}

		if resultInt >= indexOffset {
			app.urls = append(app.urls, results.Releases[resultInt-indexOffset].URL)
		} else {
			app.urls = append(app.urls, results.Tracks[resultInt-1].URL)
		}
	}
}

func extractStoreTag(query string) (store, trimmedQuery string) {
	re := regexp.MustCompile(`@\w+`)
	matches := re.FindAllString(query, -1)
	if len(matches) > 0 {
		store = strings.TrimPrefix(matches[0], "@")
		trimmedQuery = re.ReplaceAllString(query, "")
		trimmedQuery = strings.TrimSpace(trimmedQuery)
	} else {
		trimmedQuery = query
	}
	return store, trimmedQuery
}

func (app *application) parseTextFile(path string) {
	app.parseTextFileWithStore(path, "beatport") // Default to Beatport for backward compatibility
}

func (app *application) parseTextFileWithStore(path string, defaultStore string) {
	file, err := os.Open(path)
	if err != nil {
		app.FatalError("read input text file", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Split(bufio.ScanLines)

	lineCount := 0
	urlCount := 0
	trackIDCount := 0

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		lineCount++

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Check if line is a URL
		if strings.HasPrefix(line, beatport.BeatportMainUrl) || strings.HasPrefix(line, beatport.BeatsourceMainUrl) {
			app.urls = append(app.urls, line)
			urlCount++
			continue
		}

		// Check if line is a track ID (numeric only)
		if trackID, err := strconv.ParseInt(line, 10, 64); err == nil && trackID > 0 {
			trackURL := app.handleTrackIDForStore(trackID, defaultStore)
			app.urls = append(app.urls, trackURL)
			trackIDCount++
			continue
		}

		// If it's neither URL nor track ID, treat as URL (backward compatibility)
		app.urls = append(app.urls, line)
		urlCount++
	}

	if err := scanner.Err(); err != nil {
		app.FatalError("reading text file", err)
	}

	// Print summary
	fmt.Printf("📁 Parsed file: %s\n", path)
	if urlCount > 0 {
		fmt.Printf("   📎 URLs: %d\n", urlCount)
	}
	if trackIDCount > 0 {
		storeName := "Beatport"
		if strings.ToLower(defaultStore) == "beatsource" || strings.ToLower(defaultStore) == "bs" {
			storeName = "Beatsource"
		}
		fmt.Printf("   🆔 Track IDs (%s): %d\n", storeName, trackIDCount)
	}
	fmt.Printf("   📊 Total items: %d\n", urlCount+trackIDCount)
>>>>>>> 515bc7c (Initial commit)
}

var (
	ErrUnsupportedLinkType  = errors.New("unsupported link type")
	ErrUnsupportedLinkStore = errors.New("unsupported link store")
)
