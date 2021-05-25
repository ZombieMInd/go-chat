package chat

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

func StartSubscriber(dialog string) {
	/*
		this goroutine exits when the application shuts down. When the pusub connection is closed,
		the channel range loop terminates, hence terminating the goroutine
	*/
	go func() {
		log.Println("starting subscriber...")
		sub = client.Subscribe(dialog)
		messages := sub.Channel()
		for message := range messages {
			from := strings.Split(message.Payload, ":")[0]
			//send to all websocket sessions/peers
			for user, peer := range Peers {
				if from != user { //don't recieve your own messages
					peer.WriteMessage(websocket.TextMessage, []byte(message.Payload))
				}
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

type MessageDTO struct {
	Time    time.Time
	User    string
	Message string
}

func SaveMessage(dialog string, user string, msg string) {
	dto := MessageDTO{time.Now(), user, msg}
	jsonDTO, err := json.Marshal(dto)

	if err != nil {
		log.Fatal(err)
	}

	err = client.LPush(dialog, jsonDTO).Err()
	if err != nil {
		log.Fatal(err)
	}
}

func GetDialogHistory(chatID string, offset int64, limit int64) []string {
	res, err := client.LRange(chatID, offset, limit).Result()
	if err != nil {
		log.Fatal(err)
	}
	return res
}

const users = "chat-users"

func FindDialog(sender string, receiver string) (string, error) {
	dialogKey, err := client.Keys(sender + ":" + receiver).Result()
	if err != nil {
		dialogKey, err = client.Keys(receiver + ":" + sender).Result()
		if err != nil {
			return "", err
		}
	}
	if len(dialogKey) > 1 {
		return "", errors.New("More then one dialog found")
	}
	if len(dialogKey) == 0 {
		return sender + ":" + receiver, nil
	}
	return dialogKey[0], nil
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
