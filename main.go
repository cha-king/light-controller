package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	rpio "github.com/stianeikeland/go-rpio"
)

const (
	lampPin   = 24
	brokerUri = "tcp://broker.home.cha-king.com:1883"
	topicBase = "bedroom/lamp"
	topic     = topicBase + "/setState"
	qos       = 1
)

func messageHandler(pin rpio.Pin) mqtt.MessageHandler {
	return func(client mqtt.Client, message mqtt.Message) {
		val := string(message.Payload())
		log.Printf("Message received: %s", val)
		switch val {
		case "on":
			pin.High()
		case "off":
			pin.Low()
		}
	}
}

func main() {
	rpio.Open()
	defer rpio.Close()
	pin := rpio.Pin(lampPin)

	clientOptions := mqtt.NewClientOptions()
	clientOptions.AddBroker(brokerUri)
	client := mqtt.NewClient(clientOptions)

	token := client.Connect()
	defer client.Disconnect(0)
	token.Wait()
	if token.Error() != nil {
		log.Fatal(token.Error())
	}
	log.Printf("Connected to %s", brokerUri)

	token = client.Subscribe(topic, qos, messageHandler(pin))
	token.Wait()

	quit := make(chan os.Signal, 8)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Print("Exiting..")
}
