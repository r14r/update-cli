package main

import (
	"flag"
	"fmt"
	"github.com/r14r/update-cli/lib/buildconfig"
	"os"
)

func main() {
	validate := flag.Bool("validate", false, "")
	field := flag.String("field", "", "")
	expand := flag.Bool("expand", false, "")
	flag.Parse()
	c, err := buildconfig.Load("build-config.json")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *validate {
		return
	}
	if *field != "" {
		var v string
		switch *field {
		case "defaultDownloadFolder":
			v = c.DefaultDownloadFolder
		case "defaultDeploymentPath":
			v = c.DefaultDeploymentPath
		case "defaultConfigPath":
			v = c.DefaultConfigPath
		default:
			fmt.Fprintln(os.Stderr, "unknown field")
			os.Exit(2)
		}
		if *expand {
			v, err = buildconfig.ExpandPath(v)
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
		}
		fmt.Println(v)
		return
	}
	fmt.Printf("download=%s\ndeployment=%s\nconfig=%s\n", c.DefaultDownloadFolder, c.DefaultDeploymentPath, c.DefaultConfigPath)
}
