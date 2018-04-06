IoT
===

A simple framework for implementing a Google IoT device.

This package makes use of the [context package] to handle request cancelation, timeouts, and deadlines.

[![gocover.io](https://gocover.io/_badge/github.com/vaelen/iot)](https://gocover.io/github.com/vaelen/iot)
[![Go Report Card](https://goreportcard.com/badge/github.com/vaelen/iot)](https://goreportcard.com/report/github.com/vaelen/iot)
[![Go Docs](https://godoc.org/github.com/vaelen/iot?status.svg)](https://godoc.org/github.com/vaelen/iot)

Copyright 2018, Andrew C. Young <<andrew@vaelen.org>>

License: [MIT]

Here is an example showing how to use this library:
```go
package main

import (
	"context"
	"log"
	"github.com/vaelen/iot"
	// Your client must include the paho package
	// to use the default Eclipse Paho MQTT client.
	_ "github.com/vaelen/iot/paho"
)

func main() {
	ctx := context.Background()

	id := &iot.ID{
		DeviceID:  "deviceName",
		Registry:  "my-registry",
		Location:  "asia-east1",
		ProjectID: "my-project",
	}

	credentials, err := iot.LoadRSACredentials("rsa_cert.pem", "rsa_private.pem")
	if err != nil {
		panic("Couldn't load credentials")
	}

	options := iot.DefaultOptions(id, credentials)
	options.DebugLogger = log.Println
	options.InfoLogger = log.Println
	options.ErrorLogger = log.Println
	options.ConfigHandler = func(thing iot.Thing, config []byte) {
		// Do something here to process the updated config and create an updated state string
		state := []byte("ok")
		thing.PublishState(ctx, state)
	}

	thing := iot.New(options)

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
```

Thanks to [Infostellar] for supporting my development of this project.

[Andrew C. Young]: http;//vaelen.org
[Infostellar]: http://infostellar.net
[context package]: https://golang.org/pkg/context/
[MIT]: ../blob/master/LICENSE
