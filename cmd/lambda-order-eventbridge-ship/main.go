package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/getsentry/sentry-go"
	acmeserverless "github.com/retgits/acme-serverless"
	"github.com/retgits/acme-serverless-order/internal/datastore/dynamodb"
	"github.com/retgits/acme-serverless-order/internal/emitter/eventbridge"
	wflambda "github.com/wavefronthq/wavefront-lambda-go"
)

// handler handles the EventBridge events and returns an error if anything goes wrong.
// The resulting event, if no error is thrown, is sent to an EventBridge bus.
func handler(request json.RawMessage) error {
	// Initiialize a connection to Sentry to capture errors and traces
	sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
		Transport: &sentry.HTTPSyncTransport{
			Timeout: time.Second * 3,
		},
		ServerName:  os.Getenv("FUNCTION_NAME"),
		Release:     os.Getenv("VERSION"),
		Environment: os.Getenv("STAGE"),
	})

	req, err := acmeserverless.UnmarshalCreditCardValidatedEvent(request)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error unmarshalling creditcard validated event: %s", err.Error()))
		return err
	}

	shipmentStatus := acmeserverless.ShipmentData{
		OrderNumber: req.Data.OrderID,
		Status:      req.Data.Message,
	}

	dynamoStore := dynamodb.New()
	ord, err := dynamoStore.UpdateStatus(shipmentStatus)
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error updating shipment status: %s", err.Error()))
		return err
	}

	if req.Data.Success {
		em := eventbridge.New()
		evt := acmeserverless.ShipmentRequested{
			Metadata: acmeserverless.Metadata{
				Domain: acmeserverless.OrderDomain,
				Source: "ShipOrder",
				Type:   acmeserverless.ShipmentRequestedEventName,
				Status: "success",
			},
			Data: acmeserverless.ShipmentRequest{
				OrderID:  req.Data.OrderID,
				Delivery: ord.Delivery,
			},
		}

		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category:  acmeserverless.ShipmentRequestedEventName,
			Timestamp: time.Now().Unix(),
			Level:     sentry.LevelInfo,
			Data:      acmeserverless.ToSentryMap(evt.Data),
		})

		err = em.SendShipmentRequestedEvent(evt)
		if err != nil {
			sentry.CaptureException(fmt.Errorf("error sending ShipmentRequested event: %s", err.Error()))
			return err
		}

		sentry.CaptureMessage(fmt.Sprintf("shipment successfully requested for order [%s]", req.Data.OrderID))

	}

	return nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(wflambda.Wrapper(handler))
}
