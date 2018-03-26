// Copyright 2018, Andrew C. Young
// License: MIT

package iot

import (
	"crypto/tls"
	"fmt"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/eclipse/paho.mqtt.golang"
)

type ClientConstructor func(*mqtt.ClientOptions) mqtt.Client

var NewClient ClientConstructor = mqtt.NewClient

type thing struct {
	options       *ThingOptions
	client        mqtt.Client
	publishTicker *time.Ticker
}

// PublishState publishes the current device state
func (t *thing) PublishState(message []byte) error {
	return t.publish(t.stateTopic(), message, t.options.StateQOS)
}

// PublishEvent publishes an event. An optional hierarchy of event names can be provided.
func (t *thing) PublishEvent(message []byte, event ...string) error {
	return t.publish(t.eventsTopic(event...), message, t.options.EventQOS)
}

// Connect to the given MQTT server(s)
func (t *thing) Connect(servers ...string) error {
	if t.IsConnected() {
		return nil
	}
	if t.options.ID == nil || t.options.Credentials == nil {
		return ErrConfigurationError
	}
	if t.options.AuthTokenExpiration == 0 {
		t.options.AuthTokenExpiration = DefaultAuthTokenExpiration
	}

	var store mqtt.Store
	if t.options.QueueDirectory == "" {
		store = mqtt.NewMemoryStore()
	} else {
		store = mqtt.NewFileStore(t.options.QueueDirectory)
	}

	options := mqtt.NewClientOptions()

	options.SetTLSConfig(&tls.Config{
		Certificates:       []tls.Certificate{t.options.Credentials.Certificate},
		InsecureSkipVerify: true,
	})

	options.SetCleanSession(false)
	options.SetAutoReconnect(true)
	options.SetProtocolVersion(4)
	options.SetClientID(t.clientID())
	options.SetUsername("unused")
	options.SetStore(store)
	options.SetOnConnectHandler(func(i mqtt.Client) {
		t.infof("Connected")
	})
	options.SetConnectionLostHandler(func(client mqtt.Client, e error) {
		t.errorf("Connection Lost. Error: %v", e)
	})

	t.publishTicker = time.NewTicker(time.Second * 2)

	for _, server := range servers {
		options.AddBroker(server)
	}

	options.SetCredentialsProvider(func() (username string, password string) {
		authToken, err := t.authToken()
		if err != nil {
			t.errorf("Error generating auth token: %v", err)
			return "", ""
		}
		return "unused", authToken
	})

	t.client = NewClient(options)

	if t.options.LogMQTT {
		mqtt.CRITICAL = &thingMQTTLogger{t.options.ErrorLogger}
		mqtt.ERROR = &thingMQTTLogger{t.options.ErrorLogger}
		mqtt.WARN = &thingMQTTLogger{t.options.InfoLogger}
		mqtt.DEBUG = &thingMQTTLogger{t.options.DebugLogger}
	}

	connectToken := t.client.Connect()
	for !connectToken.WaitTimeout(time.Second) {
		t.debugf("PENDING CONNECT")
	}
	if connectToken.Error() != nil {
		return connectToken.Error()
	}

	t.client.Subscribe(t.configTopic(), t.options.ConfigQOS, t.configHandler)

	return nil
}

// IsConnected returns true of the client is currently connected to MQTT server(s)
func (t *thing) IsConnected() bool {
	return t.client != nil && t.client.IsConnected()
}

// Disconnect from the MQTT server(s)
func (t *thing) Disconnect() {
	if t.client != nil {
		t.client.Unsubscribe(t.configTopic())
		if t.client.IsConnected() {
			t.infof("Disconnecting")
			t.client.Disconnect(1000)
		}
	}
}

// Internal methods

func (t *thing) clientID() string {
	return fmt.Sprintf("projects/%s/locations/%s/registries/%s/devices/%s", t.options.ID.ProjectID, t.options.ID.Location, t.options.ID.Registry, t.options.ID.DeviceID)
}

func (t *thing) authToken() (string, error) {
	wt := jwt.New(jwt.GetSigningMethod("RS256"))

	expirationInterval := t.options.AuthTokenExpiration
	if expirationInterval == 0 {
		expirationInterval = time.Hour
	}

	wt.Claims = &jwt.StandardClaims{
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(expirationInterval).Unix(),
		Audience:  t.options.ID.ProjectID,
	}

	t.debugf("Auth Token: %+v", wt.Claims)

	token, err := wt.SignedString(t.options.Credentials.PrivateKey)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (t *thing) configTopic() string {
	return fmt.Sprintf("/devices/%s/config", t.options.ID.DeviceID)
}

func (t *thing) stateTopic() string {
	return fmt.Sprintf("/devices/%s/state", t.options.ID.DeviceID)
}

func (t *thing) eventsTopic(subTopic ...string) string {
	if len(subTopic) == 0 {
		return fmt.Sprintf("/devices/%s/events", t.options.ID.DeviceID)
	}
	return fmt.Sprintf("/devices/%s/events/%s", t.options.ID.DeviceID, strings.Join(subTopic, "/"))
}

func (t *thing) configHandler(i mqtt.Client, message mqtt.Message) {
	t.debugf("RECEIVED - Topic: %s, Message Length: %d bytes", message.Topic(), len(message.Payload()))
	if t.options.ConfigHandler != nil {
		t.options.ConfigHandler(t, message.Payload())
	}
}

func (t *thing) publish(topic string, message []byte, qos uint8) error {
	<-t.publishTicker.C // Don't publish more than once per second
	token := t.client.Publish(topic, qos, true, message)
	if !token.WaitTimeout(time.Second) {
		t.debugf("SEND TIMEOUT - Topic: %s, Message Length: %d bytes", topic, len(message))
		return ErrPublishFailed
	} else if token.Error() != nil {
		t.debugf("SEND FAILED - Topic: %s, Message Length: %d bytes, Error: %v", topic, len(message), token.Error())
		return token.Error()
	} else {
		t.debugf("SENT - Topic: %s, Message Length: %d bytes", topic, len(message))
		return nil
	}
}

func (t *thing) log(logger Logger, format string, v ...interface{}) {
	if logger != nil {
		msg := fmt.Sprintf(format, v...)
		logger(msg)
	}
}

func (t *thing) debugf(format string, v ...interface{}) {
	t.log(t.options.DebugLogger, format, v)
}

func (t *thing) infof(format string, v ...interface{}) {
	t.log(t.options.InfoLogger, format, v)
}

func (t *thing) errorf(format string, v ...interface{}) {
	t.log(t.options.ErrorLogger, format, v)
}

type thingMQTTLogger struct {
	logger Logger
}

func (l *thingMQTTLogger) Println(v ...interface{}) {
	if l.logger != nil {
		l.logger(v...)
	}
}

func (l *thingMQTTLogger) Printf(format string, v ...interface{}) {
	if l.logger != nil {
		l.logger(fmt.Sprintf(format, v...))
	}
}
