// Copyright 2018, Andrew C. Young
// License: MIT

package iot_test

import (
	"github.com/vaelen/iot"
)

func ExampleMockMQTTClient() {
	var mockClient *iot.MockMQTTClient
	iot.NewClient = func(t iot.Thing, o *iot.ThingOptions) iot.MQTTClient {
		mockClient = iot.NewMockClient(t, o)
		return mockClient
	}

	// Put your test code here

}
