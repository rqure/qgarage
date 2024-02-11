package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"

	"github.com/gorilla/websocket"
	qmq "github.com/rqure/qmq/src"
)

type WebService struct {
	clients      map[*websocket.Conn]struct{}
	clientsMutex sync.Mutex
	app          *qmq.QMQApplication
}

func NewWebService() *WebService {
	return &WebService{
		clients: make(map[*websocket.Conn]struct{}),
		app:     qmq.NewQMQApplication("garage"),
	}
}

func (w *WebService) Initialize() {
	w.app.Initialize()

	// Serve static files from the "static" directory
	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("./web/css"))))
	http.Handle("/img/", http.StripPrefix("/img/", http.FileServer(http.Dir("./web/img"))))
	http.Handle("/js/", http.StripPrefix("/js/", http.FileServer(http.Dir("./web/js"))))

	// Handle WebSocket and other routes
	http.Handle("/", w)

	go func() {
		err := http.ListenAndServe("0.0.0.0:20000", nil)
		if err != nil {
			w.app.Logger().Panic(fmt.Sprintf("HTTP server error: %v", err))
		}
	}()
}

func (w *WebService) Deinitialize() {
	w.app.Deinitialize()
}

func (w *WebService) Tick() {

}

func (w *WebService) ServeHTTP(wr http.ResponseWriter, req *http.Request) {
	if req.URL.Path == "/" {
		w.onIndexRequest(wr, req)
	} else if req.URL.Path == "/ws" {
		w.onWSRequest(wr, req)
	} else {
		http.NotFound(wr, req)
	}
}

func (w *WebService) onIndexRequest(wr http.ResponseWriter, req *http.Request) {
	index, err := os.ReadFile("web/index.html")

	if err != nil {
		w.app.Logger().Error(fmt.Sprintf("Error while reading file for path '/': %v", err))
		return
	}

	wr.Header().Set("Content-Type", "text/html")
	wr.Write(index)
}

func (w *WebService) onWSRequest(wr http.ResponseWriter, req *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}

	conn, err := upgrader.Upgrade(wr, req, nil)
	if err != nil {
		w.app.Logger().Error(fmt.Sprintf("Error upgrading to WebSocket: %v", err))
		return
	}
	defer conn.Close()

	w.addClient(conn)

	for {
		messageType, p, err := conn.ReadMessage()
		if err != nil {
			w.app.Logger().Error(fmt.Sprintf("Error reading WebSocket message: %v", err))
			break
		}
		if messageType == websocket.TextMessage {
			var data map[string]interface{}
			if err := json.Unmarshal(p, &data); err != nil {
				w.app.Logger().Error(fmt.Sprintf("Error decoding WebSocket message: %v", err))
				continue
			}
			if cmd, ok := data["cmd"].(string); ok && cmd == "get" {
				// response := w.schema.GetDatetime().AsJSON()
				// if err := conn.WriteMessage(websocket.TextMessage, []byte(response)); err != nil {
				// 	w.app.Logger().Error(fmt.Sprintf("Error sending WebSocket message: %v", err))
				// 	break
				// }
			}
		}
	}
	w.removeClient(conn)
}

func (w *WebService) addClient(conn *websocket.Conn) {
	w.clientsMutex.Lock()
	defer w.clientsMutex.Unlock()
	w.clients[conn] = struct{}{}
}

func (w *WebService) removeClient(conn *websocket.Conn) {
	w.clientsMutex.Lock()
	defer w.clientsMutex.Unlock()
	delete(w.clients, conn)
}

func (w *WebService) notifyClients(data interface{}) {
	w.clientsMutex.Lock()
	defer w.clientsMutex.Unlock()
	for conn := range w.clients {
		err := conn.WriteJSON(data)
		if err != nil {
			w.app.Logger().Error(fmt.Sprintf("Error sending WebSocket message: %v", err))
		}
	}
}