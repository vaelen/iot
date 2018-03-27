// Copyright 2018, Andrew C. Young
// License: MIT

// Package iot provides a simple implementation of a Google IoT Core device.
package iot

import (
	"crypto/rsa"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/dgrijalva/jwt-go"
)

// DefaultAuthTokenExpiration is the default value for Thing.AuthTokenExpiration
const DefaultAuthTokenExpiration = time.Hour

// ErrNotConnected is returned if a message is published but the client is not connected
var ErrNotConnected = fmt.Errorf("not connected")

// ErrPublishFailed is returned if the client was unable to send the message
var ErrPublishFailed = fmt.Errorf("could not publish message")

// ErrConfigurationError is returned from Connect() if either the ID or Credentials have not been set.
var ErrConfigurationError = fmt.Errorf("required configuration values are mising")

// ConfigHandler handles configuration updates received from the server.
type ConfigHandler func(thing Thing, config []byte)

// Logger is used to write log output.  If no Logger is provided, no logging will be performed.
type Logger func(args ...interface{})

// ID represents the various components that uniquely identify this device
type ID struct {
	ProjectID string
	Location  string
	Registry  string
	DeviceID  string
}

// Credentials wraps the public and private key for a device
type Credentials struct {
	Certificate tls.Certificate
	PrivateKey  *rsa.PrivateKey
}

// LoadCredentials creates a Credentials struct from the given private key and certificate
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

// ThingOptions holds the options that are used to create a Thing
type ThingOptions struct {
	// ID identifies this device.
	// This value is required.
	ID *ID
	// Credentials are used to authenticate with the server.
	// This value is required.
	Credentials *Credentials
	// DebugLogger is used to print debug level log output.
	// If no Logger is provided, no logging will occur.
	DebugLogger Logger
	// InfoLogger is used to print info level log output.
	// If no Logger is provided, no logging will occur.
	InfoLogger Logger
	// ErrorLogger is used to print error level log output.
	// If no Logger is provided, no logging will occur.
	ErrorLogger Logger
	// LogMQTT enables logging of the underlying MQTT client.
	// If enabled, the underlying MQTT client will log at the same level as the Thing itself (WARN, DEBUG, etc).
	LogMQTT bool
	// QueueDirectory should be a directory writable by the process.
	// If not provided, message queues will not be persisted between restarts.
	QueueDirectory string
	// ConfigHandler will be called when a new configuration document is received from the server.
	ConfigHandler ConfigHandler
	// ConfigQOS sets the QoS level for receiving config updates.
	// The default value will only perform best effort delivery.
	// The suggested value is 2.
	ConfigQOS uint8
	// StateQOS sets the QoS level for sending state updates.
	// The default value will only perform best effort delivery.
	// The suggested value is 1.
	// Google does not allow a value of 2 here.
	StateQOS uint8
	// EventQOS sets the QoS level for sending event updates.
	// The default value will only perform best effort delivery.
	// The suggested value is 1.
	// Google does not allow a value of 2 here.
	EventQOS uint8
	// AuthTokenExpiration determines how often a new auth token must be generated.
	// The minimum value is 10 minutes and the maximum value is 24 hours.
	// The default value is 1 hour.
	AuthTokenExpiration time.Duration
}

// Thing represents an IoT device
type Thing interface {
	// PublishState publishes the current device state
	PublishState(message []byte) error

	// PublishEvent publishes an event. An optional hierarchy of event names can be provided.
	PublishEvent(message []byte, event ...string) error

	// Connect to the given MQTT server(s)
	Connect(servers ...string) error

	// IsConnected returns true of the client is currently connected to MQTT server(s)
	IsConnected() bool

	// Disconnect from the MQTT server(s)
	Disconnect()
}

// DefaultOptions returns the default set of options.
func DefaultOptions(id *ID, credentials *Credentials) *ThingOptions {
	return &ThingOptions{
		ID:                  id,
		Credentials:         credentials,
		ConfigQOS:           2,
		StateQOS:            1,
		EventQOS:            1,
		AuthTokenExpiration: DefaultAuthTokenExpiration,
	}
}

// New returns a new Thing using the given options.
func New(options *ThingOptions) Thing {
	return &thing{options: options}
}
