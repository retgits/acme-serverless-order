// Package mock uses the log file to log all incoming events.
// This is useful for testing, but doesn't send any events to other
// services. That means if you use this in a non-testing scenario
// the event flow will stop here.
package mock

import (
	"log"

	acmeserverless "github.com/retgits/acme-serverless"
	"github.com/retgits/acme-serverless-order/internal/emitter"
)

// responder is an empty struct that implements the methods of the
// EventEmitter interface.
type responder struct{}

// New creates a new instance of the EventEmitter with mock
// as the messaging layer.
func New() emitter.EventEmitter {
	return responder{}
}

func (r responder) SendPaymentRequestedEvent(e acmeserverless.PaymentRequestedEvent) error {
	payload, err := e.Marshal()
	if err != nil {
		return err
	}

	log.Printf("Payload: %s", payload)

	return nil
}

func (r responder) SendShipmentRequestedEvent(e acmeserverless.ShipmentRequested) error {
	payload, err := e.Marshal()
	if err != nil {
		return err
	}

	log.Printf("Payload: %s", payload)

	return nil
}
