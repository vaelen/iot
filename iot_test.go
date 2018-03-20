// Copyright 2018, Andrew C. Young
// License: MIT

package iot_test

import (
	"github.com/vaelen/iot"
	"testing"
)

const CertificatePath = "test_keys/rsa_cert.pem"
const PrivateKeyPath = "test_keys/rsa_private.pem"

var TestID = &iot.ID{
	ProjectID: "test-project",
	Location: "test-location",
	Registry: "test-registry",
	DeviceID: "test-device",
}

func TestLoadCredentials(t *testing.T) {
	credentials, err := iot.LoadCredentials(CertificatePath, PrivateKeyPath)
	if err != nil {
		t.Fatalf("Couldn't load credentials: %v", err)
	}
	if credentials == nil {
		t.Fatal("Credentials not loaded.")
	}
	if credentials.PrivateKey == nil {
		t.Fatal("Private key not loaded.")
	}
}

func TestDefaultOptions(t *testing.T) {
	credentials, err := iot.LoadCredentials(CertificatePath, PrivateKeyPath)
	if err != nil {
		t.Fatalf("Couldn't load credentials: %v", err)
	}

	options := iot.DefaultOptions(TestID, credentials)
	if options == nil {
		t.Fatal("Thing wasn't returned")
	}
	if options.ID != TestID {
		t.Fatal("Incorrect ID")
	}
	if options.Credentials != credentials {
		t.Fatal("Incorrect credentials")
	}
	if options.EventQOS != 1 {
		t.Fatalf("Incorrect event QoS: %v", options.EventQOS)
	}
	if options.StateQOS != 1 {
		t.Fatalf("Incorrect state QoS: %v", options.StateQOS)
	}
	if options.ConfigQOS != 2 {
		t.Fatalf("Incorrect config QoS: %v", options.ConfigQOS)
	}
	if options.AuthTokenExpiration != iot.DefaultAuthTokenExpiration {
		t.Fatalf("Incorrect auth token expiration: %v", options.AuthTokenExpiration)
	}
}

