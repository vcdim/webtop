package server

import (
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/vcdim/webtop/internal/collector"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type message struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
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

	log.Printf("webtop listening on %s", s.addr)
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

	// Send initial data immediately
	s.sendAll(conn)

	// Read loop keeps connection alive; clean up on disconnect
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
		msgs := s.collectAll()
		s.mu.Lock()
		for conn := range s.clients {
			for _, data := range msgs {
				if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
					conn.Close()
					delete(s.clients, conn)
					break
				}
			}
		}
		s.mu.Unlock()
	}
}

func (s *Server) sendAll(conn *websocket.Conn) {
	for _, data := range s.collectAll() {
		if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
			return
		}
	}
}

func (s *Server) collectAll() [][]byte {
	var msgs [][]byte

	if entries, err := collector.Collect(); err == nil {
		if data, err := json.Marshal(message{Type: "ports", Data: entries}); err == nil {
			msgs = append(msgs, data)
		}
	} else {
		log.Printf("port collect error: %v", err)
	}

	if gpuData, err := collector.CollectGPU(); err == nil {
		if data, err := json.Marshal(message{Type: "gpu", Data: gpuData}); err == nil {
			msgs = append(msgs, data)
		}
	}
	// GPU errors are silently ignored (nvidia-smi may not exist)

	return msgs
}
