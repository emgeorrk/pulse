package main

import (
	"flag"
	"log"

	"github.com/emgeorrk/pulse/internal/app"
)

func main() {
	once := flag.Bool("once", false, "print one metrics frame to stdout and exit (sensor check without UI)")

	flag.Parse()

	run := app.Run
	if *once {
		run = app.RunOnce
	}

	if err := run(); err != nil {
		log.Fatal(err)
	}
}
