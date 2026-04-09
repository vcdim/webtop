package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"time"

	"github.com/vcdim/webtop/internal/server"
)

//go:embed web
var webFS embed.FS

func main() {
	port := flag.Int("port", 9999, "HTTP listen port")
	flag.IntVar(port, "p", 9999, "HTTP listen port (shorthand)")
	interval := flag.Duration("interval", 2*time.Second, "Data refresh interval")
	flag.DurationVar(interval, "i", 2*time.Second, "Data refresh interval (shorthand)")
	flag.Parse()

	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("failed to load embedded web files: %v", err)
	}

	addr := fmt.Sprintf(":%d", *port)
	srv := server.New(addr, *interval, webContent)

	log.Printf("Starting webtop on http://localhost:%d", *port)
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
