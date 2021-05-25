package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ZombieMInd/go-chat/v0.1/src/chat"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// var redisHost string
// var redisPassword string

const port = "8080"

func init() {
	// redisHost = os.Getenv("REDIS_HOST")
	// if redisHost == "" {
	// 	log.Fatal("missing REDIS_HOST env var")
	// }

	// redisPassword = os.Getenv("REDIS_PASSWORD")
	// if redisPassword == "" {
	// 	log.Fatal("missing REDIS_PASSWORD env var")
	// }
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/history/{sender}/{receiver}", historyHandler)
	r.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("you are good to go!"))
	})
	r.Handle("/chat/{sender}/{receiver}", http.HandlerFunc(websocketHandler))

	// http.Handle("/chat/connect/", http.HandlerFunc(websocketHandler))
	server := http.Server{Addr: ":" + port, Handler: r}
	go func() {
		// err := server.ListenAndServeTLS("fullchain.pem", "privkey.pem")
		err := server.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal("failed to start server", err)
		}
	}()

	exit := make(chan os.Signal)
	signal.Notify(exit, syscall.SIGTERM, syscall.SIGINT)
	<-exit

	log.Println("exit signalled")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	chat.Cleanup()
	server.Shutdown(ctx)

	log.Println("chat app exited")
}

func websocketHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	peer, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		log.Fatal("websocket conn failed: ", err)
	}

	chatSession := chat.NewChatSession(vars["sender"], vars["receiver"], peer)
	chatSession.Start()
}

func historyHandler(rw http.ResponseWriter, req *http.Request) {
	chatID := "chat0"
	result := chat.GetDialogHistory(chatID, 0, 3)
	for i, s := range result {
		rw.Write([]byte(s))
		rw.Write([]byte("\n"))
		log.Println(i, s)
	}
}
