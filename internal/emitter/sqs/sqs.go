package sqs

import (
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
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

	return send(payload)
}

func (r responder) SendShipmentRequestedEvent(e emitter.ShipmentRequestedEvent) error {
	payload, err := e.Marshal()
	if err != nil {
		return err
	}

	return send(payload)
}

func send(payload string) error {
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	}))

	svc := sqs.New(awsSession)

	sendMessageInput := &sqs.SendMessageInput{
		QueueUrl:    aws.String(os.Getenv("RESPONSEQUEUE")),
		MessageBody: aws.String(payload),
	}

	_, err := svc.SendMessage(sendMessageInput)
	if err != nil {
		return err
	}

	return nil
}
