// Copyright 2018, Andrew C. Young
// License: MIT

package examples

import (
	"fmt"
	"github.com/vaelen/iot"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"time"
)

type SensorReader struct {
	thing   iot.Thing
	stop    chan bool
	stopped chan error
	logger  iot.Logger
	wg      sync.WaitGroup
	cfg     string
}

func (sr *SensorReader) getTelemetry() []byte {
	cmd := exec.Command("/usr/bin/sensors")
	output, err := cmd.CombinedOutput()
	if err != nil {
		output = []byte(fmt.Sprintf("ERROR: %v", err))
	}
	return append([]byte(time.Now().String()), output...)
}

func (sr *SensorReader) updateConfig(config []byte) {
	sr.cfg = string(config)
}

func (sr *SensorReader) getState() []byte {
	return []byte("Config: " + sr.cfg)
}

func (sr *SensorReader) log(msg string) {
	if sr.logger != nil {
		sr.logger(msg)
	}
}

func NewSensorReader(id iot.ID, credentials *iot.Credentials, queueDirectory string, logger iot.Logger, logLevel iot.LogLevel, servers ...string) (*SensorReader, error) {

	sr := &SensorReader{
		stop:    make(chan bool),
		stopped: make(chan error),
		logger:  logger,
	}

	thing := iot.Thing{
		ID:          &id,
		Credentials: credentials,
		Logger:      logger,
		LogLevel:    logLevel,
		ConfigHandler: func(thing *iot.Thing, config []byte) {
			sr.log("Config Received, Sending State")
			sr.updateConfig(config)
			thing.PublishState(sr.getState())
		},
		QueueDirectory: queueDirectory,
		ConfigQOS:      1,
		StateQOS:       1,
		EventQOS:       1,
	}

	err := thing.Connect(servers...)
	if err != nil {
		return nil, err
	}

	sr.thing = thing

	sr.wg.Add(1)
	go sr.processingLoop()

	return sr, nil
}

func (sr *SensorReader) processingLoop() {
	defer sr.wg.Done()

	// Set up channel on which to send signal notifications.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	t := time.NewTicker(time.Second * 15)
	for {
		select {
		case <-t.C:
			sr.thing.PublishEvent(sr.getTelemetry())
		case <-sigc:
			// Disconnect the Network Connection.
			sr.thing.Disconnect()
			return
		case <-sr.stop:
			sr.thing.Disconnect()
			sr.stopped <- nil
			return
		}
	}
}

func (sr *SensorReader) Close() error {
	sr.stop <- true
	return <-sr.stopped
}

func (sr *SensorReader) Wait() {
	sr.wg.Wait()
}
