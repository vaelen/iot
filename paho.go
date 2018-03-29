// Copyright 2018, Andrew C. Young
// License: MIT

//+build !test

package iot

import (
	"context"
	"crypto/tls"
	"fmt"

	mqtt "github.com/vaelen/paho.mqtt.golang"
)

// PahoMQTTClient is an implementation of PahoMQTTClient that uses Eclipse Paho.
type PahoMQTTClient struct {
	thing               Thing
	options             *ThingOptions
	clientID            string
	client              mqtt.Client
	credentialsProvider MQTTCredentialsProvider
}

// NewPahoClient creates an MQTTClient instance using Eclipse Paho.
func NewPahoClient(thing Thing, options *ThingOptions) MQTTClient {
	return &PahoMQTTClient{
		thing:   thing,
		options: options,
	}
}

func init() {
	NewClient = NewPahoClient
}

// IsConnected should return true when the client is connected to the server
func (c *PahoMQTTClient) IsConnected() bool {
	if c.client == nil {
		return false
	}
	return c.client.IsConnected()
}

// Connect should connect to the given MQTT server
func (c *PahoMQTTClient) Connect(ctx context.Context, servers ...string) error {

	clientOptions := mqtt.NewClientOptions()

	var store mqtt.Store
	if c.options.QueueDirectory == "" {
		store = mqtt.NewMemoryStore()
	} else {
		store = mqtt.NewFileStore(c.options.QueueDirectory)
	}

	clientOptions.SetTLSConfig(&tls.Config{
		Certificates:       []tls.Certificate{c.options.Credentials.Certificate},
		InsecureSkipVerify: true,
	})

	clientOptions.SetCleanSession(false)
	clientOptions.SetAutoReconnect(true)
	clientOptions.SetProtocolVersion(4)
	clientOptions.SetClientID(c.clientID)
	clientOptions.SetUsername("unused")
	clientOptions.SetStore(store)
	clientOptions.SetCredentialsProvider(func() (string, string) { return c.credentialsProvider() })
	clientOptions.SetOnConnectHandler(func(i mqtt.Client) {
		if c.options.InfoLogger != nil {
			c.options.InfoLogger("Connected")
		}
	})
	clientOptions.SetConnectionLostHandler(func(client mqtt.Client, e error) {
		if c.options.ErrorLogger != nil {
			c.options.ErrorLogger(fmt.Sprintf("Connection Lost. Error: %v", e))
		}
	})

	for _, server := range servers {
		clientOptions.AddBroker(server)
	}

	c.client = mqtt.NewClient(clientOptions)

	token := c.client.Connect()
	token.Wait()
	return token.Error()

}

// Disconnect will disconnect from the given MQTT server and clean up all client resources
func (c *PahoMQTTClient) Disconnect(ctx context.Context) error {
	if c.IsConnected() {
		c.client.Disconnect(1000)
		c.client = nil
	}
	return nil
}

// Publish will publish the given payload to the given topic with the given quality of service level
func (c *PahoMQTTClient) Publish(ctx context.Context, topic string, qos uint8, payload interface{}) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}
	token := c.client.Publish(topic, qos, true, payload)
	token.Wait()
	return token.Error()
}

// Subscribe will subscribe to the given topic with the given quality of service level and message handler
func (c *PahoMQTTClient) Subscribe(ctx context.Context, topic string, qos uint8, callback ConfigHandler) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}
	handler := func(i mqtt.Client, message mqtt.Message) {
		if c.options.DebugLogger != nil {
			c.options.DebugLogger(fmt.Sprintf("RECEIVED - Topic: %s, Message Length: %d bytes", message.Topic(), len(message.Payload())))
		}
		if callback != nil {
			callback(c.thing, message.Payload())
		}
	}
	token := c.client.Subscribe(topic, qos, handler)
	token.Wait()
	return token.Error()
}

// Unsubscribe will unsubscribe from the given topic
func (c *PahoMQTTClient) Unsubscribe(ctx context.Context, topic string) error {
	if !c.IsConnected() {
		return ErrNotConnected
	}
	token := c.client.Unsubscribe(topic)
	token.Wait()
	return token.Error()
}

// SetDebugLogger sets the logger to use for logging debug messages
func (c *PahoMQTTClient) SetDebugLogger(logger Logger) {
	mqtt.DEBUG = &pahoLogger{logger}
}

// SetInfoLogger sets the logger to use for logging information or warning messages
func (c *PahoMQTTClient) SetInfoLogger(logger Logger) {
	mqtt.WARN = &pahoLogger{logger}
}

// SetErrorLogger sets the logger to use for logging error or critical messages
func (c *PahoMQTTClient) SetErrorLogger(logger Logger) {
	mqtt.CRITICAL = &pahoLogger{logger}
	mqtt.ERROR = &pahoLogger{logger}
}

// SetClientID sets the MQTT client id
func (c *PahoMQTTClient) SetClientID(clientID string) {
	c.clientID = clientID
}

// SetCredentialsProvider sets the CredentialsProvider used by the MQTT client
func (c *PahoMQTTClient) SetCredentialsProvider(credentialsProvider MQTTCredentialsProvider) {
	c.credentialsProvider = credentialsProvider
}

type pahoLogger struct {
	logger Logger
}

func (l *pahoLogger) Println(v ...interface{}) {
	if l.logger != nil {
		l.logger(v...)
	}
}

func (l *pahoLogger) Printf(format string, v ...interface{}) {
	if l.logger != nil {
		l.logger(fmt.Sprintf(format, v...))
	}
}
