package main

import (
	"embed"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"

	"local-archive/internal/database"
	"local-archive/internal/handlers"
	"local-archive/internal/sysutil"
)

//go:embed templates/* static/*
var embeddedFS embed.FS

func main() {
	portFlag := flag.Int("port", 8080, "Port for web server")
	noBrowserFlag := flag.Bool("no-browser", false, "Disable automatically opening web browser")
	installFlag := flag.Bool("install", false, "Install to Windows Startup folder for automatic boot launch")
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall from Windows Startup folder")
	flag.Parse()

	// Handle Windows startup setup or removal if requested
	if *installFlag {
		if err := sysutil.ConfigureWindowsStartup(true); err != nil {
			fmt.Printf("Error installing startup task: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Local Archive has been installed to start automatically on Windows boot.")
		return
	}
	if *uninstallFlag {
		if err := sysutil.ConfigureWindowsStartup(false); err != nil {
			fmt.Printf("Error removing startup task: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Local Archive automatic startup has been removed.")
		return
	}

	dataDir := sysutil.GetDataDir()
	uploadDir := filepath.Join(dataDir, "uploads")
	dbPath := filepath.Join(dataDir, "archive.db")

	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatalf("Could not create data directory at %s: %v", dataDir, err)
	}

	db, err := database.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	tmpl, err := template.ParseFS(embeddedFS, "templates/*.html")
	if err != nil {
		log.Fatalf("Failed to parse HTML templates: %v", err)
	}

	app := handlers.NewApp(db, tmpl, uploadDir, dbPath, *portFlag, !*noBrowserFlag, embeddedFS)
	if err := app.Start(); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}
