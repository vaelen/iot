// Copyright 2018, Andrew C. Young
// License: MIT

package iot

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/dgrijalva/jwt-go"
)

type thing struct {
	options       *ThingOptions
	client        MQTTClient
	publishTicker *clock.Ticker
}

// PublishState publishes the current device state
func (t *thing) PublishState(ctx context.Context, message []byte) error {
	return t.publish(ctx, t.stateTopic(), message, t.options.StateQOS)
}

// PublishEvent publishes an event. An optional hierarchy of event names can be provided.
func (t *thing) PublishEvent(ctx context.Context, message []byte, event ...string) error {
	return t.publish(ctx, t.eventsTopic(event...), message, t.options.EventQOS)
}

// Connect to the given MQTT server(s)
func (t *thing) Connect(ctx context.Context, servers ...string) error {
	if t.IsConnected() {
		return nil
	}
	if t.options.ID == nil || t.options.Credentials == nil {
		return ErrConfigurationError
	}
	if t.options.AuthTokenExpiration == 0 {
		t.options.AuthTokenExpiration = DefaultAuthTokenExpiration
	}
	if t.options.Clock == nil {
		t.options.Clock = clock.New()
	}

	if NewClient == nil {
		panic("No MQTT client specified. Please import the iot/paho package.")
	}
	t.client = NewClient(t, t.options)

	if t.options.LogMQTT {
		t.client.SetDebugLogger(t.options.DebugLogger)
		t.client.SetInfoLogger(t.options.InfoLogger)
		t.client.SetErrorLogger(t.options.ErrorLogger)
	}

	t.client.SetClientID(t.clientID())

	t.publishTicker = t.options.Clock.Ticker(time.Second * 2)

	t.client.SetCredentialsProvider(func() (username string, password string) {
		authToken, err := t.authToken()
		if err != nil {
			t.errorf("Error generating auth token: %v", err)
			return "", ""
		}
		return "unused", authToken
	})

	t.client.SetOnConnectHandler(func(client MQTTClient) {
		client.Subscribe(ctx, t.configTopic(), t.options.ConfigQOS, t.options.ConfigHandler)
	})

	err := t.client.Connect(ctx, servers...)
	if err != nil {
		return err
	}

	return err
}

// IsConnected returns true of the client is currently connected to MQTT server(s)
func (t *thing) IsConnected() bool {
	return t.client != nil && t.client.IsConnected()
}

// Disconnect from the MQTT server(s)
func (t *thing) Disconnect(ctx context.Context) {
	if t.client != nil {
		t.client.Unsubscribe(ctx, t.configTopic())
		if t.client.IsConnected() {
			t.infof("Disconnecting")
			t.client.Disconnect(ctx)
		}
	}
}

// Internal methods

func (t *thing) clientID() string {
	return fmt.Sprintf("projects/%s/locations/%s/registries/%s/devices/%s", t.options.ID.ProjectID, t.options.ID.Location, t.options.ID.Registry, t.options.ID.DeviceID)
}

func (t *thing) authToken() (string, error) {
	var signingMethod jwt.SigningMethod
	switch t.options.Credentials.Type {
	case CredentialTypeEC:
		signingMethod = jwt.GetSigningMethod("ES256")
	case CredentialTypeRSA:
		fallthrough
	default:
		signingMethod = jwt.GetSigningMethod("RS256")
	}

	wt := jwt.New(signingMethod)

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

func (t *thing) publish(ctx context.Context, topic string, message []byte, qos uint8) error {
	<-t.publishTicker.C // Don't publish more than once per second
	err := t.client.Publish(ctx, topic, qos, message)
	if err != nil {
		t.debugf("SEND FAILED - Topic: %s, Message Length: %d bytes, Error: %v", topic, len(message), err)
		return err
	}
	t.debugf("SENT - Topic: %s, Message Length: %d bytes", topic, len(message))
	return nil
}

func (t *thing) log(logger Logger, format string, v ...interface{}) {
	if logger != nil {
		msg := fmt.Sprintf(format, v...)
		logger(msg)
	}
}

func (t *thing) debugf(format string, v ...interface{}) {
	t.log(t.options.DebugLogger, format, v...)
}

func (t *thing) infof(format string, v ...interface{}) {
	t.log(t.options.InfoLogger, format, v...)
}

func (t *thing) errorf(format string, v ...interface{}) {
	t.log(t.options.ErrorLogger, format, v...)
}
