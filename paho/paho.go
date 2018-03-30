// Copyright 2018, Andrew C. Young
// License: MIT

// Package paho provides an iot.MQTTClient implementation that uses the Eclipse Paho MQTT client.
// To use the client, you must import this package.
package paho

import (
	"context"
	"crypto/tls"
	"fmt"
	"time"

	"github.com/vaelen/iot"
	mqtt "github.com/vaelen/paho.mqtt.golang"
)

const waitTimeoutDuration = time.Millisecond * 100

// MQTTClient is an implementation of MQTTClient that uses Eclipse Paho.
// To use the client, you must include this package.
type MQTTClient struct {
	thing               iot.Thing
	options             *iot.ThingOptions
	clientID            string
	client              mqtt.Client
	credentialsProvider iot.MQTTCredentialsProvider
}

// NewClient creates an MQTTClient instance using Eclipse Paho.
func NewClient(thing iot.Thing, options *iot.ThingOptions) iot.MQTTClient {
	return &MQTTClient{
		thing:   thing,
		options: options,
	}
}

// This method is automatically called if the package is included
func init() {
	if iot.NewClient == nil {
		iot.NewClient = NewClient
	}
}

// IsConnected should return true when the client is connected to the server
func (c *MQTTClient) IsConnected() bool {
	if c.client == nil {
		return false
	}
	return c.client.IsConnected()
}

// Connect should connect to the given MQTT server
func (c *MQTTClient) Connect(ctx context.Context, servers ...string) error {

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
	return waitForToken(ctx, token)
}

// Disconnect will disconnect from the given MQTT server and clean up all client resources
func (c *MQTTClient) Disconnect(ctx context.Context) error {
	if c.IsConnected() {
		c.client.Disconnect(1000)
		c.client = nil
	}
	return nil
}

// Publish will publish the given payload to the given topic with the given quality of service level
func (c *MQTTClient) Publish(ctx context.Context, topic string, qos uint8, payload interface{}) error {
	if !c.IsConnected() {
		return iot.ErrNotConnected
	}
	token := c.client.Publish(topic, qos, true, payload)
	return waitForToken(ctx, token)
}

// Subscribe will subscribe to the given topic with the given quality of service level and message handler
func (c *MQTTClient) Subscribe(ctx context.Context, topic string, qos uint8, callback iot.ConfigHandler) error {
	if !c.IsConnected() {
		return iot.ErrNotConnected
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
	return waitForToken(ctx, token)
}

// Unsubscribe will unsubscribe from the given topic
func (c *MQTTClient) Unsubscribe(ctx context.Context, topic string) error {
	if !c.IsConnected() {
		return iot.ErrNotConnected
	}
	token := c.client.Unsubscribe(topic)
	return waitForToken(ctx, token)
}

// SetDebugLogger sets the logger to use for logging debug messages
func (c *MQTTClient) SetDebugLogger(logger iot.Logger) {
	mqtt.DEBUG = &pahoLogger{logger}
}

// SetInfoLogger sets the logger to use for logging information or warning messages
func (c *MQTTClient) SetInfoLogger(logger iot.Logger) {
	mqtt.WARN = &pahoLogger{logger}
}

// SetErrorLogger sets the logger to use for logging error or critical messages
func (c *MQTTClient) SetErrorLogger(logger iot.Logger) {
	mqtt.CRITICAL = &pahoLogger{logger}
	mqtt.ERROR = &pahoLogger{logger}
}

// SetClientID sets the MQTT client id
func (c *MQTTClient) SetClientID(clientID string) {
	c.clientID = clientID
}

// SetCredentialsProvider sets the CredentialsProvider used by the MQTT client
func (c *MQTTClient) SetCredentialsProvider(credentialsProvider iot.MQTTCredentialsProvider) {
	c.credentialsProvider = credentialsProvider
}

func waitForToken(ctx context.Context, token mqtt.Token) error {
	result := make(chan error)
	cancelled := false
	go func() {
		defer func() { result <- token.Error() }()
		for {
			if (token.WaitTimeout(waitTimeoutDuration)) || cancelled {
				return
			}
		}
	}()
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		cancelled = true
	}
	return iot.ErrCancelled
}

type pahoLogger struct {
	logger iot.Logger
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
