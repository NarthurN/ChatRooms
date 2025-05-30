package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Hub struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	mutex      sync.Mutex
}

func newHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (h *Hub) run() {
	for {
		select {
		case client := <-h.register:
			h.mutex.Lock()
			h.clients[client] = true
			h.mutex.Unlock()
		case client := <-h.unregister:
			h.mutex.Lock()
			if _, ok := h.clients[client]; ok {
				close(client.send)
				delete(h.clients, client)
			}
			h.mutex.Unlock()
		case message := <-h.broadcast:
			h.mutex.Lock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mutex.Unlock()
		}
	}
}

func (h *Hub) handleConnections(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Ошибка обновления: %v", err)
		return
	}

	client := &Client{conn: ws, send: make(chan []byte)}
	h.register <- client

	defer func() {
		h.unregister <- client
		ws.Close()
	}()

	go func() {
		for message := range client.send {
			err := client.conn.WriteMessage(websocket.TextMessage, message)
			if err != nil {
				log.Printf("Ошибка отправки: %v", err)
				return
			}
		}
	}()

	for {
		_, msg, err := ws.ReadMessage()
		if err != nil {
			log.Printf("Ошибка чтения: %v", err)
			break
		}
		h.broadcast <- msg
		fmt.Printf("Получено: %s\n", msg)
	}
}

func main() {
	hub := newHub()
	go hub.run()

	http.HandleFunc("/ws", hub.handleConnections)
	log.Println("Сервер запущен на :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("Ошибка сервера: ", err)
	}
}