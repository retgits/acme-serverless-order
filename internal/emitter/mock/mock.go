package mock

import (
	"log"

	"github.com/retgits/acme-serverless-order/internal/emitter"
)

type responder struct{}

func New() emitter.EventEmitter {
	return responder{}
}

func (r responder) SendPaymentRequestedEvent(e emitter.PaymentRequestedEvent) error {
	payload, err := e.Marshal()
	if err != nil {
		return err
	}

	log.Printf("Payload: %s", payload)

	return nil
}

func (r responder) SendShipmentRequestedEvent(e emitter.ShipmentRequestedEvent) error {
	payload, err := e.Marshal()
	if err != nil {
		return err
	}

	log.Printf("Payload: %s", payload)

	return nil
}
