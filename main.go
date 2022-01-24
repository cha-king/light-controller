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
		log.Printf("Message received on topic %s: %t", onTopic, val)

		if val {
			log.Println("Setting pin high")
			pin.High()
		} else {
			log.Println("Setting pin low")
			pin.Low()
		}

		log.Println("Publishing light state")
		go publishOn(client, pin)
	}
}

func publishOn(client mqtt.Client, pin rpio.Pin) {
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

func publishOnline(client mqtt.Client) {
	token := client.Publish(onlineTopic, qos, true, []byte("true"))
	token.Wait()
	log.Println("Published online")
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

		log.Println("Publishing online status")
		publishOnline(c)

		log.Println("Publishing light status")
		publishOn(c, pin)

		log.Printf("Subscribing to %s", setOnTopic)
		token := c.Subscribe(setOnTopic, qos, handleSetOn(pin))

		token.Wait()
		err := token.Error()
		if err != nil {
			panic(err)
		}
		log.Println("Subscribed")
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
