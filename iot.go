// Copyright 2018, Andrew C. Young
// License: MIT

// Package iot provides a simple implementation of a Google IoT Core device.
//
// Note: Your application must import the iot/paho package to use the default Eclipse Paho MQTT client.
package iot

import (
	"context"
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/benbjohnson/clock"
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

// ErrCancelled is returned when a context is canceled or times out.
var ErrCancelled = fmt.Errorf("operation was cancelled or timed out")

// ClientConstructor defines a function for creating an MQTT client instance
type ClientConstructor func(thing Thing, options *ThingOptions) MQTTClient

// NewClient is the ClientConstructor used to create MQTT client instances
// Override this value during testing to provide an MQTT client mock implementation
var NewClient ClientConstructor

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

// CredentialType defines the key type of the credential key pair.
type CredentialType uint8

const (
	// CredentialTypeRSA specfies that the credentials use RSA keys
	CredentialTypeRSA CredentialType = 0
	// CredentialTypeEC specifies that the credentials use Eliptic Curve keys
	CredentialTypeEC CredentialType = 1
)

// Credentials wraps the public and private key for a device
type Credentials struct {
	Type        CredentialType
	Certificate tls.Certificate
	PrivateKey  interface{}
}

// LoadRSACredentials creates a Credentials struct from the given RSA private key and certificate
func LoadRSACredentials(certificatePath string, privateKeyPath string) (*Credentials, error) {
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
		Type:        CredentialTypeRSA,
		Certificate: certificate,
		PrivateKey:  privateKey,
	}, nil
}

// LoadECCredentials creates a Credentials struct from the given EC private key and certificate
func LoadECCredentials(certificatePath string, privateKeyPath string) (*Credentials, error) {
	signBytes, err := ioutil.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	privateKey, err := jwt.ParseECPrivateKeyFromPEM(signBytes)
	if err != nil {
		return nil, err
	}

	certificate, err := tls.LoadX509KeyPair(certificatePath, privateKeyPath)
	if err != nil {
		return nil, err
	}

	return &Credentials{
		Type:        CredentialTypeEC,
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
	// Clock represents the system clock.
	// This value can be overriden for testing purposes.
	// If not provided, this will default to the regular system clock.
	Clock clock.Clock
}

// Thing represents an IoT device
type Thing interface {
	// PublishState publishes the current device state
	PublishState(ctx context.Context, message []byte) error

	// PublishEvent publishes an event. An optional hierarchy of event names can be provided.
	PublishEvent(ctx context.Context, message []byte, event ...string) error

	// Connect to the given MQTT server(s)
	Connect(ctx context.Context, servers ...string) error

	// IsConnected returns true of the client is currently connected to MQTT server(s)
	IsConnected() bool

	// Disconnect from the MQTT server(s)
	Disconnect(ctx context.Context)
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

// MQTTCredentialsProvider should return the current username and password for the MQTT client to use.
type MQTTCredentialsProvider func() (username string, password string)

// The MQTTClient interface represents an underlying MQTT client implementation in an abstract way.
type MQTTClient interface {
	// IsConnected should return true when the client is connected to the server
	IsConnected() bool

	// Connect should connect to the given MQTT server
	Connect(ctx context.Context, servers ...string) error

	// Disconnect should disconnect from the given MQTT server and clean up all client resources
	Disconnect(ctx context.Context) error

	// Publish should publish the given payload to the given topic with the given quality of service level
	Publish(ctx context.Context, topic string, qos uint8, payload interface{}) error

	// Subscribe should subscribe to the given topic with the given quality of service level and message handler
	Subscribe(ctx context.Context, topic string, qos uint8, callback ConfigHandler) error

	// Unsubscribe should unsubscribe from the given topic
	Unsubscribe(ctx context.Context, topic string) error

	// SetDebugLogger should set the logger to use for logging debug messages
	SetDebugLogger(logger Logger)

	// SetInfoLogger should set the logger to use for logging information or warning messages
	SetInfoLogger(logger Logger)

	// SetErrorLogger should set the logger to use for logging error or critical messages
	SetErrorLogger(logger Logger)

	// SetClientID should set the MQTT client id.
	SetClientID(clientID string)

	// SetCredentialsProvider should set the CredentialsProvider used by the MQTT client
	SetCredentialsProvider(crendentialsProvider MQTTCredentialsProvider)
}
