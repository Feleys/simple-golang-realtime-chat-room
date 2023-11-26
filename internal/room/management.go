package room

import (
	"sync"
)

var mu sync.Mutex

var rooms = make(map[string]*Room)

func GetRoom(code string) (*Room, bool) {
	mu.Lock()
	r, exists := rooms[code]
	mu.Unlock()

	if exists {
		return r, true
	}

	return createRoom(code)
}

func createRoom(code string) (*Room, bool) {
	mu.Lock()
	defer mu.Unlock()

	if r, exists := rooms[code]; exists {
		return r, true
	}

	newRoom := NewRoom(code)
	go newRoom.Run()
	rooms[code] = newRoom
	return newRoom, false
}
