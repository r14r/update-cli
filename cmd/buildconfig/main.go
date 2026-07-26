package main

import (
	"flag"
	"fmt"
	"os"

	"release-updater/lib/buildconfig"
)

func main() {
	path := flag.String("config", "build-config.json", "Pfad zur Build-Konfiguration")
	field := flag.String("field", "", "ein einzelnes Feld ausgeben")
	validate := flag.Bool("validate", false, "Build-Konfiguration nur validieren")
	expand := flag.Bool("expand", false, "Pfadvariablen wie $HOME auflösen")
	flag.Parse()

	cfg, err := buildconfig.Load(*path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *validate {
		fmt.Println("build-config.json ist gültig")
		return
	}
	printValue := func(value string) {
		if *expand {
			expanded, err := buildconfig.ExpandPath(value)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			value = expanded
		}
		fmt.Println(value)
	}
	switch *field {
	case "defaultDownloadFolder":
		printValue(cfg.DefaultDownloadFolder)
	case "defaultDeploymentPath":
		printValue(cfg.DefaultDeploymentPath)
	case "defaultConfigPath":
		printValue(cfg.DefaultConfigPath)
	case "":
		fmt.Printf("defaultDownloadFolder=%s\ndefaultDeploymentPath=%s\ndefaultConfigPath=%s\n", cfg.DefaultDownloadFolder, cfg.DefaultDeploymentPath, cfg.DefaultConfigPath)
	default:
		fmt.Fprintf(os.Stderr, "unbekanntes Feld %q\n", *field)
		os.Exit(2)
	}
}
