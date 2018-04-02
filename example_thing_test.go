// Copyright 2018, Andrew C. Young
// License: MIT

package iot

import (
	"context"
	"io/ioutil"
	"log"
)

func ExampleThing() {
	ctx := context.Background()

	// Your client must include the paho package
	// to use the default Eclipse Paho MQTT client.
	//
	// include 	_ "github.com/vaelen/iot/paho"

	id := &ID{
		DeviceID:  "deviceName",
		Registry:  "my-registry",
		Location:  "asia-east1",
		ProjectID: "my-project",
	}

	credentials, err := LoadRSACredentials("rsa_cert.pem", "rsa_private.pem")
	if err != nil {
		panic("Couldn't load credentials")
	}

	tmpDir, err := ioutil.TempDir("", "queue-")
	if err != nil {
		panic("Couldn't create temp directory")
	}

	options := DefaultOptions(id, credentials)
	options.DebugLogger = log.Println
	options.InfoLogger = log.Println
	options.ErrorLogger = log.Println
	options.QueueDirectory = tmpDir
	options.ConfigHandler = func(thing Thing, config []byte) {
		// Do something here to process the updated config and create an updated state string
		state := []byte("ok")
		thing.PublishState(ctx, state)
	}

	thing := New(options)

	err = thing.Connect(ctx, "ssl://mqtt.googleapis.com:443")
	if err != nil {
		panic("Couldn't connect to server")
	}
	defer thing.Disconnect(ctx)

	// This publishes to /events
	thing.PublishEvent(ctx, []byte("Top level telemetry event"))
	// This publishes to /events/a
	thing.PublishEvent(ctx, []byte("Sub folder telemetry event"), "a")
	// This publishes to /events/a/b
	thing.PublishEvent(ctx, []byte("Sub folder telemetry event"), "a", "b")
}
