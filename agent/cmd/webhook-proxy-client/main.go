package main

import (
	"flag"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var (
	verbose        bool
	url            string
	subscriptionID string
)

type SubscribeMessage struct {
	Action         string `json:"action"`
	SubscriptionID string `json:"subscriptionId"`
}

type ReceivedMessage struct {
	Action         string `json:"action"`
	SubscriptionID string `json:"subscriptionId"`
	Body           string `json:"body"`
}

func main() {
	flag.Parse()

	log := logrus.New()
	if verbose {
		log.Level = logrus.DebugLevel
		log.Debug("Logging level set to debug")
	}

	if subscriptionID == "" {
		log.Fatal("subscriptionId must be set")
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	con, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Fatal("Connection failed: ", err)
	}

	defer con.Close()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			message := ReceivedMessage{}
			err := con.ReadJSON(&message)
			if err != nil {
				log.Error("ReadMessage failed: ", err)
				return
			}

			log.Info("ReadMessage: ", message)
		}
	}()

	log.Info("WebSocket connected")

	log.Info("Sending subscription request")
	subscriptionRequestMessage := SubscribeMessage{
		Action:         "subscribe",
		SubscriptionID: subscriptionID,
	}
	err = con.WriteJSON(subscriptionRequestMessage)
	if err != nil {
		log.Fatal("Sending subscription request message: ", err)
	}

	for {
		select {
		case <-done:
			return
		case <-interrupt:
			log.Info("Shutting down")
			err = con.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Error("WriteMessage (closing) failed: ", err)
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				return
			}
		}
	}
}

func init() {
	flag.BoolVar(&verbose, "verbose", false, "Verbose logging")
	flag.StringVar(&url, "url", "wss://localhost", "WebSocket URL of webhook-proxy cloud component")
	flag.StringVar(&subscriptionID, "subscriptionId", "", "Unique subscriptionId")
}
