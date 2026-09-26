package main

import (
	"embed"
	"flag"
	"fmt"
	"os"
)

//go:embed payload/*
var payloadFS embed.FS

func main() {
	uninstallFlag := flag.Bool("uninstall", false, "Uninstall Local Archive from this computer")
	silentFlag := flag.Bool("silent", false, "Run without displaying message dialogs")
	flag.Parse()

	if *uninstallFlag {
		if err := runUninstall(*silentFlag); err != nil {
			fmt.Printf("Uninstall error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if err := runInstall(*silentFlag); err != nil {
		fmt.Printf("Install error: %v\n", err)
		os.Exit(1)
	}
}
