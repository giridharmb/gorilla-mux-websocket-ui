package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"os"
	"os/signal"
)

// We'll need to define an Upgrader
// this will require a Read and Write buffer size
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
}

/*
https://pkg.go.dev/github.com/gorilla/websocket@v1.4.2#pkg-constants
*/

// data is read from websocket and what ever data is read (which is []byte)
// is put into readChannel
// it is then read , and if it has a ping message
// we put the pong message onto writeChannel
// what ever is put onto writeChannel
// is then written out into the websocket
var readChannel = make(chan []byte, 1)
var writeChannel = make(chan []byte, 1)

func wsEndpoint(w http.ResponseWriter, r *http.Request) {
	upgrader.CheckOrigin = func(r *http.Request) bool {
		return true
	}

	// upgrade this connection to a WebSocket
	// connection
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("(ERROR) : wsEndpoint() : %v", err.Error())
		return
	}

	// helpful log statement to show connections
	log.Printf("Client Connected...")

	stopChan := make(chan struct{})

	go func() {
		for {
			_, dataBytes, err := ws.ReadMessage()
			if err != nil {
				log.Printf("(ERROR) : ws.ReadMessage() : %v", err.Error())
				return
			}
			readChannel <- dataBytes
		}
	}()

	go func() {
		for {
			dataFromReadChannel := <-readChannel
			jsonData := make(map[string]interface{})
			umarshalErr := json.Unmarshal(dataFromReadChannel, &jsonData)
			if umarshalErr != nil {
				log.Printf("dataFromReadChannel := <-readChannel : data sent from browser is not a JSON object")
			} else {
				log.Printf("dataFromReadChannel := <-readChannel : jsonData : %v", jsonData)
				_, ok := jsonData["data"]
				if ok {
					actualData := jsonData["data"]
					_, ok := actualData.(string)
					if ok {
						if actualData == "ping" {
							pongMessage := fmt.Sprintf(`{"data":"pong"}`)
							writeChannel <- []byte(pongMessage)
						}
					}

				}
			}
		}
	}()

	go func() {
		for {
			dataFromWriteChannel := <-writeChannel
			if err := ws.WriteMessage(1, dataFromWriteChannel); err != nil {
				log.Printf("(ERROR) : ws.WriteMessage(1, dataFromWriteChannel) : %v", err.Error())
				return
			}
		}
	}()

	<-stopChan
	//reader(ws)
}

func main() {
	fmt.Printf("\nstarting http server...\n")

	// if we crash, we get the filename and line-number
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	router := mux.NewRouter().StrictSlash(false)

	srv := &http.Server{
		Addr:    ":5000",
		Handler: router,
	}

	staticWebDirectory := "/webui"

	router.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	router.HandleFunc("/ws", wsEndpoint)

	router.PathPrefix("/home").Handler(http.StripPrefix("/home", http.FileServer(http.Dir("."+staticWebDirectory))))

	// Wait for CTRL+C
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	go func() {
		_ = srv.ListenAndServe()
	}()

	// Block until a signal is received.
	<-ch
	log.Printf("Stopping the server...")

	err := srv.Close()
	if err != nil {
		log.Printf("(ERROR) : %v", err.Error())
	}

	err = srv.Shutdown(context.TODO())
	if err != nil {
		log.Printf("(ERROR) : %v", err.Error())
	}
	log.Printf("Stopped the server.")

}
