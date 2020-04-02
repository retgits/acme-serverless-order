package main

import (
	"fmt"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/getsentry/sentry-go"
	order "github.com/retgits/acme-serverless-order"
	"github.com/retgits/acme-serverless-order/internal/datastore/dynamodb"
	"github.com/retgits/acme-serverless-order/internal/emitter/sqs"
	payment "github.com/retgits/acme-serverless-payment"
	shipment "github.com/retgits/acme-serverless-shipment"
	wflambda "github.com/wavefronthq/wavefront-lambda-go"
)

func handler(request events.SQSEvent) error {
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

	req, err := payment.UnmarshalCreditCardValidated([]byte(request.Records[0].Body))
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error unmarshalling creditcard validated event: %s", err.Error()))
		return err
	}

	shipmentStatus := shipment.ShipmentData{
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
		em := sqs.New()
		evt := shipment.ShipmentRequested{
			Metadata: shipment.Metadata{
				Domain: order.Domain,
				Source: "ShipOrder",
				Type:   shipment.ShipmentRequestedEvent,
				Status: "success",
			},
			Data: shipment.ShipmentRequest{
				OrderID:  req.Data.OrderID,
				Delivery: ord.Delivery,
			},
		}

		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category:  shipment.ShipmentRequestedEvent,
			Timestamp: time.Now().Unix(),
			Level:     sentry.LevelInfo,
			Data:      evt.Data.ToMap(),
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
