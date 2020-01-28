package emitter

import (
	"encoding/json"

	"github.com/retgits/creditcard"
	"github.com/retgits/acme-serverless-order"
)

// PaymentRequestedEvent ...
type PaymentRequestedEvent struct {
	Metadata order.Metadata       `json:"metadata"`
	Data     PaymentRequestedData `json:"data"`
}

// PaymentRequestedData ...
type PaymentRequestedData struct {
	OrderID string          `json:"orderID"`
	Card    creditcard.Card `json:"card"`
	Total   string          `json:"total"`
}

// ShipmentRequestedEvent ...
type ShipmentRequestedEvent struct {
	Metadata order.Metadata        `json:"metadata"`
	Data     ShipmentRequestedData `json:"data"`
}

// ShipmentRequestedData ...
type ShipmentRequestedData struct {
	OrderID  string `json:"_id"`
	Delivery string `json:"delivery"`
}

// SentShipmentEvent ...
type SentShipmentEvent struct {
	Metadata order.Metadata   `json:"metadata"`
	Data     SentShipmentData `json:"data"`
}

// SentShipmentData ...
type SentShipmentData struct {
	TrackingNumber string `json:"trackingNumber"`
	OrderNumber    string `json:"orderNumber"`
	Status         string `json:"status"`
}

// EventEmitter ...
type EventEmitter interface {
	SendPaymentRequestedEvent(e PaymentRequestedEvent) error
	SendShipmentRequestedEvent(e ShipmentRequestedEvent) error
}

// Marshal returns the JSON encoding of PaymentRequestedEvent.
func (e *PaymentRequestedEvent) Marshal() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Marshal returns the JSON encoding of ShipmentRequestedEvent.
func (e *ShipmentRequestedEvent) Marshal() (string, error) {
	b, err := json.Marshal(e)
	if err != nil {
		return "", err
	}
	return string(b), nil
}
