package eventbridge

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/eventbridge"
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

	return send(payload, e.Metadata.Source)
}

func (r responder) SendShipmentRequestedEvent(e emitter.ShipmentRequestedEvent) error {
	payload, err := e.Marshal()
	if err != nil {
		return err
	}

	return send(payload, e.Metadata.Source)
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
