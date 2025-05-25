package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"unspok3n/beatportdl/config"
	"unspok3n/beatportdl/internal/beatport"

	"github.com/fatih/color"
	"github.com/vbauerster/mpb/v8"
)

const (
	configFilename = "beatportdl-config.yml"
	cacheFilename  = "beatportdl-credentials.json"
	errorFilename  = "beatportdl-err.log"
)

type application struct {
	config      *config.AppConfig
	logFile     *os.File
	logWriter   io.Writer
	ctx         context.Context
	wg          sync.WaitGroup
	downloadSem chan struct{}
	globalSem   chan struct{}
	pbp         *mpb.Progress

	urls             []string
	activeFiles      map[string]struct{}
	activeFilesMutex sync.RWMutex

	bp *beatport.Beatport
	bs *beatport.Beatport
}

// printConfig prints the current configuration values
func printConfig(cfg *config.AppConfig) {
	boldBlue := color.New(color.FgBlue, color.Bold)
	boldGreen := color.New(color.FgGreen, color.Bold)

	boldBlue.Println("\n━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	boldBlue.Println("📋 Current Configuration:")
	boldBlue.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")

	fmt.Printf("Username: %s\n", cfg.Username)
	fmt.Printf("Downloads Directory: %s\n", cfg.DownloadsDirectory)
	fmt.Printf("Quality: %s\n", cfg.Quality)
	fmt.Printf("Show Progress: %t\n", cfg.ShowProgress)
	fmt.Printf("Write Error Log: %t\n", cfg.WriteErrorLog)

	fmt.Printf("Max Download Workers: %d\n", cfg.MaxDownloadWorkers)
	fmt.Printf("Max Global Workers: %d\n", cfg.MaxGlobalWorkers)

	fmt.Printf("Sort By Context: %t\n", cfg.SortByContext)
	fmt.Printf("Sort By Label: %t\n", cfg.SortByLabel)
	fmt.Printf("Force Release Directories: %t\n", cfg.ForceReleaseDirectories)
	fmt.Printf("Track Exists Behavior: %s\n", cfg.TrackExists)
	fmt.Printf("Track Number Padding: %d\n", cfg.TrackNumberPadding)
	fmt.Printf("Create M3U8 Playlist: %t\n", cfg.CreateM3U8Playlist)

	fmt.Printf("Cover Size: %s\n", cfg.CoverSize)
	fmt.Printf("Keep Cover: %t\n", cfg.KeepCover)
	fmt.Printf("Fix Tags: %t\n", cfg.FixTags)

	if cfg.Proxy != "" {
		fmt.Printf("Using Proxy: %s\n", cfg.Proxy)
	}

	boldGreen.Println("━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━")
	fmt.Println()
}

func main() {
	// Define command line flags
	configDir := flag.String("config-dir", "", "Directory to load configuration files from (beatportdl-config.yml and beatportdl-credentials.json)")
	quitFlag := flag.Bool("q", false, "Quit the main loop after finishing")
	createPlaylistFlag := flag.Bool("playlist", false, "Create an m3u8 playlist file when multiple tracks are downloaded from a single URL")

	flag.Parse()
	inputArgs := flag.Args()

	// Check if the playlist flag was explicitly provided
	createPlaylistFlagSet := false
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "playlist" {
			createPlaylistFlagSet = true
		}
	})

	cfg, cachePath, err := Setup(*configDir)
	if err != nil {
		fmt.Println(err.Error())
		Pause()
	}

	// Only override the CreateM3U8Playlist config option if the flag is explicitly provided
	if createPlaylistFlagSet {
		cfg.CreateM3U8Playlist = *createPlaylistFlag
	}

	// Print the configuration values
	printConfig(cfg)

	ctx, cancel := context.WithCancel(context.Background())

	app := &application{
		config:      cfg,
		downloadSem: make(chan struct{}, cfg.MaxDownloadWorkers),
		globalSem:   make(chan struct{}, cfg.MaxGlobalWorkers),
		ctx:         ctx,
		logWriter:   os.Stdout,
	}

	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		<-sigCh

		if len(app.urls) > 0 {
			app.LogInfo("Shutdown signal received. Waiting for download workers to finish")
			cancel()

			<-sigCh
		}

		os.Exit(0)
	}()

	if cfg.WriteErrorLog {
		logFilePath, _, err := FindErrorLogFile(*configDir)
		if err != nil {
			fmt.Println(err.Error())
			Pause()
		}
		f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0600)
		if err != nil {
			panic(err)
		}
		app.logFile = f
		defer f.Close()
	}

	auth := beatport.NewAuth(cfg.Username, cfg.Password, cachePath)
	bp := beatport.New(beatport.StoreBeatport, cfg.Proxy, auth)
	bs := beatport.New(beatport.StoreBeatsource, cfg.Proxy, auth)

	if err := auth.LoadCache(); err != nil {
		if err := auth.Init(bp); err != nil {
			app.FatalError("beatport", err)
		}
	}

	app.bp = bp
	app.bs = bs

	for _, arg := range inputArgs {
		if strings.HasSuffix(arg, ".txt") {
			app.parseTextFile(arg)
		} else {
			app.urls = append(app.urls, arg)
		}
	}

	for {
		if len(app.urls) == 0 {
			app.mainPrompt()
		}

		app.pbp = mpb.New(mpb.WithAutoRefresh(), mpb.WithOutput(color.Output))
		app.logWriter = app.pbp
		app.activeFiles = make(map[string]struct{}, len(app.urls))

		for _, url := range app.urls {
			app.globalWorker(func() {
				app.handleUrl(url)
			})
		}

		app.wg.Wait()
		app.pbp.Shutdown()

		if *quitFlag || ctx.Err() != nil {
			break
		}

		app.urls = []string{}
	}
}
