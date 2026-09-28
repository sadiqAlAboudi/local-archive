package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"local-archive/internal/database"
	"local-archive/internal/sysutil"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/linux"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend
var assets embed.FS

//go:embed build/appicon.png
var appIcon []byte

func main() {
	isDev := os.Getenv("ENV") == "development"
	paths, err := sysutil.ResolveDataPaths(isDev)
	if err != nil {
		log.Fatalf("Failed to initialize storage paths: %v", err)
	}

	db, err := database.Open(paths.DBPath)
	if err != nil {
		log.Fatalf("Failed to open database at %s: %v", paths.DBPath, err)
	}

	app := NewApp(db, paths)

	// Gracefully handle OS interrupt and termination signals to flush WAL before exit
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		if app != nil {
			app.shutdown(context.Background())
		}
		os.Exit(0)
	}()

	// Custom asset handler to stream uploaded files (PDFs, images) securely to the webview
	customFileServer := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/view-file") {
			filename := r.URL.Query().Get("file")
			if filename == "" {
				http.NotFound(w, r)
				return
			}
			cleanPath := filepath.Clean(filepath.Join(paths.UploadDir, filepath.Base(filename)))
			if _, err := os.Stat(cleanPath); os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.ServeFile(w, r, cleanPath)
			return
		}
		http.NotFound(w, r)
	})

	err = wails.Run(&options.App{
		Title:             "الأرشيف المحلي",
		Width:             1240,
		Height:            820,
		MinWidth:          960,
		MinHeight:         640,
		Frameless:         false,
		StartHidden:       false,
		HideWindowOnClose: false,
		BackgroundColour:  &options.RGBA{R: 0, G: 0, B: 0, A: 255},
		AssetServer: &assetserver.Options{
			Assets:  assets,
			Handler: customFileServer,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		Linux: &linux.Options{
			Icon:        appIcon,
			ProgramName: "local-archive",
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		fmt.Printf("Wails runtime error: %v\n", err)
	}

	// Ensure database WAL is flushed and handles closed if wails exits
	app.shutdown(context.Background())
}
