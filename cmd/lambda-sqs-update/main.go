package main

import (
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	order "github.com/retgits/acme-serverless-order"
	"github.com/retgits/acme-serverless-order/internal/datastore/dynamodb"
)

func handler(request events.SQSEvent) error {
	log.Println(request.Records[0].Body)

	req, err := order.UnmarshalShipmentUpdateEvent([]byte(request.Records[0].Body))
	if err != nil {
		return err
	}

	log.Println(req)

	dynamoStore := dynamodb.New()
	_, err = dynamoStore.UpdateStatus(req.Data)

	return err
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(handler)
}
