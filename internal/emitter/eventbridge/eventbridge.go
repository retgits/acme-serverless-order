// Package eventbridge uses Amazon EventBridge as a serverless event bus that makes it easy to connect
// applications together using data from your own applications, integrated Software-as-a-Service (SaaS)
// applications, and Serverless Fitness Shops.
package eventbridge

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
	"github.com/retgits/acme-serverless-order/internal/emitter"
	payment "github.com/retgits/acme-serverless-payment"
	shipment "github.com/retgits/acme-serverless-shipment"
)

// responder is an empty struct that implements the methods of the
// EventEmitter interface
type responder struct{}

// New creates a new instance of the EventEmitter with EventBridge
// as the messaging layer.
func New() emitter.EventEmitter {
	return responder{}
}

func (r responder) SendPaymentRequestedEvent(e payment.PaymentRequested) error {
	payload, err := e.Marshal()
	if err != nil {
		return err
	}

	return send(string(payload), e.Metadata.Source)
}

func (r responder) SendShipmentRequestedEvent(e shipment.ShipmentRequested) error {
	payload, err := e.Marshal()
	if err != nil {
		return err
	}

	return send(string(payload), e.Metadata.Source)
}

func send(payload string, source string) error {
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	}))

	svc := eventbridge.New(awsSession)

	entries := make([]*eventbridge.PutEventsRequestEntry, 1)

	entries[0] = &eventbridge.PutEventsRequestEntry{
		Detail:       aws.String(payload),
		EventBusName: aws.String(os.Getenv("EVENTBUS")),
		Source:       aws.String(source),
	}

	event := &eventbridge.PutEventsInput{
		Entries: entries,
	}

	_, err := svc.PutEvents(event)
	if err != nil {
		return err
	}

	return nil
}
