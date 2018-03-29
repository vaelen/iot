// Copyright 2018, Andrew C. Young
// License: MIT

package iot

import (
	"context"
)

// MockMQTTClient implements a mock MQTT client for use in testing
// To use this client, use code like the following:
// set iot.NewClient = iot.NewMockClient
type MockMQTTClient struct {
	t                   Thing
	o                   *ThingOptions
	Connected           bool
	ConnectedTo         []string
	Messages            map[string][]interface{}
	Subscriptions       map[string]ConfigHandler
	DebugLogger         Logger
	InfoLogger          Logger
	ErrorLogger         Logger
	ClientID            string
	CredentialsProvider MQTTCredentialsProvider
}

// NewMockClient returns an instance of MockMQTTClient
// The MockMQTTClient documentation explains how to use this method when writing tests.
func NewMockClient(t Thing, o *ThingOptions) *MockMQTTClient {
	return &MockMQTTClient{
		t:             t,
		o:             o,
		Messages:      make(map[string][]interface{}),
		Subscriptions: make(map[string]ConfigHandler),
	}
}

// Receive imitates the client receiving a message on the given topic for testing purposes.
func (c *MockMQTTClient) Receive(topic string, message []byte) {
	handler := c.Subscriptions[topic]
	if handler != nil {
		handler(c.t, message)
	}
}

// IsConnected returns the value of the Connected field
func (c *MockMQTTClient) IsConnected() bool {
	return c.Connected
}

// Connect sets the Connected field to true and the ConnectedTo field to the list of servers
func (c *MockMQTTClient) Connect(ctx context.Context, servers ...string) error {
	c.Connected = true
	c.ConnectedTo = servers
	return nil
}

// Disconnect sets the Connected field to false and clears the ConnectedTo field
func (c *MockMQTTClient) Disconnect(ctx context.Context) error {
	c.Connected = false
	c.ConnectedTo = nil
	return nil
}

// Publish adds the given payload to the Messages map under the given topic
func (c *MockMQTTClient) Publish(ctx context.Context, topic string, qos uint8, payload interface{}) error {
	l, ok := c.Messages[topic]
	if !ok {
		l = make([]interface{}, 0, 1)
	}
	c.Messages[topic] = append(l, payload)
	return nil
}

// Subscribe addes the given ConfigHandler to the Subscriptions map for the given topic
func (c *MockMQTTClient) Subscribe(ctx context.Context, topic string, qos uint8, callback ConfigHandler) error {
	c.Subscriptions[topic] = callback
	return nil
}

// Unsubscribe removes the ConfigHandler from the Subscriptions map for the given topic
func (c *MockMQTTClient) Unsubscribe(ctx context.Context, topic string) error {
	delete(c.Subscriptions, topic)
	return nil
}

// SetDebugLogger sets DebugLogger
func (c *MockMQTTClient) SetDebugLogger(logger Logger) {
	c.DebugLogger = logger
}

// SetInfoLogger sets InfoLogger
func (c *MockMQTTClient) SetInfoLogger(logger Logger) {
	c.InfoLogger = logger
}

// SetErrorLogger sets ErrorLogger
func (c *MockMQTTClient) SetErrorLogger(logger Logger) {
	c.ErrorLogger = logger
}

// SetClientID sets ClientID
func (c *MockMQTTClient) SetClientID(clientID string) {
	c.ClientID = clientID
}

// SetCredentialsProvider sets CredentialsProvider
func (c *MockMQTTClient) SetCredentialsProvider(crendentialsProvider MQTTCredentialsProvider) {
	c.CredentialsProvider = crendentialsProvider
}
