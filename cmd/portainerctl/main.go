package main

import (
	"log"
	"os"

	"portainerctl/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
}
