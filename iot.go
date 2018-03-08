// Copyright 2018, Andrew C. Young
// License: MIT

package iot

import (
	"crypto/rsa"
	"crypto/tls"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/eclipse/paho.mqtt.golang"
	"io/ioutil"
	"strings"
	"time"
)

var ErrNotConnected = fmt.Errorf("not connected")

type ConfigHandler func(thing *Thing, config []byte)
type Logger func(msg string)

type LogLevel uint8

const (
	LogLevelError LogLevel = 0
	LogLevelInfo  LogLevel = 1
	LogLevelDebug LogLevel = 2
)

type ID struct {
	ProjectID string
	Location  string
	Registry  string
	DeviceID  string
}

type Credentials struct {
	Certificate tls.Certificate
	PrivateKey  *rsa.PrivateKey
}

// LoadCredentials creates a Credentials from the given private key and certificate
func LoadCredentials(certificatePath string, privateKeyPath string) (*Credentials, error) {
	signBytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	privateKey, err := jwt.ParseRSAPrivateKeyFromPEM(signBytes)
	if err != nil {
		return nil, err
	}

	certificate, err := tls.LoadX509KeyPair(certificatePath, privateKeyPath)
	if err != nil {
		return nil, err
	}

	return &Credentials{
		Certificate: certificate,
		PrivateKey:  privateKey,
	}, nil
}

type Thing struct {
	ID             ID
	Credentials    *Credentials
	Logger         Logger
	LogLevel       LogLevel
	QueueDirectory string
	ConfigHandler  ConfigHandler
	ConfigQOS      uint8
	StateQOS       uint8
	EventQOS       uint8
	client         mqtt.Client
}

// PublishState publishes the current device state
func (t *Thing) PublishState(message []byte) error {
	return t.publish(t.stateTopic(), message, t.StateQOS)
}

// PublishEvent publishes an event. An optional hierarchy of event names can be provided.
func (t *Thing) PublishEvent(message []byte, event ...string) error {
	return t.publish(t.eventsTopic(event...), message, t.EventQOS)
}

// Connect to the given MQTT server(s)
func (t *Thing) Connect(servers ...string) error {
	authToken, err := t.authToken()
	if err != nil {
		return err
	}

	var store mqtt.Store
	if t.QueueDirectory == "" {
		store = mqtt.NewMemoryStore()
	} else {
		store = mqtt.NewFileStore(t.QueueDirectory)
	}

	options := mqtt.NewClientOptions()

	options.SetTLSConfig(&tls.Config{
		Certificates:       []tls.Certificate{t.Credentials.Certificate},
		InsecureSkipVerify: true,
	})

	options.SetProtocolVersion(4)
	options.SetClientID(t.clientID())
	options.SetUsername("unused")
	options.SetPassword(authToken)
	options.SetStore(store)
	options.SetOnConnectHandler(func(i mqtt.Client) {
		t.infof("Connected")
	})
	options.SetConnectionLostHandler(func(i mqtt.Client, e error) {
		t.errorf("Connection Lost. Error: %v", e)
	})

	for _, server := range servers {
		options.AddBroker(server)
	}

	if t.client != nil {
		t.Disconnect()
	}

	t.client = mqtt.NewClient(options)

	connectToken := t.client.Connect()
	connectToken.Wait()
	if connectToken.Error() != nil {
		return connectToken.Error()
	}

	t.client.Subscribe(t.configTopic(), t.ConfigQOS, t.configHandler)

	return nil
}

// IsConnected returns true of the client is currently connected to MQTT server(s)
func (t *Thing) IsConnected() bool {
	return t.client != nil && t.client.IsConnected()
}

// Disconnect from the MQTT server(s)
func (t *Thing) Disconnect() {
	if t.client != nil {
		t.client.Unsubscribe(t.configTopic())
		t.client.Disconnect(250)
	}
}

// Internal methods

func (t *Thing) clientID() string {
	return fmt.Sprintf("projects/%s/locations/%s/registries/%s/devices/%s", t.ID.ProjectID, t.ID.Location, t.ID.Registry, t.ID.DeviceID)
}

func (t *Thing) authToken() (string, error) {
	wt := jwt.New(jwt.GetSigningMethod("RS256"))

	wt.Claims = &jwt.StandardClaims{
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: time.Now().Add(time.Hour * 1).Unix(),
		Audience:  t.ID.ProjectID,
	}

	token, err := wt.SignedString(t.Credentials.PrivateKey)
	if err != nil {
		return "", err
	}

	return token, nil
}

func (t *Thing) configTopic() string {
	return fmt.Sprintf("/devices/%s/config", t.ID.DeviceID)
}

func (t *Thing) stateTopic() string {
	return fmt.Sprintf("/devices/%s/state", t.ID.DeviceID)
}

func (t *Thing) eventsTopic(subTopic ...string) string {
	if len(subTopic) == 0 {
		return fmt.Sprintf("/devices/%s/events", t.ID.DeviceID)
	}
	return fmt.Sprintf("/devices/%s/events/%s", t.ID.DeviceID, strings.Join(subTopic, "/"))
}

func (t *Thing) configHandler(i mqtt.Client, message mqtt.Message) {
	t.debugf("RECEIVED - Topic: %s, Message Length: %d bytes", message.Topic(), len(message.Payload()))
	if t.ConfigHandler != nil {
		t.ConfigHandler(t, message.Payload())
	}
}

func (t *Thing) publish(topic string, message []byte, qos uint8) error {
	if t.client == nil || !t.client.IsConnected() {
		return ErrNotConnected
	}
	token := t.client.Publish(topic, qos, true, message)
	token.Wait()
	if token.Error() != nil {
		return token.Error()
	}
	t.debugf("SENT - Topic: %s, Message Length: %d bytes", topic, len(message))
	return nil
}

func (t *Thing) log(level string, msg string) {
	if t.Logger != nil {
		t.Logger(fmt.Sprintf("|%s| %s", level, msg))
	}
}

func (t *Thing) debugf(format string, v ...interface{}) {
	if t.Logger != nil && t.LogLevel >= LogLevelDebug {
		msg := fmt.Sprintf(format, v...)
		t.log("INFO", msg)
	}
}

func (t *Thing) infof(format string, v ...interface{}) {
	if t.Logger != nil && t.LogLevel >= LogLevelInfo {
		msg := fmt.Sprintf(format, v...)
		t.log("INFO", msg)
	}
}

func (t *Thing) errorf(format string, v ...interface{}) {
	if t.Logger != nil && t.LogLevel >= LogLevelError {
		msg := fmt.Sprintf(format, v...)
		t.log("ERROR", msg)
	}
}
