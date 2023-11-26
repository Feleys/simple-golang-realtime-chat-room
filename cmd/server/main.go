package main

import (
	"chatroom/internal/chat"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/ws", chat.HandleConnection)

	log.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
