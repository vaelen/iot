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

const RSACertificatePath = "test_keys/rsa_cert.pem"
const RSAPrivateKeyPath = "test_keys/rsa_private.pem"

const ECCertificatePath = "test_keys/ec_cert.pem"
const ECPrivateKeyPath = "test_keys/ec_private.pem"

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

var mockClient *iot.MockMQTTClient

func TestLoadRSACredentials(t *testing.T) {
	credentials, err := iot.LoadRSACredentials(RSACertificatePath, RSAPrivateKeyPath)
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

func TestLoadECCredentials(t *testing.T) {
	credentials, err := iot.LoadECCredentials(ECCertificatePath, ECPrivateKeyPath)
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
	credentials, err := iot.LoadRSACredentials(RSACertificatePath, RSAPrivateKeyPath)
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

func TestThingWithBadOptions(t *testing.T) {
	ctx := context.Background()
	var mockClient *iot.MockMQTTClient
	iot.NewClient = func(t iot.Thing, o *iot.ThingOptions) iot.MQTTClient {
		mockClient = iot.NewMockClient(t, o)
		return mockClient
	}

	options := &iot.ThingOptions{}
	thing := iot.New(options)
	if thing == nil {
		t.Fatal("Thing was not returned from New() with bad options")
	}

	err := thing.Connect(ctx, "bad options")
	if err != iot.ErrConfigurationError {
		t.Fatalf("Wrong error returned from Connect() with invalid options: %v", err)
	}
}

func TestRSAThingFull(t *testing.T) {
	initMockClient()
	credentials := getCredentials(t, iot.CredentialTypeRSA)
	options, configReceived := getOptions(t, credentials)
	thing := getThing(t, options)
	serverAddress := "ssl://mqtt.example.com:443"
	doConnectionTest(t, thing, serverAddress)
	doAlreadyConnectedTest(t, thing, serverAddress)
	checkClientValues(t, options)
	doConfigTest(t, configReceived)
	doEventTest(t, thing)
	doDisconnectTest(t, thing)
}

func TestECThingConnectOnly(t *testing.T) {
	initMockClient()
	credentials := getCredentials(t, iot.CredentialTypeEC)
	options, _ := getOptions(t, credentials)
	thing := getThing(t, options)
	serverAddress := "ssl://mqtt.example.com:443"
	doConnectionTest(t, thing, serverAddress)
	checkClientValues(t, options)
	doDisconnectTest(t, thing)
}

func initMockClient() {
	iot.NewClient = func(t iot.Thing, o *iot.ThingOptions) iot.MQTTClient {
		mockClient = iot.NewMockClient(t, o)
		return mockClient
	}
}

func getThing(t *testing.T, options *iot.ThingOptions) iot.Thing {
	thing := iot.New(options)
	if thing == nil {
		t.Fatal("Thing wasn't returned from New()")
	}

	if thing.IsConnected() {
		t.Fatal("Thing thinks it is connected when it really is not")
	}
	return thing
}

func getCredentials(t *testing.T, credentialType iot.CredentialType) *iot.Credentials {
	var credentials *iot.Credentials
	var err error
	switch credentialType {
	case iot.CredentialTypeEC:
		credentials, err = iot.LoadECCredentials(ECCertificatePath, ECPrivateKeyPath)
	case iot.CredentialTypeRSA:
		fallthrough
	default:
		credentials, err = iot.LoadRSACredentials(RSACertificatePath, RSAPrivateKeyPath)
	}
	if err != nil {
		t.Fatalf("Couldn't load credentials: %v", err)
	}
	return credentials
}

func getOptions(t *testing.T, credentials *iot.Credentials) (*iot.ThingOptions, *bytes.Buffer) {
	options := iot.DefaultOptions(TestID, credentials)
	if options == nil {
		t.Fatal("Options structure wasn't returned")
	}

	debugWriter := &bytes.Buffer{}
	infoWriter := &bytes.Buffer{}
	errorWriter := &bytes.Buffer{}

	configReceived := &bytes.Buffer{}

	options.AuthTokenExpiration = 0
	options.DebugLogger = func(a ...interface{}) { fmt.Fprint(debugWriter, a...) }
	options.InfoLogger = func(a ...interface{}) { fmt.Fprint(infoWriter, a...) }
	options.ErrorLogger = func(a ...interface{}) { fmt.Fprint(errorWriter, a...) }
	options.LogMQTT = true
	options.ConfigHandler = func(thing iot.Thing, config []byte) {
		ctx := context.Background()
		configReceived.Truncate(0)
		configReceived.Write(config)
		state := []byte("ok")
		thing.PublishState(ctx, state)
	}

	return options, configReceived
}

func doConnectionTest(t *testing.T, thing iot.Thing, serverAddress string) {
	ctx := context.Background()
	err := thing.Connect(ctx, serverAddress)
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
}

func doAlreadyConnectedTest(t *testing.T, thing iot.Thing, serverAddress string) {
	ctx := context.Background()

	err := thing.Connect(ctx, "already connected")
	if err != nil {
		t.Fatalf("Calling Connect() while already connected returned an error: %v", err)
	}

	if len(mockClient.ConnectedTo) < 1 || mockClient.ConnectedTo[0] != serverAddress {
		t.Fatalf("Calling Connect() while already connected caused client to reconnect: %v", mockClient.ConnectedTo)
	}

	if mockClient.CredentialsProvider == nil {
		t.Fatal("Credentials provider not set")
	}
}

func checkClientValues(t *testing.T, options *iot.ThingOptions) {
	options.AuthTokenExpiration = 0
	username, password := mockClient.CredentialsProvider()
	if username == "" || password == "" {
		t.Fatalf("Bad username and/or password returned. Username: %v, Password: %v", username, password)
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

	if mockClient.OnConnectHandler == nil {
		t.Fatalf("OnConnectHandler not set")
	}
}

func doConfigTest(t *testing.T, configReceived *bytes.Buffer) {
	mockClient.Receive(ConfigTopic, []byte("test config"))

	if configReceived.String() != "test config" {
		t.Fatalf("Wrong configuration received: %v", configReceived.String())
	}

	l, ok := mockClient.Messages[StateTopic]
	if !ok || l == nil || len(l) == 0 {
		t.Fatalf("State not published")
	}

	if string(l[0].([]byte)) != "ok" {
		t.Fatalf("Wrong state published: %v", string(l[0].([]byte)))
	}
}

func doEventTest(t *testing.T, thing iot.Thing) {
	ctx := context.Background()
	topLevelMessage := "Top"
	events := make(map[string]string)
	events["a"] = "A"
	events["a/b"] = "B"

	err := thing.PublishEvent(ctx, []byte(topLevelMessage))
	if err != nil {
		t.Fatalf("Couldn't publish. Error: %v", err)
	}

	for k, v := range events {
		err = thing.PublishEvent(ctx, []byte(v), strings.Split(k, "/")...)
		if err != nil {
			t.Fatalf("Couldn't publish. Error: %v", err)
		}
	}

	l, ok := mockClient.Messages[EventsTopic]
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
}

func doDisconnectTest(t *testing.T, thing iot.Thing) {
	thing.Disconnect(context.Background())
	if mockClient.Connected {
		t.Fatal("Didn't disconnect")
	}
}
