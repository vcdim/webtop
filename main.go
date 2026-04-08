package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"time"

	"github.com/vcdim/portview/internal/server"
)

//go:embed web
var webFS embed.FS

func main() {
	port := flag.Int("port", 8080, "HTTP listen port")
	interval := flag.Duration("interval", 2*time.Second, "Data refresh interval")
	flag.Parse()

	webContent, err := fs.Sub(webFS, "web")
	if err != nil {
		log.Fatalf("failed to load embedded web files: %v", err)
	}

	addr := fmt.Sprintf(":%d", *port)
	srv := server.New(addr, *interval, webContent)

	log.Printf("Starting portview on http://localhost:%d", *port)
	if err := srv.Start(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
