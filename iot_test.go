// Copyright 2018, Andrew C. Young
// License: MIT

package iot_test

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/vaelen/iot"
)

const CertificatePath = "test_keys/rsa_cert.pem"
const PrivateKeyPath = "test_keys/rsa_private.pem"

var TestID = &iot.ID{
	ProjectID: "test-project",
	Location:  "test-location",
	Registry:  "test-registry",
	DeviceID:  "test-device",
}

var ClientID = "projects/test-project/locations/test-location/registries/test-registry/devices/test-device"
var ConfigTopic = "/devices/test-device/config"
var StateTopic = "/devices/test-device/state"
var EventsTopic = "/devices/test-device/events"

func TestLoadCredentials(t *testing.T) {
	credentials, err := iot.LoadCredentials(CertificatePath, PrivateKeyPath)
	if err != nil {
		t.Fatalf("Couldn't load credentials: %v", err)
	}
	if credentials == nil {
		t.Fatal("Credentials not loaded.")
	}
	if credentials.PrivateKey == nil {
		t.Fatal("Private key not loaded.")
	}
}

func TestDefaultOptions(t *testing.T) {
	credentials, err := iot.LoadCredentials(CertificatePath, PrivateKeyPath)
	if err != nil {
		t.Fatalf("Couldn't load credentials: %v", err)
	}

	options := iot.DefaultOptions(TestID, credentials)
	if options == nil {
		t.Fatal("Options structure wasn't returned")
	}
	if options.ID != TestID {
		t.Fatal("Incorrect ID")
	}
	if options.Credentials != credentials {
		t.Fatal("Incorrect credentials")
	}
	if options.EventQOS != 1 {
		t.Fatalf("Incorrect event QoS: %v", options.EventQOS)
	}
	if options.StateQOS != 1 {
		t.Fatalf("Incorrect state QoS: %v", options.StateQOS)
	}
	if options.ConfigQOS != 2 {
		t.Fatalf("Incorrect config QoS: %v", options.ConfigQOS)
	}
	if options.AuthTokenExpiration != iot.DefaultAuthTokenExpiration {
		t.Fatalf("Incorrect auth token expiration: %v", options.AuthTokenExpiration)
	}
}
func TestThing(t *testing.T) {
	ctx := context.Background()
	var mockClient *iot.MockMQTTClient
	iot.NewClient = func(t iot.Thing, o *iot.ThingOptions) iot.MQTTClient {
		mockClient = iot.NewMockClient(t, o)
		return mockClient
	}

	credentials, err := iot.LoadCredentials(CertificatePath, PrivateKeyPath)
	if err != nil {
		t.Fatalf("Couldn't load credentials: %v", err)
	}

	options := iot.DefaultOptions(TestID, credentials)
	if options == nil {
		t.Fatal("Options structure wasn't returned")
	}

	debugWriter := &bytes.Buffer{}
	infoWriter := &bytes.Buffer{}
	errorWriter := &bytes.Buffer{}

	var configReceived []byte

	options.DebugLogger = func(a ...interface{}) { fmt.Fprint(debugWriter, a...) }
	options.InfoLogger = func(a ...interface{}) { fmt.Fprint(infoWriter, a...) }
	options.ErrorLogger = func(a ...interface{}) { fmt.Fprint(errorWriter, a...) }
	options.LogMQTT = true
	options.ConfigHandler = func(thing iot.Thing, config []byte) {
		configReceived = config
		state := []byte("ok")
		thing.PublishState(ctx, state)
	}

	thing := iot.New(options)
	if thing == nil {
		t.Fatal("Thing wasn't returned from New()")
	}

	if thing.IsConnected() {
		t.Fatal("Thing thinks it is connected when it really is not")
	}

	serverAddress := "ssl://mqtt.example.com:443"

	err = thing.Connect(ctx, serverAddress)
	if err != nil {
		t.Fatalf("Couldn't connect. Error: %v", err)
	}

	if !mockClient.Connected {
		t.Fatalf("Client not connected")
	}

	if len(mockClient.ConnectedTo) < 1 || mockClient.ConnectedTo[0] != serverAddress {
		t.Fatalf("Client connected to wrong server: %v", mockClient.ConnectedTo)
	}

	if !thing.IsConnected() {
		t.Fatal("Thing thinks it is not connected when it really is")
	}

	if mockClient.CredentialsProvider == nil {
		t.Fatal("Credentials provider not set")
	}

	if mockClient.DebugLogger == nil {
		t.Fatal("Debug logger not set")
	}

	if mockClient.InfoLogger == nil {
		t.Fatal("Info logger not set")
	}

	if mockClient.ErrorLogger == nil {
		t.Fatal("Error logger not set")
	}

	if mockClient.ClientID != ClientID {
		t.Fatalf("Client ID not set properly: %v", mockClient.ClientID)
	}

	if len(mockClient.Subscriptions) != 1 {
		t.Fatalf("Wrong number of subscriptions: %v", len(mockClient.Subscriptions))
	}

	mockClient.Receive(ConfigTopic, []byte("test config"))

	if string(configReceived) != "test config" {
		t.Fatalf("Wrong configuration received: %v", string(configReceived))
	}

	l, ok := mockClient.Messages[StateTopic]
	if !ok || l == nil || len(l) == 0 {
		t.Fatalf("State not published")
	}

	if string(l[0].([]byte)) != "ok" {
		t.Fatalf("Wrong state published: %v", string(l[0].([]byte)))
	}

	topLevelMessage := "Top"
	events := make(map[string]string)
	events["a"] = "A"
	events["a/b"] = "B"

	err = thing.PublishEvent(ctx, []byte(topLevelMessage))
	if err != nil {
		t.Fatalf("Couldn't publish. Error: %v", err)
	}

	for k, v := range events {
		err = thing.PublishEvent(ctx, []byte(v), strings.Split(k, "/")...)
		if err != nil {
			t.Fatalf("Couldn't publish. Error: %v", err)
		}
	}

	l, ok = mockClient.Messages[EventsTopic]
	if !ok || l == nil || len(l) == 0 {
		t.Fatalf("Message not published. Topic: %v", EventsTopic)
	}

	if len(l) > 1 {
		t.Fatalf("Too many messages published. Topic: %v, Count; %v", EventsTopic, len(l))
	}

	if string(l[0].([]byte)) != topLevelMessage {
		t.Fatalf("Wrong message published.  Topic: %v, Message: %v", EventsTopic, string(l[0].([]byte)))
	}

	for k, v := range events {
		topic := EventsTopic + "/" + k
		l, ok = mockClient.Messages[topic]
		if !ok || l == nil || len(l) == 0 {
			t.Fatalf("Message not published. Topic: %v", topic)
		}

		if len(l) > 1 {
			t.Fatalf("Too many messages published. Topic: %v, Count; %v", topic, len(l))
		}

		if string(l[0].([]byte)) != v {
			t.Fatalf("Wrong message published.  Topic: %v, Message: %v", topic, string(l[0].([]byte)))
		}
	}

	thing.Disconnect(ctx)
	if mockClient.Connected {
		t.Fatal("Didn't disconnect")
	}
}
