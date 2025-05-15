package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

type client struct {
	conn *websocket.Conn
	id   string
}

func randomID() string {
	b := make([]byte, 3)
	rand.Read(b)
	return hex.EncodeToString(b)
}

var (
	clientsMu sync.Mutex
	clients   = make(map[*client]struct{})
)

func broadcastUserCount() {
	clientsMu.Lock()
	defer clientsMu.Unlock()
	msg := []byte(fmt.Sprintf(`{"type":"users","count":%d}`, len(clients)))
	for c := range clients {
		c.conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c := &client{conn: conn, id: randomID()}
	clientsMu.Lock()
	clients[c] = struct{}{}
	log.Printf("[INFO] Client connected: %s. Total clients: %d", c.id, len(clients))
	clientsMu.Unlock()
	broadcastUserCount()
	defer func() {
		clientsMu.Lock()
		delete(clients, c)
		log.Printf("[INFO] Client disconnected: %s. Total clients: %d", c.id, len(clients))
		clientsMu.Unlock()
		broadcastUserCount()
		conn.Close()
	}()
	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return
		}
		log.Printf("[VOLUME] Received update from %s: %s", c.id, string(msg))
		// Broadcast to all other clients
		clientsMu.Lock()
		for other := range clients {
			if other != c {
				other.conn.WriteMessage(websocket.TextMessage, msg)
			}
		}
		log.Printf("[VOLUME] Broadcasted update from %s to %d clients", c.id, len(clients)-1)
		clientsMu.Unlock()
	}
}

func main() {
	http.HandleFunc("/ws", wsHandler)
	log.Println("Volume relay server started on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
