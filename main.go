package main

import (
	"crypto/tls"
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

func handleSetState(pin rpio.Pin) mqtt.MessageHandler {
	return func(client mqtt.Client, message mqtt.Message) {
		val := string(message.Payload())
		log.Printf("Message received on topic %s: %s", setTopic, val)
		switch val {
		case "on":
			pin.High()
		case "off":
			pin.Low()
		}
	}
}

func handleGetState(pin rpio.Pin) mqtt.MessageHandler {
	return func(client mqtt.Client, message mqtt.Message) {
		log.Printf("Message received on topic %s", getTopic)
		publishState(pin, client)
	}
}

func publishState(pin rpio.Pin, client mqtt.Client) {
	state := pin.Read()
	var message string
	if state == rpio.High {
		message = "on"
	} else if state == rpio.Low {
		message = "off"
	} else {
		panic(fmt.Errorf("unknown pin state %v", state))
	}

	token := client.Publish(publishTopic, qos, false, message)
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
	client := mqtt.NewClient(clientOptions)

	token := client.Connect()
	defer client.Disconnect(0)
	token.Wait()
	if token.Error() != nil {
		log.Fatal(token.Error())
	}

	quit := make(chan os.Signal, 8)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Print("Exiting..")
}
