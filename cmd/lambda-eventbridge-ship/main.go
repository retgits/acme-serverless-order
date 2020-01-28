package main

import (
	"encoding/json"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/retgits/acme-serverless-order"
	"github.com/retgits/acme-serverless-order/internal/datastore/dynamodb"
	"github.com/retgits/acme-serverless-order/internal/emitter"
	"github.com/retgits/acme-serverless-order/internal/emitter/eventbridge"
)

func handler(request json.RawMessage) error {
	req, err := order.UnmarshalCreditcardValidatedEvent(request)
	if err != nil {
		return err
	}

	shipmentStatus := order.ShipmentStatus{
		OrderNumber: req.Data.OrderID,
		Status:      req.Data.Message,
	}

	dynamoStore := dynamodb.New()
	ord, err := dynamoStore.UpdateStatus(shipmentStatus)
	if err != nil {
		return err
	}

	if req.Data.Success {
		em := eventbridge.New()
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

		return em.SendShipmentRequestedEvent(evt)
	}

	return nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(handler)
}
