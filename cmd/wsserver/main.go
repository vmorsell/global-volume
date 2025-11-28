package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

const (
	defaultPort        = "8080"
	randomIDByteLength = 8
	readBufferSize     = 1024
	writeBufferSize    = 1024
	writeWait          = 10 * time.Second
	pongWait           = 60 * time.Second
	pingPeriod         = (pongWait * 9) / 10
	maxMessageSize     = 1024
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  readBufferSize,
		WriteBufferSize: writeBufferSize,
		CheckOrigin:     checkOrigin,
	}
)

type client struct {
	conn   *websocket.Conn
	id     string
	send   chan []byte
	server *server
}

type server struct {
	clients    map[*client]struct{}
	clientsMu  sync.RWMutex
	broadcast  chan []byte
	register   chan *client
	unregister chan *client
}

func newServer() *server {
	return &server{
		clients:    make(map[*client]struct{}),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *client),
		unregister: make(chan *client),
	}
}

func (s *server) run() {
	for {
		select {
		case client := <-s.register:
			s.clientsMu.Lock()
			s.clients[client] = struct{}{}
			count := len(s.clients)
			s.clientsMu.Unlock()
			log.Printf("[INFO] Client connected: %s. Total clients: %d", client.id, count)
			s.broadcastClientCount()

		case client := <-s.unregister:
			s.clientsMu.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
				count := len(s.clients)
				log.Printf("[INFO] Client disconnected: %s. Total clients: %d", client.id, count)
			}
			s.clientsMu.Unlock()
			s.broadcastClientCount()

		case message := <-s.broadcast:
			s.clientsMu.RLock()
			for client := range s.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(s.clients, client)
				}
			}
			s.clientsMu.RUnlock()
		}
	}
}

func (s *server) broadcastClientCount() {
	s.clientsMu.RLock()
	count := len(s.clients)
	s.clientsMu.RUnlock()

	msg := []byte(fmt.Sprintf(`{"type":"clients","clients":%d}`, count))
	s.broadcast <- msg
}

func (c *client) readPump() {
	defer func() {
		c.server.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("[WARN] WebSocket error: %v", err)
			}
			break
		}

		if len(message) > maxMessageSize {
			log.Printf("[WARN] Message too large from %s: %d bytes (max: %d)", c.id, len(message), maxMessageSize)
			c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseMessageTooBig, "message too large"))
			break
		}

		log.Printf("[VOLUME] Received update from %s: %s", c.id, string(message))

		c.server.clientsMu.RLock()
		broadcastCount := 0
		for other := range c.server.clients {
			if other == c {
				continue
			}
			select {
			case other.send <- message:
				broadcastCount++
			default:
				close(other.send)
				delete(c.server.clients, other)
			}
		}
		totalClients := len(c.server.clients)
		c.server.clientsMu.RUnlock()

		log.Printf("[VOLUME] Broadcasted update from %s to %d clients (total clients: %d)", c.id, broadcastCount, totalClients)
	}
}

func (c *client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func wsHandler(s *server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("[ERROR] Failed to upgrade connection: %v", err)
			return
		}

		clientID, err := generateClientID()
		if err != nil {
			log.Printf("[ERROR] Failed to generate client ID: %v", err)
			conn.Close()
			return
		}

		client := &client{
			conn:   conn,
			id:     clientID,
			send:   make(chan []byte, 256),
			server: s,
		}

		s.register <- client

		go client.writePump()
		go client.readPump()
	}
}

func generateClientID() (string, error) {
	b := make([]byte, randomIDByteLength)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate random ID: %w", err)
	}
	return hex.EncodeToString(b), nil
}

func checkOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	return true
}

func getPort() string {
	port := os.Getenv("PORT")
	if port == "" {
		return defaultPort
	}
	return port
}

func main() {
	port := getPort()
	s := newServer()

	go s.run()

	http.HandleFunc("/ws", wsHandler(s))

	server := &http.Server{
		Addr:         ":" + port,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("[INFO] Volume relay server started on :%s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("[FATAL] Server failed to start: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("[INFO] Shutting down server...")
	log.Println("[INFO] Server exited")
}
