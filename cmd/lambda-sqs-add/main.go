package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/gofrs/uuid"
	order "github.com/retgits/acme-serverless-order"
	"github.com/retgits/acme-serverless-order/internal/datastore/dynamodb"
	"github.com/retgits/acme-serverless-order/internal/emitter"
	"github.com/retgits/acme-serverless-order/internal/emitter/sqs"
)

func handleError(area string, err error) (events.APIGatewayProxyResponse, error) {
	msg := fmt.Sprintf("error %s: %s", area, err.Error())
	log.Println(msg)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       msg,
	}, err
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Update the order with an OrderID
	ord, err := order.UnmarshalOrder(request.Body)
	if err != nil {
		return handleError("unmarshal", err)
	}
	ord.OrderID = uuid.Must(uuid.NewV4()).String()

	dynamoStore := dynamodb.New()
	ord, err = dynamoStore.AddOrder(ord)
	if err != nil {
		return handleError("store", err)
	}

	prEvent := emitter.PaymentRequestedEvent{
		Metadata: order.Metadata{
			Domain: "Order",
			Source: "AddOrder",
			Type:   "PaymentRequested",
			Status: "success",
		},
		Data: emitter.PaymentRequestedData{
			OrderID: ord.OrderID,
			Card:    ord.Card,
			Total:   ord.Total,
		},
	}

	em := sqs.New()
	err = em.SendPaymentRequestedEvent(prEvent)
	if err != nil {
		return handleError("request payment", err)
	}

	status := order.OrderStatus{
		OrderID: ord.OrderID,
		Userid:  ord.UserID,
		Payment: order.PaymentStatus{
			Message: "pending payment",
			Success: "false",
		},
	}

	payload, err := status.Marshal()
	if err != nil {
		return handleError("response", err)
	}

	response := events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(payload),
	}

	return response, nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(handler)
}