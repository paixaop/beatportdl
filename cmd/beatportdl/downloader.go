package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"unspok3n/beatportdl/config"
	"unspok3n/beatportdl/internal/beatport"
	"unspok3n/beatportdl/internal/taglib"

	"github.com/google/uuid"
)

func (app *application) errorLogWrapper(url, step string, err error) {
	app.LogError(fmt.Sprintf("[%s] %s", url, step), err)
}

func (app *application) infoLogWrapper(url, message string) {
	app.LogInfo(fmt.Sprintf("[%s] %s", url, message))
}

func (app *application) createDirectory(baseDir string, subDir ...string) (string, error) {
	fullPath := filepath.Join(baseDir, filepath.Join(subDir...))
	err := CreateDirectory(fullPath)
	return fullPath, err
}

type DownloadsDirectoryEntity interface {
	DirectoryName(n beatport.NamingPreferences) string
}

func (app *application) setupDownloadsDirectory(baseDir string, entity DownloadsDirectoryEntity) (string, error) {
	if app.config.SortByContext {
		var subDir string
		switch castedEntity := entity.(type) {
		case *beatport.Release:
			subDir = castedEntity.DirectoryName(
				beatport.NamingPreferences{
					Template:           app.config.ReleaseDirectoryTemplate,
					Whitespace:         app.config.WhitespaceCharacter,
					ArtistsLimit:       app.config.ArtistsLimit,
					ArtistsShortForm:   app.config.ArtistsShortForm,
					TrackNumberPadding: app.config.TrackNumberPadding,
					AsciiOnly:          app.config.AsciiOnlyFileNames,
				},
			)
			if app.config.SortByLabel && entity != nil {
				baseDir = filepath.Join(baseDir, castedEntity.Label.Name)
			}
		case *beatport.Playlist:
			subDir = castedEntity.DirectoryName(
				beatport.NamingPreferences{
					Template:           app.config.PlaylistDirectoryTemplate,
					Whitespace:         app.config.WhitespaceCharacter,
					TrackNumberPadding: app.config.TrackNumberPadding,
					AsciiOnly:          app.config.AsciiOnlyFileNames,
				},
			)
		case *beatport.Chart:
			subDir = castedEntity.DirectoryName(
				beatport.NamingPreferences{
					Template:           app.config.ChartDirectoryTemplate,
					Whitespace:         app.config.WhitespaceCharacter,
					TrackNumberPadding: app.config.TrackNumberPadding,
					AsciiOnly:          app.config.AsciiOnlyFileNames,
				},
			)
		case *beatport.Label:
			subDir = castedEntity.DirectoryName(
				beatport.NamingPreferences{
					Template:   app.config.LabelDirectoryTemplate,
					Whitespace: app.config.WhitespaceCharacter,
					AsciiOnly:  app.config.AsciiOnlyFileNames,
				},
			)
		case *beatport.Artist:
			subDir = castedEntity.DirectoryName(
				beatport.NamingPreferences{
					Template:   app.config.ArtistDirectoryTemplate,
					Whitespace: app.config.WhitespaceCharacter,
					AsciiOnly:  app.config.AsciiOnlyFileNames,
				},
			)
		case *beatport.Collection:
			subDir = castedEntity.DirectoryName(
				beatport.NamingPreferences{
					Template:           app.config.PlaylistDirectoryTemplate,
					Whitespace:         app.config.WhitespaceCharacter,
					TrackNumberPadding: app.config.TrackNumberPadding,
				},
			)
		}
		baseDir = filepath.Join(baseDir, subDir)
	}
	return app.createDirectory(baseDir)
}

func (app *application) requireCover(respectFixTags, respectKeepCover bool) bool {
	fixTags := respectFixTags && app.config.FixTags &&
		(app.config.CoverSize != config.DefaultCoverSize || app.config.Quality != "lossless")
	keepCover := respectKeepCover && app.config.SortByContext && app.config.KeepCover
	return fixTags || keepCover
}

func (app *application) downloadCover(image beatport.Image, downloadsDir string) (string, error) {
	coverUrl := image.FormattedUrl(app.config.CoverSize)
	coverPath := filepath.Join(downloadsDir, uuid.New().String())
	err := app.downloadFile(coverUrl, coverPath, "")
	if err != nil {
		os.Remove(coverPath)
		return "", err
	}
	return coverPath, nil
}

func (app *application) handleCoverFile(path string) error {
	if path == "" {
		return nil
	}
	if app.config.KeepCover && app.config.SortByContext {
		newPath := filepath.Dir(path) + "/cover.jpg"
		if err := os.Rename(path, newPath); err != nil {
			return err
		}
	} else {
		os.Remove(path)
	}
	return nil
}

var (
	ErrTrackFileExists = errors.New("file already exists")
)

func (app *application) saveTrack(inst *beatport.Beatport, track *beatport.Track, directory string, quality string) (string, error) {
	var fileExtension string
	var displayQuality string

	var stream *beatport.TrackStream
	var download *beatport.TrackDownload

	switch app.config.Quality {
	case "medium-hls":
		trackStream, err := inst.StreamTrack(track.ID)
		if err != nil {
			return "", err
		}
		fileExtension = ".m4a"
		displayQuality = "AAC 128kbps - HLS"
		stream = trackStream
	default:
		trackDownload, err := inst.DownloadTrack(track.ID, quality)
		if err != nil {
			return "", err
		}
		switch trackDownload.StreamQuality {
		case ".128k.aac.mp4":
			fileExtension = ".m4a"
			displayQuality = "AAC 128kbps"
		case ".256k.aac.mp4":
			fileExtension = ".m4a"
			displayQuality = "AAC 256kbps"
		case ".flac":
			fileExtension = ".flac"
			displayQuality = "FLAC"
		default:
			return "", fmt.Errorf("invalid stream quality: %s", trackDownload.StreamQuality)
		}
		download = trackDownload
	}

	// Handle path templates: if template contains "/", split into directory path and filename
	template := app.config.TrackFileTemplate
	var fileTemplate string
	var pathTemplate string

	if lastSlashIndex := strings.LastIndex(template, "/"); lastSlashIndex >= 0 {
		pathTemplate = template[:lastSlashIndex]
		fileTemplate = template[lastSlashIndex+1:]
	} else {
		fileTemplate = template
	}

	// Create directory structure if path template exists
	if pathTemplate != "" {
		// Parse the path template to get directory names
		pathName := track.Filename(
			beatport.NamingPreferences{
				Template:           pathTemplate,
				Whitespace:         app.config.WhitespaceCharacter,
				ArtistsLimit:       app.config.ArtistsLimit,
				ArtistsShortForm:   app.config.ArtistsShortForm,
				TrackNumberPadding: app.config.TrackNumberPadding,
				KeySystem:          app.config.KeySystem,
				AsciiOnly:          app.config.AsciiOnlyFileNames,
			},
		)

		// Create the full directory path
		fullPath := filepath.Join(directory, pathName)
		if err := CreateDirectory(fullPath); err != nil {
			return "", fmt.Errorf("failed to create directory %s: %w", fullPath, err)
		}
		directory = fullPath
	}

	fileName := track.Filename(
		beatport.NamingPreferences{
			Template:           fileTemplate,
			Whitespace:         app.config.WhitespaceCharacter,
			ArtistsLimit:       app.config.ArtistsLimit,
			ArtistsShortForm:   app.config.ArtistsShortForm,
			TrackNumberPadding: app.config.TrackNumberPadding,
			KeySystem:          app.config.KeySystem,
			AsciiOnly:          app.config.AsciiOnlyFileNames,
		},
	)
	filePath := fmt.Sprintf("%s/%s%s", directory, fileName, fileExtension)
	if _, err := os.Stat(filePath); err == nil {
		app.activeFilesMutex.RLock()
		_, exists := app.activeFiles[filePath]
		app.activeFilesMutex.RUnlock()

		if exists {
			i := 1
			for {
				filePath = fmt.Sprintf("%s/%s (%d)%s", directory, fileName, i, fileExtension)
				if _, err := os.Stat(filePath); os.IsNotExist(err) {
					break
				}
				i++
			}
		} else {
			switch app.config.TrackExists {
			case "skip":
				return "", nil
			case "update":
				app.infoLogWrapper(track.StoreUrl(), "updating tags")
				return filePath, nil
			case "error":
				return "", ErrTrackFileExists
			}
		}
	}

	app.activeFilesMutex.Lock()
	app.activeFiles[filePath] = struct{}{}
	app.activeFilesMutex.Unlock()

	var prefix string
	infoDisplay := fmt.Sprintf("%s (%s) [%s]", track.Name.String(), track.MixName.String(), displayQuality)
	if app.config.ShowProgress {
		prefix = infoDisplay
	} else {
		fmt.Println("Downloading " + infoDisplay)
	}

	if download != nil {
		if err := app.downloadFile(download.Location, filePath, prefix); err != nil {
			os.Remove(filePath)
			return "", err
		}
	} else if stream != nil {
		segments, key, err := getStreamSegments(stream.Url)
		if err != nil {
			return "", fmt.Errorf("get stream segments: %v", err)
		}
		segmentsFile, err := app.downloadSegments(directory, *segments, *key, prefix)
		defer os.Remove(segmentsFile)
		if err != nil {
			return "", fmt.Errorf("download segments: %v", err)
		}
		if err := remuxToM4A(segmentsFile, filePath); err != nil {
			os.Remove(filePath)
			return "", fmt.Errorf("remux to m4a: %v", err)
		}
	}

	if !app.config.ShowProgress {
		fmt.Printf("Finished downloading %s\n", infoDisplay)
	}

	return filePath, nil
}

const (
	rawTagSuffix = "_raw"
)

func (app *application) tagTrack(location string, track *beatport.Track, coverPath string) error {
	fileExt := filepath.Ext(location)
	if !app.config.FixTags {
		return nil
	}
	file, err := taglib.Read(location)
	if err != nil {
		return err
	}
	defer file.Close()

	subgenre := ""
	if track.Subgenre != nil {
		subgenre = track.Subgenre.Name
	}
	mappingValues := map[string]string{
		"track_id":       strconv.Itoa(int(track.ID)),
		"track_url":      track.StoreUrl(),
		"track_name":     fmt.Sprintf("%s (%s)", track.Name.String(), track.MixName.String()),
		"track_artists":  track.Artists.Display(0, ""),
		"track_remixers": track.Remixers.Display(0, ""),
		"track_artists_limited": track.Artists.Display(
			app.config.ArtistsLimit,
			app.config.ArtistsShortForm,
		),
		"track_remixers_limited": track.Remixers.Display(
			app.config.ArtistsLimit,
			app.config.ArtistsShortForm,
		),
		"track_number":              strconv.Itoa(track.Number),
		"track_number_with_padding": beatport.NumberWithPadding(track.Number, track.Release.TrackCount, app.config.TrackNumberPadding),
		"track_number_with_total":   fmt.Sprintf("%d/%d", track.Number, track.Release.TrackCount),
		"track_genre":               track.Genre.Name,
		"track_subgenre":            subgenre,
		"track_genre_with_subgenre": track.GenreWithSubgenre("|"),
		"track_subgenre_or_genre":   track.SubgenreOrGenre(),
		"track_key":                 track.Key.Display(app.config.KeySystem),
		"track_bpm":                 strconv.Itoa(track.BPM),
		"track_isrc":                track.ISRC,

		"release_id":   strconv.Itoa(int(track.Release.ID)),
		"release_url":  track.Release.StoreUrl(),
		"release_name": track.Release.Name.String(),
		"release_artists": track.Release.Artists.Display(
			0,
			"",
		),
		"release_remixers": track.Release.Remixers.Display(
			0,
			"",
		),
		"release_artists_limited": track.Release.Artists.Display(
			app.config.ArtistsLimit,
			app.config.ArtistsShortForm,
		),
		"release_remixers_limited": track.Release.Remixers.Display(
			app.config.ArtistsLimit,
			app.config.ArtistsShortForm,
		),
		"release_date":        track.Release.Date,
		"release_year":        track.Release.Year(),
		"release_track_count": strconv.Itoa(track.Release.TrackCount),
		"release_track_count_with_padding": beatport.NumberWithPadding(
			track.Release.TrackCount, track.Release.TrackCount, app.config.TrackNumberPadding,
		),
		"release_catalog_number": track.Release.CatalogNumber.String(),
		"release_upc":            track.Release.UPC,
		"release_label":          track.Release.Label.Name,
		"release_label_url":      track.Release.Label.StoreUrl(),
	}

	if fileExt == ".m4a" {
		if err = file.StripMp4(); err != nil {
			return err
		}
	} else {
		existingTags, err := file.PropertyKeys()
		if err != nil {
			return fmt.Errorf("read existing tags: %v", err)
		}

		for _, tag := range existingTags {
			file.SetProperty(tag, nil)
		}
	}

	if fileExt == ".flac" {
		for field, property := range app.config.TagMappings["flac"] {
			value := mappingValues[field]
			if value != "" {
				file.SetProperty(property, &value)
			}
		}
	} else if fileExt == ".m4a" {
		rawTags := make(map[string]string)

		for field, property := range app.config.TagMappings["m4a"] {
			if strings.HasSuffix(property, rawTagSuffix) {
				if mappingValues[field] != "" {
					property = strings.TrimSuffix(property, rawTagSuffix)
					rawTags[property] = mappingValues[field]
				}
			} else {
				value := mappingValues[field]
				if value != "" {
					file.SetProperty(property, &value)
				}
			}
		}

		for tag, value := range rawTags {
			file.SetItemMp4(tag, value)
		}
	}

	if coverPath != "" && (app.config.CoverSize != config.DefaultCoverSize || fileExt == ".m4a") {
		data, err := os.ReadFile(coverPath)
		if err != nil {
			return err
		}
		picture := taglib.Picture{
			MimeType:    "image/jpeg",
			PictureType: "Front",
			Description: "Cover",
			Data:        data,
			Size:        uint(len(data)),
		}
		if err := file.SetPicture(&picture); err != nil {
			return err
		}
	}

	if err = file.Save(); err != nil {
		return err
	}

	return nil
}

func (app *application) handleTrack(inst *beatport.Beatport, track *beatport.Track, downloadsDir string, coverPath string) (string, error) {
	location, err := app.saveTrack(inst, track, downloadsDir, app.config.Quality)
	if err != nil {
		return "", fmt.Errorf("save track: %v", err)
	}
	if err = app.tagTrack(location, track, coverPath); err != nil && location != "" {
		return "", fmt.Errorf("tag track: %v", err)
	}
	return location, nil
}

func (app *application) cleanup(downloadsDir string) {
	if downloadsDir != app.config.DownloadsDirectory {
		os.Remove(downloadsDir)
	}
}

func ForPaginated[T any](
	entityId int64,
	params string,
	fetchPage func(id int64, page int, params string) (results *beatport.Paginated[T], err error),
	processItem func(item T, i int) error,
) error {
	page := 1
	for {
		paginated, err := fetchPage(entityId, page, params)
		if err != nil {
			return fmt.Errorf("fetch page: %w", err)
		}

		for i, item := range paginated.Results {
			if err := processItem(item, i); err != nil {
				return fmt.Errorf("process item: %w", err)
			}
		}

		if paginated.Next == nil {
			break
		}
		page++
	}
	return nil
}

func (app *application) handleUrl(url string) {
	link, err := app.bp.ParseUrl(url)
	if err != nil {
		app.errorLogWrapper(url, "parse url", err)
		return
	}

	var inst *beatport.Beatport
	switch link.Store {
	case beatport.StoreBeatport:
		inst = app.bp
	case beatport.StoreBeatsource:
		inst = app.bs
	default:
		app.LogError("handle URL", ErrUnsupportedLinkStore)
		return
	}

	switch link.Type {
	case beatport.TrackLink:
		app.handleTrackLink(inst, link)
	case beatport.ReleaseLink:
		app.handleReleaseLink(inst, link)
	case beatport.PlaylistLink:
		app.handlePlaylistLink(inst, link)
	case beatport.ChartLink:
		app.handleChartLink(inst, link)
	case beatport.LabelLink:
		app.handleLabelLink(inst, link)
	case beatport.ArtistLink:
		app.handleArtistLink(inst, link)
	case beatport.Top100Link:
		app.handleTop100Link(inst, link)
	case beatport.CollectionLink:
		app.handleCollectionLink(inst, link)
	default:
		app.LogError("handle URL", ErrUnsupportedLinkType)
	}
}

func (app *application) handleTrackLink(inst *beatport.Beatport, link *beatport.Link) {
	track, err := inst.GetTrack(link.ID)
	if err != nil {
		app.errorLogWrapper(link.Original, "fetch track", err)
		return
	}

	release, err := inst.GetRelease(track.Release.ID)
	if err != nil {
		app.errorLogWrapper(link.Original, "fetch track release", err)
		return
	}
	track.Release = *release

	downloadsDir, err := app.setupDownloadsDirectory(app.config.DownloadsDirectory, release)
	if err != nil {
		app.errorLogWrapper(link.Original, "setup downloads directory", err)
		return
	}

	wg := sync.WaitGroup{}
	app.downloadWorker(&wg, func() {
		var cover string
		if app.requireCover(true, true) {
			cover, err = app.downloadCover(track.Release.Image, downloadsDir)
			if err != nil {
				app.errorLogWrapper(link.Original, "download track release cover", err)
			}
		}

		_, err := app.handleTrack(inst, track, downloadsDir, cover)
		if err != nil {
			app.errorLogWrapper(link.Original, "handle track", err)
			os.Remove(cover)
			return
		}

		if err := app.handleCoverFile(cover); err != nil {
			app.errorLogWrapper(link.Original, "handle cover file", err)
			return
		}
	})
	wg.Wait()

	app.cleanup(downloadsDir)
}

func (app *application) handleReleaseLink(inst *beatport.Beatport, link *beatport.Link) {
	release, err := inst.GetRelease(link.ID)
	if err != nil {
		app.errorLogWrapper(link.Original, "fetch release", err)
		return
	}

	downloadsDir, err := app.setupDownloadsDirectory(app.config.DownloadsDirectory, release)
	if err != nil {
		app.errorLogWrapper(link.Original, "setup downloads directory", err)
		return
	}

	var cover string
	if app.requireCover(true, true) {
		app.semAcquire(app.downloadSem)
		cover, err = app.downloadCover(release.Image, downloadsDir)
		if err != nil {
			app.errorLogWrapper(link.Original, "download release cover", err)
		}
		app.semRelease(app.downloadSem)
	}

	wg := sync.WaitGroup{}
	// Track paths for playlist creation
	var trackPaths []string
	var trackPathsMutex sync.Mutex

	for _, trackUrl := range release.TrackUrls {
		app.downloadWorker(&wg, func() {
			trackLink, err := inst.ParseUrl(trackUrl)
			if err != nil {
				app.errorLogWrapper(link.Original, "parse track url", err)
				return
			}

			track, err := inst.GetTrack(trackLink.ID)
			if err != nil {
				app.errorLogWrapper(trackUrl, "fetch release track", err)
				return
			}
			trackStoreUrl := track.StoreUrl()
			track.Release = *release

			filePath, err := app.handleTrack(inst, track, downloadsDir, cover)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "handle track", err)
				return
			}

			// If track was successfully downloaded, add it to the playlist tracks
			if filePath != "" {
				trackPathsMutex.Lock()
				trackPaths = append(trackPaths, filePath)
				trackPathsMutex.Unlock()
			}
		})
	}
	wg.Wait()

	if err := app.handleCoverFile(cover); err != nil {
		app.errorLogWrapper(link.Original, "handle cover file", err)
		return
	}

	// Create playlist file if enabled and we have multiple tracks
	if len(trackPaths) > 1 {
		playlistName := release.DirectoryName(
			beatport.NamingPreferences{
				Template:           app.config.ReleaseDirectoryTemplate,
				Whitespace:         app.config.WhitespaceCharacter,
				ArtistsLimit:       app.config.ArtistsLimit,
				ArtistsShortForm:   app.config.ArtistsShortForm,
				TrackNumberPadding: app.config.TrackNumberPadding,
				AsciiOnly:          app.config.AsciiOnlyFileNames,
			},
		)

		if err := app.createM3U8Playlist(downloadsDir, playlistName, trackPaths); err != nil {
			app.errorLogWrapper(link.Original, "create playlist file", err)
		}
	}

	app.cleanup(downloadsDir)
}

func (app *application) handlePlaylistLink(inst *beatport.Beatport, link *beatport.Link) {
	playlist, err := inst.GetPlaylist(link.ID)
	if err != nil {
		app.errorLogWrapper(link.Original, "fetch playlist", err)
		return
	}

	downloadsDir, err := app.setupDownloadsDirectory(app.config.DownloadsDirectory, playlist)
	if err != nil {
		app.errorLogWrapper(link.Original, "setup downloads directory", err)
		return
	}

	wg := sync.WaitGroup{}
	// Track paths for playlist creation
	var trackPaths []string
	var trackPathsMutex sync.Mutex

	err = ForPaginated[beatport.PlaylistItem](link.ID, "", inst.GetPlaylistItems, func(item beatport.PlaylistItem, i int) error {
		app.downloadWorker(&wg, func() {
			trackStoreUrl := item.Track.StoreUrl()

			release, err := inst.GetRelease(item.Track.Release.ID)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "fetch track release", err)
				return
			}
			item.Track.Release = *release

			trackDownloadsDir := downloadsDir
			trackFull, err := inst.GetTrack(item.Track.ID)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "fetch full track", err)
				return
			}
			item.Track.Number = trackFull.Number
			if app.config.SortByContext && app.config.ForceReleaseDirectories {
				trackDownloadsDir, err = app.setupDownloadsDirectory(downloadsDir, release)
				if err != nil {
					app.errorLogWrapper(trackStoreUrl, "setup track release directory", err)
					return
				}
			}

			var cover string
			if app.requireCover(true, app.config.ForceReleaseDirectories) {
				cover, err = app.downloadCover(item.Track.Release.Image, trackDownloadsDir)
				if err != nil {
					app.errorLogWrapper(trackStoreUrl, "download track release cover", err)
				} else if !app.config.ForceReleaseDirectories {
					defer os.Remove(cover)
				}
			}

			filePath, err := app.handleTrack(inst, &item.Track, trackDownloadsDir, cover)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "handle track", err)
				os.Remove(cover)
				app.cleanup(trackDownloadsDir)
				return
			}

			// If track was successfully downloaded, add it to the playlist tracks
			if filePath != "" {
				trackPathsMutex.Lock()
				trackPaths = append(trackPaths, filePath)
				trackPathsMutex.Unlock()
			}

			if app.config.ForceReleaseDirectories {
				if err := app.handleCoverFile(cover); err != nil {
					app.errorLogWrapper(trackStoreUrl, "handle track release cover file", err)
					return
				}
			}

			app.cleanup(trackDownloadsDir)
		})
		return nil
	})

	if err != nil {
		app.errorLogWrapper(link.Original, "handle playlist items", err)
		return
	}

	wg.Wait()

	// Create playlist file if enabled and we have multiple tracks
	if len(trackPaths) > 1 {
		playlistName := playlist.DirectoryName(
			beatport.NamingPreferences{
				Template:           app.config.PlaylistDirectoryTemplate,
				Whitespace:         app.config.WhitespaceCharacter,
				TrackNumberPadding: app.config.TrackNumberPadding,
				AsciiOnly:          app.config.AsciiOnlyFileNames,
			},
		)

		if err := app.createM3U8Playlist(downloadsDir, playlistName, trackPaths); err != nil {
			app.errorLogWrapper(link.Original, "create playlist file", err)
		}
	}
}

func (app *application) handleChartLink(inst *beatport.Beatport, link *beatport.Link) {
	chart, err := inst.GetChart(link.ID)
	if err != nil {
		app.errorLogWrapper(link.Original, "fetch chart", err)
		return
	}

	downloadsDir, err := app.setupDownloadsDirectory(app.config.DownloadsDirectory, chart)
	if err != nil {
		app.errorLogWrapper(link.Original, "setup downloads directory", err)
		return
	}
	wg := sync.WaitGroup{}

	if app.requireCover(false, true) {
		app.downloadWorker(&wg, func() {
			cover, err := app.downloadCover(chart.Image, downloadsDir)
			if err != nil {
				app.errorLogWrapper(link.Original, "download chart cover", err)
			}
			if err := app.handleCoverFile(cover); err != nil {
				app.errorLogWrapper(link.Original, "handle cover file", err)
				return
			}
		})
	}

	// Track paths for playlist creation
	var trackPaths []string
	var trackPathsMutex sync.Mutex

	err = ForPaginated[beatport.Track](link.ID, "", inst.GetChartTracks, func(track beatport.Track, i int) error {
		app.downloadWorker(&wg, func() {
			trackStoreUrl := track.StoreUrl()

			release, err := inst.GetRelease(track.Release.ID)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "fetch track release", err)
				return
			}
			track.Release = *release

			trackDownloadsDir := downloadsDir
			trackFull, err := inst.GetTrack(track.ID)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "fetch full track", err)
				return
			}
			track.Number = trackFull.Number
			if app.config.SortByContext && app.config.ForceReleaseDirectories {
				trackDownloadsDir, err = app.setupDownloadsDirectory(downloadsDir, release)
				if err != nil {
					app.errorLogWrapper(trackStoreUrl, "setup track release directory", err)
					return
				}
			}

			var cover string
			if app.requireCover(true, app.config.ForceReleaseDirectories) {
				cover, err = app.downloadCover(track.Release.Image, trackDownloadsDir)
				if err != nil {
					app.errorLogWrapper(trackStoreUrl, "download track release cover", err)
				} else if !app.config.ForceReleaseDirectories {
					defer os.Remove(cover)
				}
			}

			filePath, err := app.handleTrack(inst, &track, trackDownloadsDir, cover)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "handle track", err)
				os.Remove(cover)
				app.cleanup(trackDownloadsDir)
				return
			}

			// If track was successfully downloaded, add it to the playlist tracks
			if filePath != "" {
				trackPathsMutex.Lock()
				trackPaths = append(trackPaths, filePath)
				trackPathsMutex.Unlock()
			}

			if app.config.ForceReleaseDirectories {
				if err := app.handleCoverFile(cover); err != nil {
					app.errorLogWrapper(trackStoreUrl, "handle track release cover file", err)
					return
				}
			}

			app.cleanup(trackDownloadsDir)
		})
		return nil
	})

	if err != nil {
		app.errorLogWrapper(link.Original, "handle chart items", err)
		return
	}

	wg.Wait()

	// Create playlist file if enabled and we have multiple tracks
	if len(trackPaths) > 1 {
		chartName := chart.DirectoryName(
			beatport.NamingPreferences{
				Template:           app.config.ChartDirectoryTemplate,
				Whitespace:         app.config.WhitespaceCharacter,
				TrackNumberPadding: app.config.TrackNumberPadding,
				AsciiOnly:          app.config.AsciiOnlyFileNames,
			},
		)

		if err := app.createM3U8Playlist(downloadsDir, chartName, trackPaths); err != nil {
			app.errorLogWrapper(link.Original, "create playlist file", err)
		}
	}
}

func (app *application) handleLabelLink(inst *beatport.Beatport, link *beatport.Link) {
	label, err := inst.GetLabel(link.ID)
	if err != nil {
		app.errorLogWrapper(link.Original, "fetch label", err)
		return
	}

	downloadsDir, err := app.setupDownloadsDirectory(app.config.DownloadsDirectory, label)
	if err != nil {
		app.errorLogWrapper(link.Original, "setup downloads directory", err)
		return
	}

	err = ForPaginated[beatport.Release](link.ID, link.Params, inst.GetLabelReleases, func(release beatport.Release, i int) error {
		app.globalWorker(func() {
			releaseStoreUrl := release.StoreUrl()
			releaseDir, err := app.setupDownloadsDirectory(downloadsDir, &release)
			if err != nil {
				app.errorLogWrapper(releaseStoreUrl, "setup release downloads directory", err)
				return
			}

			var cover string
			if app.requireCover(true, true) {
				app.semAcquire(app.downloadSem)
				cover, err = app.downloadCover(release.Image, releaseDir)
				if err != nil {
					app.errorLogWrapper(releaseStoreUrl, "download release cover", err)
				}
				app.semRelease(app.downloadSem)
			}

			// Track paths for playlist creation
			var trackPaths []string
			var trackPathsMutex sync.Mutex
			trackWg := sync.WaitGroup{}

			for _, trackUrl := range release.TrackUrls {
				app.downloadWorker(&trackWg, func() {
					trackLink, err := inst.ParseUrl(trackUrl)
					if err != nil {
						app.errorLogWrapper(releaseStoreUrl, "parse track url", err)
						return
					}

					track, err := inst.GetTrack(trackLink.ID)
					if err != nil {
						app.errorLogWrapper(trackUrl, "fetch release track", err)
						return
					}

					trackStoreUrl := track.StoreUrl()
					track.Release = release

					filePath, err := app.handleTrack(inst, track, releaseDir, cover)
					if err != nil {
						app.errorLogWrapper(trackStoreUrl, "handle track", err)
						return
					}

					// If track was successfully downloaded, add it to the playlist tracks
					if filePath != "" {
						trackPathsMutex.Lock()
						trackPaths = append(trackPaths, filePath)
						trackPathsMutex.Unlock()
					}
				})
			}
			trackWg.Wait()

			if err := app.handleCoverFile(cover); err != nil {
				app.errorLogWrapper(releaseStoreUrl, "handle cover file", err)
				return
			}

			// Create playlist file if enabled and we have multiple tracks
			if len(trackPaths) > 1 {
				releaseName := release.DirectoryName(
					beatport.NamingPreferences{
						Template:           app.config.ReleaseDirectoryTemplate,
						Whitespace:         app.config.WhitespaceCharacter,
						ArtistsLimit:       app.config.ArtistsLimit,
						ArtistsShortForm:   app.config.ArtistsShortForm,
						TrackNumberPadding: app.config.TrackNumberPadding,
						AsciiOnly:          app.config.AsciiOnlyFileNames,
					},
				)

				if err := app.createM3U8Playlist(releaseDir, releaseName, trackPaths); err != nil {
					app.errorLogWrapper(releaseStoreUrl, "create playlist file", err)
				}
			}

			app.cleanup(releaseDir)
		})
		return nil
	})
	if err != nil {
		app.errorLogWrapper(link.Original, "fetch label releases", err)
		return
	}
}

func (app *application) handleArtistLink(inst *beatport.Beatport, link *beatport.Link) {
	artist, err := inst.GetArtist(link.ID)
	if err != nil {
		app.errorLogWrapper(link.Original, "fetch artist", err)
		return
	}

	downloadsDir, err := app.setupDownloadsDirectory(app.config.DownloadsDirectory, artist)
	if err != nil {
		app.errorLogWrapper(link.Original, "setup downloads directory", err)
		return
	}

	err = ForPaginated[beatport.Track](link.ID, "", inst.GetArtistTracks, func(track beatport.Track, i int) error {
		app.downloadWorker(&app.wg, func() {
			trackStoreUrl := track.StoreUrl()

			release, err := inst.GetRelease(track.Release.ID)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "fetch track release", err)
				return
			}
			track.Release = *release

			trackDownloadsDir := downloadsDir
			trackFull, err := inst.GetTrack(track.ID)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "fetch full track", err)
				return
			}
			track.Number = trackFull.Number
			if app.config.SortByContext && app.config.ForceReleaseDirectories {
				trackDownloadsDir, err = app.setupDownloadsDirectory(downloadsDir, release)
				if err != nil {
					app.errorLogWrapper(trackStoreUrl, "setup track release directory", err)
					return
				}
			}

			var cover string
			if app.requireCover(true, app.config.ForceReleaseDirectories) {
				cover, err = app.downloadCover(track.Release.Image, trackDownloadsDir)
				if err != nil {
					app.errorLogWrapper(trackStoreUrl, "download track release cover", err)
				} else if !app.config.ForceReleaseDirectories {
					defer os.Remove(cover)
				}
			}

			_, err = app.handleTrack(inst, &track, trackDownloadsDir, cover)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "handle track", err)
				os.Remove(cover)
				app.cleanup(trackDownloadsDir)
				return
			}

			if app.config.ForceReleaseDirectories {
				if err := app.handleCoverFile(cover); err != nil {
					app.errorLogWrapper(trackStoreUrl, "handle track release cover file", err)
					return
				}
			}

			app.cleanup(trackDownloadsDir)
		})
		return nil
	})

	if err != nil {
		app.errorLogWrapper(link.Original, "fetch artist tracks", err)
		return
	}
}

func (app *application) handleTop100Link(inst *beatport.Beatport, link *beatport.Link) {
	top100, err := inst.GetTop100FromLink(app.ctx, link)
	if err != nil {
		app.errorLogWrapper(link.Original, "fetch top-100", err)
		return
	}

	// Create a directory name based on the top100 info
	var folderName string
	if top100.GenreID > 0 {
		folderName = fmt.Sprintf("Top 100 %s", top100.GenreName)
	} else {
		folderName = "Top 100"
	}

	// Create a directory for the top-100 downloads
	downloadsDir, err := app.createDirectory(app.config.DownloadsDirectory, folderName)
	if err != nil {
		app.errorLogWrapper(link.Original, "create top-100 directory", err)
		return
	}

	// Track paths for playlist creation
	var trackPaths []string
	var trackPathsMutex sync.Mutex
	wg := sync.WaitGroup{}

	// Process each track in the top 100 list
	for i, track := range top100.Tracks {
		app.downloadWorker(&wg, func() {
			trackStoreUrl := track.StoreUrl()

			// We need the full track details
			trackFull, err := inst.GetTrack(track.ID)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "fetch full track", err)
				return
			}

			// Set the track number to its position in the list
			trackFull.Number = i + 1

			// Get the release details
			release, err := inst.GetRelease(trackFull.Release.ID)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "fetch track release", err)
				return
			}
			trackFull.Release = *release

			trackDownloadsDir := downloadsDir
			if app.config.SortByContext && app.config.ForceReleaseDirectories {
				trackDownloadsDir, err = app.setupDownloadsDirectory(downloadsDir, release)
				if err != nil {
					app.errorLogWrapper(trackStoreUrl, "setup track release directory", err)
					return
				}
			}

			var cover string
			if app.requireCover(true, app.config.ForceReleaseDirectories) {
				cover, err = app.downloadCover(trackFull.Release.Image, trackDownloadsDir)
				if err != nil {
					app.errorLogWrapper(trackStoreUrl, "download track release cover", err)
				} else if !app.config.ForceReleaseDirectories {
					defer os.Remove(cover)
				}
			}

			filePath, err := app.handleTrack(inst, trackFull, trackDownloadsDir, cover)
			if err != nil {
				app.errorLogWrapper(trackStoreUrl, "handle track", err)
				os.Remove(cover)
				app.cleanup(trackDownloadsDir)
				return
			}

			// If track was successfully downloaded, add it to the playlist tracks
			if filePath != "" {
				trackPathsMutex.Lock()
				trackPaths = append(trackPaths, filePath)
				trackPathsMutex.Unlock()
			}

			if app.config.ForceReleaseDirectories {
				if err := app.handleCoverFile(cover); err != nil {
					app.errorLogWrapper(trackStoreUrl, "handle track release cover file", err)
					return
				}
			}

			app.cleanup(trackDownloadsDir)
		})
	}

	wg.Wait()

	// Create playlist file if enabled and we have multiple tracks
	if len(trackPaths) > 1 {
		if err := app.createM3U8Playlist(downloadsDir, folderName, trackPaths); err != nil {
			app.errorLogWrapper(link.Original, "create playlist file", err)
		}
	}
}

func (app *application) handleCollectionLink(inst *beatport.Beatport, link *beatport.Link) {
	// Initialize Chrome browser for authentication
	if err := inst.InitializeChromeBrowser(false); err != nil { // visible mode
		app.errorLogWrapper(link.Original, "initialize Chrome browser", err)
		return
	}
	defer inst.CloseChrome()

	// Use Chrome browser automation to download collection page and extract track links
	collectionURL := beatport.BeatportMainUrl + beatport.BeatportMyCollection
	trackLinks, err := inst.DownloadPageWithAuth(collectionURL, app.config, app.configPath)
	if err != nil {
		app.errorLogWrapper(link.Original, "download collection page with Chrome", err)
		return
	}

	if len(trackLinks) == 0 {
		app.LogInfo("No tracks found in collection")
		return
	}

	app.LogInfo(fmt.Sprintf("Found %d tracks in collection", len(trackLinks)))

	// Create collection directory
	collectionName := "My Collection"
	downloadsDir, err := app.createDirectory(app.config.DownloadsDirectory, collectionName)
	if err != nil {
		app.errorLogWrapper(link.Original, "create collection directory", err)
		return
	}

	wg := sync.WaitGroup{}
	// Track paths for playlist creation
	var trackPaths []string
	var trackPathsMutex sync.Mutex

	// Process each track link found in the collection
	for i, trackURL := range trackLinks {
		trackNum := i + 1

		app.downloadWorker(&wg, func() {
			// Parse the track link to get track ID
			trackLink, err := inst.ParseUrl(trackURL)
			if err != nil {
				app.errorLogWrapper(trackURL, "parse track URL", err)
				return
			}

			if trackLink.Type != beatport.TrackLink {
				app.errorLogWrapper(trackURL, "skip non-track URL", nil)
				return
			}

			// Get full track details
			trackFull, err := inst.GetTrack(trackLink.ID)
			if err != nil {
				app.errorLogWrapper(trackURL, "fetch full track", err)
				return
			}

			// Set track number based on position in collection
			trackFull.Number = trackNum

			// Get the release details
			release, err := inst.GetRelease(trackFull.Release.ID)
			if err != nil {
				app.errorLogWrapper(trackURL, "fetch track release", err)
				return
			}
			trackFull.Release = *release

			trackDownloadsDir := downloadsDir
			if app.config.SortByContext && app.config.ForceReleaseDirectories {
				trackDownloadsDir, err = app.setupDownloadsDirectory(downloadsDir, release)
				if err != nil {
					app.errorLogWrapper(trackURL, "setup track release directory", err)
					return
				}
			}

			var cover string
			if app.requireCover(true, app.config.ForceReleaseDirectories) {
				cover, err = app.downloadCover(trackFull.Release.Image, trackDownloadsDir)
				if err != nil {
					app.errorLogWrapper(trackURL, "download track release cover", err)
				} else if !app.config.ForceReleaseDirectories {
					defer os.Remove(cover)
				}
			}

			filePath, err := app.handleTrack(inst, trackFull, trackDownloadsDir, cover)
			if err != nil {
				app.errorLogWrapper(trackURL, "handle track", err)
				os.Remove(cover)
				app.cleanup(trackDownloadsDir)
				return
			}

			// If track was successfully downloaded, add it to the playlist tracks
			if filePath != "" {
				trackPathsMutex.Lock()
				trackPaths = append(trackPaths, filePath)
				trackPathsMutex.Unlock()
			}

			if app.config.ForceReleaseDirectories {
				if err := app.handleCoverFile(cover); err != nil {
					app.errorLogWrapper(trackURL, "handle track release cover file", err)
					return
				}
			}

			app.cleanup(trackDownloadsDir)
		})
	}

	wg.Wait()

	// Create playlist file if enabled and we have multiple tracks
	if len(trackPaths) > 1 {
		if err := app.createM3U8Playlist(downloadsDir, collectionName, trackPaths); err != nil {
			app.errorLogWrapper(link.Original, "create playlist file", err)
		}
	}

	app.cleanup(downloadsDir)
}
