package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/ZombieMInd/go-chat/v0.1/src/chat"
	"github.com/ZombieMInd/go-chat/v0.1/src/db"
	"github.com/ZombieMInd/go-chat/v0.1/src/notifier"
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
	r.HandleFunc("/history/{sender}/{receiver}", historyHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/dialogs/{user}", dialogsHandler).Methods("GET", "OPTIONS")
	r.HandleFunc("/", func(rw http.ResponseWriter, req *http.Request) {
		rw.Write([]byte("you are good to go!"))
	})
	r.Handle("/chat/{sender}/{receiver}", http.HandlerFunc(websocketHandler))
	r.Handle("/notify/{receiver}", http.HandlerFunc(notifierHandler))

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

	db.Cleanup()
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

func notifierHandler(rw http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	peer, err := upgrader.Upgrade(rw, req, nil)
	if err != nil {
		log.Fatal("websocket conn failed: ", err)
	}

	notifierSession := notifier.NewNotificationSession(vars["receiver"], peer)
	notifierSession.Start()
}

func historyHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Access-Control-Allow-Origin", "*")

	rw.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	offset := 0
	limit := 20
	vars := mux.Vars(req)
	offsetVal, found := mux.Vars(req)["offset"]
	if found {
		offset, _ = strconv.Atoi(offsetVal)
	}
	limitVal, found := mux.Vars(req)["limit"]
	if found {
		limit, _ = strconv.Atoi(limitVal)
	}
	dialog, err := db.FindDialog(vars["sender"], vars["receiver"])
	if err != nil {
		log.Fatal(err)
	}
	if dialog == "" {
		http.Error(rw, "Dialog not found", 404)
		return
	}
	result := db.GetDialogHistory(dialog, int64(offset), int64(limit))
	json.NewEncoder(rw).Encode(result)
}

func dialogsHandler(rw http.ResponseWriter, req *http.Request) {
	rw.Header().Set("Access-Control-Allow-Origin", "*")

	rw.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	vars := mux.Vars(req)

	user := vars["user"]
	result := db.GetDialogs(user)
	log.Println("result", result)
	err := json.NewEncoder(rw).Encode(result)
	if err != nil {
		log.Fatal(err)
	}
}
