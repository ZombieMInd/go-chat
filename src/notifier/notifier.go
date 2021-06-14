package notifier

import (
	"log"

	"github.com/ZombieMInd/go-chat/v0.1/src/db"
	"github.com/gorilla/websocket"
)

type NotificationSession struct {
	receiver string
	peer     *websocket.Conn
}

func NewNotificationSession(receiver string, peer *websocket.Conn) *NotificationSession {
	return &NotificationSession{receiver: receiver, peer: peer}
}

func (s *NotificationSession) Start() {

	go func() {
		db.StartNotifySubscriber(s.receiver, s.peer)

		// for {
		// 	_, msg, err := s.peer.ReadMessage()
		// 	if err != nil {
		// 		_, ok := err.(*websocket.CloseError)
		// 		if ok {
		// 			log.Println("connection closed by user")
		// 			s.disconnect()
		// 		}
		// 		return
		// 	}
		// 	s.notifyPeer(string(msg))
		// }
	}()
}

func (s *NotificationSession) notifyPeer(msg string) {
	err := s.peer.WriteMessage(websocket.TextMessage, []byte(msg))
	if err != nil {
		log.Println("failed to write message", err)
	}
}

func (s *NotificationSession) disconnect() {
	s.peer.Close()

	//remove from Peers
	delete(db.Peers, s.receiver)
}
