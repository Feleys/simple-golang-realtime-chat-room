package chat

import (
	"chatroom/internal/room"
	"log"
	"net/http"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func HandleConnection(w http.ResponseWriter, r *http.Request) {
	roomCode := r.URL.Query().Get("roomCode")
	rInstance, _ := room.GetRoom(roomCode)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "Failed to upgrade connection", http.StatusInternalServerError)
		return
	}

	client := &room.Client{
		ID:       conn.RemoteAddr().String(),
		NickName: "User_" + conn.RemoteAddr().String(),
		Send:     make(chan room.Message),
	}
	rInstance.Join <- client

	go readPump(client, conn, rInstance)
	go writePump(client, conn)
}

func readPump(client *room.Client, conn *websocket.Conn, r *room.Room) {
	defer func() {
		r.Leave <- client
		conn.Close()
	}()

	for {
		var msg room.Message
		err := conn.ReadJSON(&msg)
		if err != nil {
			break
		}
		r.Msg <- msg
	}
}

func writePump(client *room.Client, conn *websocket.Conn) {
	defer conn.Close()

	for message := range client.Send {
		log.Println(message)
		err := conn.WriteJSON(message)
		if err != nil {
			break
		}
	}
}
