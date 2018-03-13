// Copyright 2018, Andrew C. Young
// License: MIT

package main

import (
	"fmt"
	"github.com/vaelen/iot"
	"github.com/vaelen/iot/examples"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"log"
	"os"
)

func logMessage(msg string) {
	log.Println(msg)
}

func handleError(description string, err error) {
	if err != nil {
		log.Fatalf("%s: %v\n", description, err)
	}
}

type Config struct {
	ID          iot.ID
	Certificate string
	PrivateKey  string
	Server      string
}

func main() {

	configFile := "config.yaml"
	if len(os.Args) > 1 {
		configFile = os.Args[1]
	}

	configBytes, err := ioutil.ReadFile(configFile)
	handleError("Couldn't read config", err)

	config := &Config{}
	err = yaml.Unmarshal(configBytes, config)
	handleError("Couldn't parse config", err)

	logMessage(fmt.Sprintf("Config: %+v", config))

	credentials, err := iot.LoadCredentials(config.Certificate, config.PrivateKey)
	handleError("Couldn't load credentials", err)

	queueDirectory, err := ioutil.TempDir("", "iot-queue-")
	handleError("Couldn't create queue directory", err)

	sr, err := examples.NewSensorReader(config.ID, credentials, queueDirectory, logMessage, iot.LogLevelDebug, config.Server)
	handleError("Couldn't start sensor reader", err)

	sr.Wait()

}
