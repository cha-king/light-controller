package main

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	rpio "github.com/stianeikeland/go-rpio"
)

const (
	lampPin     = 24
	brokerUri   = "tcp://broker.home.cha-king.com:1883"
	topicBase   = "bedroom/lamp"
	onTopic     = topicBase + "/on"
	setOnTopic  = topicBase + "/setOn"
	onlineTopic = topicBase + "/online"
	qos         = 0
)

func handleSetOn(pin rpio.Pin) mqtt.MessageHandler {
	return func(client mqtt.Client, message mqtt.Message) {
		payload := message.Payload()
		var val bool
		err := json.Unmarshal(payload, &val)
		if err != nil {
			log.Printf("Unable to decode message JSON: %s", payload)
			return
		}
		log.Printf("Message received on topic %s: %s", onTopic, val)
		if val {
			pin.High()
		} else {
			pin.Low()
		}
	}
}

func publishOn(pin rpio.Pin, client mqtt.Client) {
	state := pin.Read()
	var message []byte
	if state == rpio.High {
		message = []byte("true")
	} else if state == rpio.Low {
		message = []byte("false")
	} else {
		panic(fmt.Errorf("unknown pin state %v", state))
	}

	token := client.Publish(onTopic, qos, true, message)
	token.Wait()
	log.Printf("Published state %v", state)
}

func main() {
	rpio.Open()
	defer rpio.Close()
	pin := rpio.Pin(lampPin)
	pin.Output()

	clientOptions := mqtt.NewClientOptions()
	clientOptions.AddBroker(brokerUri)
	clientOptions.SetConnectionAttemptHandler(func(broker *url.URL, tlsCfg *tls.Config) *tls.Config {
		log.Println("Attempting connection..")
		return tlsCfg
	})
	clientOptions.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		log.Printf("Connection lost: %v", err)
	})
	clientOptions.SetOnConnectHandler(func(c mqtt.Client) {
		log.Printf("Connected to %s", brokerUri)

		c.Publish(onlineTopic, qos, true, []byte("true"))

		var tokens []mqtt.Token

		log.Printf("Subscribing to %s", setTopic)
		token := c.Subscribe(setTopic, qos, handleSetState(pin))
		tokens = append(tokens, token)

		log.Printf("Subscribing to %s", getTopic)
		token = c.Subscribe(getTopic, qos, handleGetState(pin))
		tokens = append(tokens, token)

		for _, token := range tokens {
			token.Wait()
			err := token.Error()
			if err != nil {
				panic(err)
			}
		}
		log.Println("Subscribed")

		publishState(pin, c)
	})
	clientOptions.SetReconnectingHandler(func(c mqtt.Client, opts *mqtt.ClientOptions) {
		log.Println("Reconnecting..")
	})
	clientOptions.SetBinaryWill(onlineTopic, []byte("false"), qos, true)
	client := mqtt.NewClient(clientOptions)

	token := client.Connect()
	defer client.Disconnect(0)
	token.Wait()
	if token.Error() != nil {
		log.Fatal(token.Error())
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Print("Exiting..")
}
