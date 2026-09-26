package handlers

import (
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

var Clients = make(map[*websocket.Conn]bool)
var ClientsMutex = sync.Mutex{}
var Broadcast = make(chan interface{})

// HandleWebSocket obsługuje endpoint GET /api/v1/ws
// @Summary      Nawiąż połączenie WebSocket
// @Description  Endpoint służący do nasłuchiwania w czasie rzeczywistym na nowe zdarzenia (logi reklam).
// @Tags         websockets
// @Router       /ws [get]
func HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("Błąd krytyczny podczas nawiązywania połączenia WS: %v", err)
		return
	}
	defer func(ws *websocket.Conn) {
		err := ws.Close()
		if err != nil {

		}
	}(ws)

	ClientsMutex.Lock()
	Clients[ws] = true
	ClientsMutex.Unlock()

	log.Println("Nowy klient podłączony do strumienia WebSocket!")

	for {
		_, _, err := ws.ReadMessage()
		if err != nil {
			ClientsMutex.Lock()
			delete(Clients, ws)
			ClientsMutex.Unlock()
			log.Println("Klient rozłączony.")
			break
		}
	}
}

func HandleMessages() {
	for {
		msg := <-Broadcast

		ClientsMutex.Lock()
		for client := range Clients {
			err := client.WriteJSON(msg)
			if err != nil {
				log.Printf("Błąd wysyłania do klienta (odłączanie): %v", err)
				err := client.Close()
				if err != nil {
					return
				}
				delete(Clients, client)
			}
		}
		ClientsMutex.Unlock()
	}
}
