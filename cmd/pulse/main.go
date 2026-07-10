package main

import (
	"flag"
	"log"

	"github.com/emgeorrk/pulse/internal/app"
)

func main() {
	once := flag.Bool("once", false, "напечатать один кадр метрик в stdout и выйти (проверка сенсоров без UI)")
	flag.Parse()

	run := app.Run
	if *once {
		run = app.RunOnce
	}
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
