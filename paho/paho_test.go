// Copyright 2018, Andrew C. Young
// License: MIT

package paho

import (
	"context"
	"log"
	"testing"

	"github.com/vaelen/iot"
)

var ID = &iot.ID{
	DeviceID:  "vaelen_iot_test",
	Registry:  "x",
	Location:  "y",
	ProjectID: "z",
}

var ConfigTopic = "/devices/vaelen_iot_test/config"

// This test is here mainly for coverage.
// The functionality is tested in the main iot package.
func TestPahoClient(t *testing.T) {
	ctx := context.Background()

	var mqttClient *MQTTClient

	iot.NewClient = func(t iot.Thing, o *iot.ThingOptions) iot.MQTTClient {
		mqttClient = NewClient(t, o).(*MQTTClient)
		return mqttClient
	}

	options := getOptions(t)

	thing := iot.New(options)

	err := thing.Connect(ctx, "tcp://iot.eclipse.org:1883")
	if err != nil {
		panic("Couldn't connect to server")
	}
	defer thing.Disconnect(ctx)

	mqttClient.Publish(ctx, ConfigTopic, 0, []byte("test config"))

	// This publishes to /events
	thing.PublishEvent(ctx, []byte("Top level telemetry event"))
	// This publishes to /events/a
	thing.PublishEvent(ctx, []byte("Sub folder telemetry event"), "a")
	// This publishes to /events/a/b
	thing.PublishEvent(ctx, []byte("Sub folder telemetry event"), "a", "b")
}

func getOptions(t *testing.T) *iot.ThingOptions {
	ctx := context.Background()

	credentials, err := iot.LoadCredentials("../test_keys/rsa_cert.pem", "../test_keys/rsa_private.pem")
	if err != nil {
		t.Fatal("Couldn't load credentials")
	}

	options := iot.DefaultOptions(ID, credentials)
	options.LogMQTT = true
	options.DebugLogger = log.Println
	options.InfoLogger = log.Println
	options.ErrorLogger = log.Println
	options.ConfigHandler = func(thing iot.Thing, config []byte) {
		state := []byte("ok")
		thing.PublishState(ctx, state)
	}

	return options
}
