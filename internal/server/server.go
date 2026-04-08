package server

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vcdim/portview/internal/collector"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type Server struct {
	addr     string
	interval time.Duration
	webFS    fs.FS
	clients  map[*websocket.Conn]struct{}
	mu       sync.Mutex
}

func New(addr string, interval time.Duration, webFS fs.FS) *Server {
	return &Server{
		addr:     addr,
		interval: interval,
		webFS:    webFS,
		clients:  make(map[*websocket.Conn]struct{}),
	}
}

func (s *Server) Start() error {
	http.Handle("/", http.FileServer(http.FS(s.webFS)))
	http.HandleFunc("/ws", s.handleWS)

	go s.broadcastLoop()

	log.Printf("portview listening on %s", s.addr)
	return http.ListenAndServe(s.addr, nil)
}

func (s *Server) handleWS(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	s.mu.Lock()
	s.clients[conn] = struct{}{}
	s.mu.Unlock()

	// Send initial snapshot
	s.sendSnapshot(conn)

	// Keep connection alive; remove on close
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			s.mu.Lock()
			delete(s.clients, conn)
			s.mu.Unlock()
			conn.Close()
			return
		}
	}
}

func (s *Server) broadcastLoop() {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for range ticker.C {
		data := s.collectJSON()
		if data == nil {
			continue
		}
		s.mu.Lock()
		for conn := range s.clients {
			if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
				conn.Close()
				delete(s.clients, conn)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) sendSnapshot(conn *websocket.Conn) {
	data := s.collectJSON()
	if data != nil {
		conn.WriteMessage(websocket.TextMessage, data)
	}
}

func (s *Server) collectJSON() []byte {
	entries, err := collector.Collect()
	if err != nil {
		log.Printf("collect error: %v", err)
		return nil
	}
	data, err := json.Marshal(entries)
	if err != nil {
		log.Printf("json marshal error: %v", err)
		return nil
	}
	return data
}
