### Intro
We'll understand how GO lang handles multi-threading using goroutine when creating real-time chat.

### WebSocket
WebSocket is a protocol and technology used for real-time bidirectional communication in web application. Unlike the traditional HTTP request-response model, WebSocket allows the establishment of presisten connections between clients and servers, enabling immediate sending and receiving of message whenever either party has new data without the need for HTTP requests.

In this case, we'll be using [gorilla/websocket](https://github.com/gorilla/websocket) to implement WebSocket functionality, with a focus on the chat feature.
A goroutine is a lightweight thread or coroutine in go languague, and it is a core part of Go's concurrency model, allowing you to execute functions or code blocks concurrently in a program without the need to explicitly manage threads or process.

### Feature require
 - Real-time message receive and send.
 - The room can be create by client
 - Client can join specific room with room's code

### File Structrue
 ```
 project/
 |-- cmd/
 |   |-- server/
 |       |-- main.go
 |-- internal/
 |   |-- chat/
 |       |-- chat.go
 |   |-- room/
 |       |-- room.go
 |       |-- managment.go
 ```

#### main.go
Initialize and start the server.

#### chat/
Manage client connections using WebSockets. Handle the reading and writing of messages to and from the server.

#### room/
Manage chat rooms: initialization, and listening to client actions such as joining, leaving, and sending messages.

### Understanding Processes, Goroutines, and WebSocket Interactions in Chat Servers

Each process operates within its own distinct memory space, ensuring that other processes cannot access it. When a program is executed, it spawns a new process. Within this process, several threads can run concurrently, all sharing the memory of that process.

When the WebSocket server starts, it initializes a process to manage server interactions. As a client connects to the server or when a room is created, a goroutine is spawned.

For example, if two clients join the same room:
- **Main process**: Listens for connections and handles service requests.
- **Goroutines**: For each client that connects, two goroutines are spawned; one for writing and another for reading. Additionally, a separate goroutine is initiated when a room is created.

When client connect to the chat room, they are allocated to different goroutines which running in same process.

In summary:
 - 1 main process
 - 5 goroutunes 

### Setup Websocket server

In the main.go file. we've set up a WebSocket server that listens on port 8080.

The feature requires a "room" concept, so when the client sends the request to the server with query string containing "roomCode", the server checks if there is existing process with the same roomCode. If one exist, that process generates new goroutine for the client. If not, the server create the new process with the roomCode which taken from the client's query string, and then allocates a new goroutine for that client.

```
// main.go
func main() {
	http.HandleFunc("/ws", chat.HandleConnection)

	log.Println("Server started on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
```

```
// chat/chat.go
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
```

We get room instance with this code:
```
rInstance, _ := room.GetRoom(roomCode)
```

In the `GetRoom` method, we want to ensure that when checking for existance, a new room isn't being created simultaneously, so we add the lock method.
```
// room/managment.go

func GetRoom(code string) (*Room, bool) {
	mu.Lock()
	r, exists := rooms[code]
	mu.Unlock()

	if exists {
		return r, true
	}

	return createRoom(code)
}
```

### How client interactive with room
When client connect to the server, there're few things we need to do.
1. create a goroutine for read.
2. create a goroutune for write.

```
func HandleConnection(w http.ResponseWriter, r *http.Request) {
    // ...
	go readPump(client, conn, rInstance)
	go writePump(client, conn)
}
```

In the readPump method, we continuously monitor for incoming messages using a for loop. This loop listens for new messages from the client through the conn.ReadJSON(&msg) call. If an error occurs — typically when the client disconnects — the loop breaks as conn returns an error. In cases where no error is encountered, the received message, stored in msg, is sent to the room via the r.Msg channel. The defer statement ensures that when the loop exits (either due to an error or otherwise), the client is removed from the room, and the connection is properly closed.

```
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
```

In the writePump method, we listen for new messages on the client.Send channel using a for loop. This channel is responsible for holding messages that need to be sent to the client. Whenever a new message appears in the channel, it is sent back to the client's WebSocket connection using conn.WriteJSON(message). If any error occurs during this process (usually indicating a connection issue or the client has disconnected), the loop breaks. The defer conn.Close() ensures that the WebSocket connection is properly closed when the function exits.

This function is focused on sending messages to the client and operates independently from the readPump function, which is responsible for receiving messages and sending them to the room.

```
func writePump(client *room.Client, conn *websocket.Conn) {
	defer conn.Close()

	for message := range client.Send {
		err := conn.WriteJSON(message)
		if err != nil {
			break
		}
	}
}
```

### How a room instance handles messages & client actions 

As previously mentioned, creating a room necessitates a goroutine to manage its operations. We've defined several key events that a room instance is responsible for handling:

1. Client joining
2. Client leaving
3. Receiving a message

If a room with the roomCode from the query string does not exist, then a room corresponding to that roomCode will be created.

```
// room/management.go
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
```

In the room instance, we have implemented the events that were just defined.

Here, we use the select statement to handle the I/O operations of channels. It allows us to wait for operations across multiple channels. Additionally, it effectively manages the events we have defined.

```
// room/room.go

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

```

Now, we'll set up three terminal sessions: one as the server and two as clients.

To enter the project and start the server in the first session:
chatroom/cmd/server
Run the following command:

```go run main.go```

In the other two sessions, enter the following commands:

```websocat "ws://127.0.0.1:8080/ws?roomCode=room1"```

Next, input the JSON-formatted string in the client session:

```
{"Author": "client 1", "Content": "content1"}
```

In the other client session, you will receive the same message.
With this, we have completed all the functionality we need.

![server](/assets/images/server.png)
![server](/assets/images/clients.png)


This article introduces an implementation of a real-time chat room based on Go and WebSocket. Initially, we discussed the significance of processes and goroutines in handling client requests and messages. Whenever a client connects to the server via WebSocket, corresponding read and write goroutines are created for communication.

The focus is on the management of Room instances, which handle multiple events through the Run method and select statement, including client joining, leaving, and message receiving. When a new client joins, a new room instance is created, or if it already exists, the client joins the existing room. These room instances are managed by goroutines, ensuring real-time message transmission.

The readPump and writePump methods play a crucial role here. readPump is responsible for receiving messages from clients, while writePump handles sending messages back to clients. This design allows the chat room to handle interactions with multiple clients in real-time and efficiently.

Overall, this chat room implementation showcases the powerful capabilities of Go in handling concurrency and network communications, while also providing a clear, modular approach to managing real-time communication needs.