package room

import (
	"log"
	"sync"
)

type Message struct {
	Author  string
	Content string
}

type Client struct {
	ID       string
	NickName string
	Send     chan Message
}

type Room struct {
	Code    string
	Clients map[*Client]bool
	Join    chan *Client
	Leave   chan *Client
	Msg     chan Message
	mu      sync.Mutex
}

func NewRoom(code string) *Room {
	return &Room{
		Code:    code,
		Clients: make(map[*Client]bool),
		Join:    make(chan *Client),
		Leave:   make(chan *Client),
		Msg:     make(chan Message),
	}
}

func (r *Room) Run() {
	for {
		select {
		case client := <-r.Join:
			log.Println(client.NickName, "someone joined!")
			r.mu.Lock()
			r.Clients[client] = true
			r.mu.Unlock()

		case client := <-r.Leave:
			log.Println("someone leaved!")
			r.mu.Lock()
			delete(r.Clients, client)
			close(client.Send)
			r.mu.Unlock()
		case msg := <-r.Msg:
			log.Println("someone sent message!")
			for client := range r.Clients {
				client.Send <- msg
			}
		}
	}
}
