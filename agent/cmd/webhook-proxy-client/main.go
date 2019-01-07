package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
)

var (
	verbose        bool
	url            string
	subscriptionID string
	targetUrl      string
)

const (
	pongWait   = 10 * time.Second
	pingPeriod = (pongWait * 9) / 10

	reconnectDelay = 5 * time.Second
)

type SubscribeMessage struct {
	Action         string `json:"action"`
	SubscriptionID string `json:"subscriptionId"`
}

type ReceivedMessage struct {
	Action         string `json:"action"`
	SubscriptionID string `json:"subscriptionId"`
	Body           string `json:"body"`
	ContentType    string `json:"contentType"`
}

func receivedMessage(msg ReceivedMessage, log *logrus.Logger) {
	log.Debug("ReadMessage: ", msg)

	_, err := http.Post(targetUrl, msg.ContentType, strings.NewReader(msg.Body))
	if err != nil {
		log.Error("Sending POST to targetUrl: ", err)
	}
}

func startConnection(log *logrus.Logger, interrupt chan os.Signal) bool {
	shuttingDown := false

	con, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		log.Error("Connection failed: ", err)
		return shuttingDown
	}

	ticker := time.NewTicker(pingPeriod)

	defer func() {
		con.Close()
		ticker.Stop()
	}()

	con.SetReadDeadline(time.Now().Add(pongWait))
	con.SetPongHandler(func(string) error {
		log.Debug("Received keepalive response (pong)")
		con.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

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

			receivedMessage(message, log)

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
		case <-ticker.C:
			log.Debug("Sending keepalive request (ping)")
			if err := con.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				log.Error("Error sending keepalive: ", err)
				return shuttingDown
			}
		case <-done:
			return shuttingDown
		case <-interrupt:
			log.Info("Shutting down")
			shuttingDown = true

			err = con.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				log.Error("WriteMessage (closing) failed: ", err)
				return shuttingDown
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				return shuttingDown
			}
		}
	}

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

	if targetUrl == "" {
		log.Fatal("targerUrl must be set")
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	for {
		log.Info("Starting connection")
		shuttingDown := startConnection(log, interrupt)

		if shuttingDown {
			return
		}

		log.Info(fmt.Sprintf("Waiting %v seconds to reconnect ...", reconnectDelay))
		time.Sleep(reconnectDelay)
	}
}

func init() {
	flag.BoolVar(&verbose, "verbose", false, "Verbose logging")
	flag.StringVar(&url, "url", "wss://localhost", "WebSocket URL of webhook-proxy cloud component")
	flag.StringVar(&subscriptionID, "subscriptionId", "", "Unique subscriptionId")
	flag.StringVar(&targetUrl, "targetUrl", "", "Internal URL where proxied webhook events should be sent")
}
