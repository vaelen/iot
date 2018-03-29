// Copyright 2018, Andrew C. Young
// License: MIT

package examples

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"time"

	"github.com/vaelen/iot"
)

// SensorReader is a Google IoT Core device that reads device sensors
type SensorReader struct {
	thing   iot.Thing
	stop    chan bool
	stopped chan error
	logger  iot.Logger
	wg      sync.WaitGroup
	cfg     string
	command string
}

func (sr *SensorReader) getTelemetry() []byte {
	cmd := exec.Command(sr.command)
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

// NewSensorReader creates a new sensor reader
func NewSensorReader(id *iot.ID, credentials *iot.Credentials, queueDirectory string, logger iot.Logger, servers ...string) (*SensorReader, error) {
	ctx := context.Background()

	sr := &SensorReader{
		stop:    make(chan bool),
		stopped: make(chan error),
		logger:  logger,
		command: "/usr/bin/sensors",
	}

	options := iot.DefaultOptions(id, credentials)
	options.DebugLogger = logger
	options.InfoLogger = logger
	options.ErrorLogger = logger
	options.QueueDirectory = queueDirectory
	options.ConfigHandler = func(thing iot.Thing, config []byte) {
		sr.log("Config Received, Sending State")
		sr.updateConfig(config)
		thing.PublishState(ctx, sr.getState())
	}

	thing := iot.New(options)

	err := thing.Connect(ctx, servers...)
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

	ctx := context.Background()

	// Set up channel on which to send signal notifications.
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill)

	t := time.NewTicker(time.Second * 15)
	for {
		select {
		case <-t.C:
			sr.thing.PublishEvent(ctx, sr.getTelemetry())
		case <-sigc:
			// Disconnect the Network Connection.
			sr.thing.Disconnect(ctx)
			return
		case <-sr.stop:
			sr.thing.Disconnect(ctx)
			sr.stopped <- nil
			return
		}
	}
}

// Close shuts down the SensorReader
func (sr *SensorReader) Close() error {
	sr.stop <- true
	return <-sr.stopped
}

// Wait blocks until the SensorReader has stopped
func (sr *SensorReader) Wait() {
	sr.wg.Wait()
}
