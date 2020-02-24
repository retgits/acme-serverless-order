package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/getsentry/sentry-go"
	order "github.com/retgits/acme-serverless-order"
	"github.com/retgits/acme-serverless-order/internal/datastore/dynamodb"
)

func handler(request events.SQSEvent) error {
	sentrySyncTransport := sentry.NewHTTPSyncTransport()
	sentrySyncTransport.Timeout = time.Second * 3

	sentry.Init(sentry.ClientOptions{
		Dsn:         os.Getenv("SENTRY_DSN"),
		Transport:   sentrySyncTransport,
		ServerName:  os.Getenv("FUNCTION_NAME"),
		Release:     os.Getenv("VERSION"),
		Environment: os.Getenv("STAGE"),
	})

	req, err := order.UnmarshalShipmentUpdateEvent([]byte(request.Records[0].Body))
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error unmarshalling shipment update event: %s", err.Error()))
		return err
	}

	log.Println(req)

	dynamoStore := dynamodb.New()
	_, err = dynamoStore.UpdateStatus(req.Data)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error updating shipment status for order [%s]: %s", req.Data.OrderNumber, err.Error()))
		return err
	}

	sentry.CaptureMessage(fmt.Sprintf("shipment status successfully updated for order [%s]", req.Data.OrderNumber))

	return nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(handler)
}
