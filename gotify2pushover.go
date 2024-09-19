package main

import (
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/gregdel/pushover"
)

type GotifyMessage struct {
	Id       int       `json:"id"`
	Appid    int       `json:"appid"`
	Message  string    `json:"message"`
	Title    string    `json:"title"`
	Priority int       `json:"priority"`
	Date     time.Time `json:"date"`
}

func main() {
	gotify_host := os.Getenv("GOTIFY_HOST")
	gotify_client_token := os.Getenv("GOTIFY_CLIENT_TOKEN")

	var addr = flag.String("addr", gotify_host, "http service address")

	flag.Parse()
	log.SetFlags(0)

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	u := url.URL{Scheme: "ws", Host: *addr, Path: "/stream"}
	log.Printf("connecting to %s", u.String())

	c, _, err := websocket.DefaultDialer.Dial(u.String(), http.Header{"X-Gotify-Key": []string{gotify_client_token}})
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer c.Close()

	done := make(chan struct{})

	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				return
			}
			log.Printf("recv: %s", message)
			gotify_message := GotifyMessage{}
			json.Unmarshal(message, &gotify_message)

			sendMenssageToPushover(gotify_message)
		}
	}()

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			err := c.WriteMessage(websocket.TextMessage, []byte(t.String()))
			if err != nil {
				log.Println("write:", err)
				return
			}
		case <-interrupt:
			log.Println("interrupt")

			// Cleanly close the connection by sending a close message and then
			// waiting (with timeout) for the server to close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Println("write close:", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}

func sendMenssageToPushover(gotify_message GotifyMessage) {
	pushover_user_token := os.Getenv("PUSHOVER_USER_TOKEN")
	pushover_application_token := os.Getenv("PUSHOVER_APPLICATION_TOKEN")

	app := pushover.New(pushover_application_token)
	recipient := pushover.NewRecipient(pushover_user_token)

	message := pushover.NewMessage(gotify_message.Message)

	response, err := app.SendMessage(message, recipient)
	if err != nil {
		log.Panic(err)
	}

	log.Println(response)
}
