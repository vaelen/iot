// Copyright 2018, Andrew C. Young
// License: MIT

package iot

func ExampleMockMQTTClient() {
	var mockClient *MockMQTTClient
	NewClient = func(t Thing, o *ThingOptions) MQTTClient {
		mockClient = NewMockClient(t, o)
		return mockClient
	}

	// Put your test code here

}
