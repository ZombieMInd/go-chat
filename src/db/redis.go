package db

import (
	"encoding/json"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
)

var client *redis.Client
var redisHost string
var redisPassword string
var sub *redis.PubSub

// Peers maps a chat user to the websocket connection (pointer)
var Peers map[string]*websocket.Conn
var Dialogs []string

func init() {

	// redisHost = os.Getenv("REDIS_HOST")
	// if redisHost == "" {
	// 	log.Fatal("missing REDIS_HOST env var")
	// }

	// redisPassword = os.Getenv("REDIS_PASSWORD")
	// if redisPassword == "" {
	// 	log.Fatal("missing REDIS_PASSWORD env var")
	// }

	log.Println("connecting to Redis...")
	client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "denis895",
		DB:       0,
	})

	_, err := client.Ping().Result()
	if err != nil {
		log.Fatal("failed to connect to redis", err)
	}
	log.Println("connected to redis", redisHost)

}

func StartSubscriber(dialog string, user string, peer *websocket.Conn) {

	go func() {
		log.Println("starting subscriber...")
		sub = client.Subscribe(dialog)
		messages := sub.Channel()
		for message := range messages {
			from := strings.Split(message.Payload, ":")[0]
			log.Println("message: ", message.Payload)
			if from != user { //don't recieve your own messages
				err := peer.WriteMessage(websocket.TextMessage, []byte(message.Payload))
				if err != nil {
					log.Println("connection closed")
					peer.Close()
					err := sub.Unsubscribe(dialog)
					if err != nil {
						log.Println("failed to unsubscribe redis channel subscription:", err)
					}
					return
				}

			}
		}
	}()
}

func StartNotifySubscriber(user string, peer *websocket.Conn) {

	go func() {
		log.Println("starting notify subscriber...")
		sub = client.Subscribe(user)
		log.Println("user chanel: ", user)
		messages := sub.Channel()
		for message := range messages {
			log.Println("notification: ", "{ \"sender\": \""+strings.Split(message.Payload, ":")[0]+"\", \"message\": \""+strings.Split(message.Payload, ":")[1]+"\"}")
			err := peer.WriteMessage(websocket.TextMessage, []byte("{ \"sender\": \""+strings.Split(message.Payload, ":")[0]+"\", \"message\": \""+strings.Split(message.Payload, ":")[1]+"\"}"))
			if err != nil {
				log.Println("connection closed")
				peer.Close()
				err := sub.Unsubscribe(user)
				if err != nil {
					log.Println("failed to unsubscribe redis channel subscription:", err)
				}
				return
			}

		}
	}()
}

// SendToChannel pusblishes on a redis pubsub channel
func SendToChannel(channel string, msg string) {
	err := client.Publish(channel, msg).Err()
	if err != nil {
		log.Println("could not publish to channel", err)
	}
}

type ChatDTO struct {
	Name     string
	ChatType string
	Members  []string
}

type MessageDTO struct {
	Time    time.Time
	User    string
	Message string
}

type DialogsDTO struct {
	Receiver    string
	LastMessage MessageDTO
}

func SaveMessage(dialog string, user string, msg string) {
	dto := MessageDTO{time.Now(), user, msg}
	jsonDTO, err := json.Marshal(dto)
	if err != nil {
		log.Fatal(err)
	}

	err = client.LPush(dialog+":messages", jsonDTO).Err()
	if err != nil {
		log.Fatal(err)
	}
}

func GetDialogHistory(chatID string, offset int64, limit int64) []string {
	res, err := client.LRange(chatID+":messages", offset, limit).Result()
	if err != nil {
		log.Fatal(err)
	}
	return res
}

func GetDialogs(user string) []DialogsDTO {
	res, err := client.SMembers(user + ":chats").Result()
	if err != nil {
		log.Fatal(err)
	}
	var result []DialogsDTO
	for _, dialog := range res {
		var msgDTO MessageDTO

		lastMessage, err := client.LRange(dialog+":messages", 1, 1).Result()
		if err != nil {
			log.Println(err)
			lastMessage = nil
		}
		if len(lastMessage) > 0 {
			err = json.Unmarshal([]byte(lastMessage[0]), &msgDTO)
			if err != nil {
				log.Println(err)
			}
		} else {
			err = json.Unmarshal([]byte(""), &msgDTO)
			if err != nil {
				log.Println(err)
			}
		}

		receiver := strings.Split(dialog, ":")[1]
		if receiver == user {
			receiver = strings.Split(dialog, ":")[0]
		}
		dto := DialogsDTO{LastMessage: msgDTO, Receiver: receiver}

		result = append(result, dto)
	}
	return result
}

const users = "chat-users"

func FindDialog(sender string, receiver string) (string, error) {
	// TODO change Keys to Scan
	dialogKey, err := client.Keys(sender + ":" + receiver).Result()
	if err != nil {
		return "", err
	}
	if len(dialogKey) > 1 {
		return "", errors.New("more then one dialog found")
	}
	if len(dialogKey) == 0 {
		dialogKey, err = client.Keys(receiver + ":" + sender).Result()
		if err != nil {
			return "", err
		}
		if len(dialogKey) == 0 {
			return "", nil
		}
	}
	return dialogKey[0], nil
}

func CreateDialog(sender string, receiver string) (string, error) {
	members, err := json.Marshal([]string{sender, receiver})
	if err != nil {
		log.Fatal(err)
	}
	dialog := map[string]interface{}{"type": "dialog", "members": members, "is_active": "true"}

	err = client.HMSet(sender+":"+receiver, dialog).Err()
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	err = client.SAdd(sender+":chats", sender+":"+receiver).Err()
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	err = client.SAdd(receiver+":chats", sender+":"+receiver).Err()
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	return sender + ":" + receiver, nil
}

func SubUserToChats(user string, peer *websocket.Conn) {
	dialogs, err := client.SMembers(user + ":chats").Result()
	if err != nil {
		log.Fatal(err)
	}
	for _, dialog := range dialogs {
		StartSubscriber(dialog, user, peer)
	}
}

// CreateUser creates a new user in the SET of active chat users
func CreateUser(user string) error {
	err := client.SAdd(users, user).Err()
	if err != nil {
		return err
	}
	return nil
}

// RemoveUser removes a user from the SET of active chat users
func RemoveUser(user string) {
	err := client.SRem(users, user).Err()
	if err != nil {
		log.Println("failed to remove user:", user)
		return
	}
	log.Println("removed user from redis:", user)
}

// Cleanup is invoked when the app is shutdown - disconnects websocket peers, closes pusb-sub and redis client connection
func Cleanup() {
	for _, peer := range Peers {
		// client.SRem(users, user)
		peer.Close()
	}
	for _, dialog := range Dialogs {
		// client.SRem(users, user)
		err := sub.Unsubscribe(dialog)
		if err != nil {
			log.Println("failed to unsubscribe redis channel subscription:", err)
		}
	}

	log.Println("cleaned up users and sessions...")

	err := sub.Close()
	if err != nil {
		log.Println("failed to close redis channel subscription:", err)
	}

	err = client.Close()
	if err != nil {
		log.Println("failed to close redis connection: ", err)
		return
	}
}
