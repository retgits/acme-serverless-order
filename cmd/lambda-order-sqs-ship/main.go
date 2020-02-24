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
	"github.com/retgits/acme-serverless-order/internal/emitter"
	"github.com/retgits/acme-serverless-order/internal/emitter/sqs"
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

	req, err := order.UnmarshalCreditcardValidatedEvent([]byte(request.Records[0].Body))
	if err != nil {
		sentry.CaptureException(fmt.Errorf("error unmarshalling creditcard validated event: %s", err.Error()))
		return err
	}

	shipmentStatus := order.ShipmentStatus{
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
		evt := emitter.ShipmentRequestedEvent{
			Metadata: order.Metadata{
				Domain: "Order",
				Source: "ShipOrder",
				Type:   "ShipmentRequested",
				Status: "success",
			},
			Data: emitter.ShipmentRequestedData{
				OrderID:  req.Data.OrderID,
				Delivery: ord.Delivery,
			},
		}

		sentry.AddBreadcrumb(&sentry.Breadcrumb{
			Category:  "ShipmentRequested",
			Timestamp: time.Now().Unix(),
			Level:     sentry.LevelInfo,
			Data: map[string]interface{}{
				"OrderID":  req.Data.OrderID,
				"Delivery": ord.Delivery,
			},
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
	lambda.Start(handler)
}
