package main

import (
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/kelseyhightower/envconfig"

	wflambda "github.com/wavefronthq/wavefront-lambda-go"
)

var wfAgent = wflambda.NewWavefrontAgent(&wflambda.WavefrontConfig{})

// config is the struct that is used to keep track of all environment variables
type config struct {
	AWSRegion     string `required:"true" split_words:"true" envconfig:"REGION"`
	DynamoDBTable string `required:"true" split_words:"true" envconfig:"TABLENAME"`
}

type Shipment struct {
	TrackingNumber string `json:"trackingNumber"`
	OrderNumber    string `json:"orderNumber"`
	Status         string `json:"status"`
}

// UnmarshalShipment takes a byte array and turns that into a Shipment
func UnmarshalShipment(data []byte) (Shipment, error) {
	var r Shipment
	err := json.Unmarshal(data, &r)
	return r, err
}

var c config

func handler(request events.SQSEvent) error {
	// Get configuration set using environment variables
	err := envconfig.Process("", &c)
	if err != nil {
		log.Printf("error starting function: %s", err.Error())
		return err
	}

	// Create an AWS session
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(c.AWSRegion),
	}))

	// Create a DynamoDB session
	dbs := dynamodb.New(awsSession)

	for _, record := range request.Records {
		msg, err := UnmarshalShipment([]byte(record.Body))
		if err != nil {
			log.Printf("error unmarshaling request: %s", err.Error())
			break
		}

		// Create a map of DynamoDB Attribute Values containing the table keys
		km := make(map[string]*dynamodb.AttributeValue)
		km["ID"] = &dynamodb.AttributeValue{
			S: aws.String(msg.OrderNumber),
		}

		em := make(map[string]*dynamodb.AttributeValue)
		em[":status"] = &dynamodb.AttributeValue{
			S: aws.String(msg.Status),
		}

		uii := &dynamodb.UpdateItemInput{
			TableName:                 aws.String(c.DynamoDBTable),
			Key:                       km,
			ExpressionAttributeValues: em,
			UpdateExpression:          aws.String("SET Status = :status"),
		}

		_, err = dbs.UpdateItem(uii)
		if err != nil {
			log.Printf("error updating dynamodb: %s", err.Error())
			return err
		}
	}

	return nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(wfAgent.WrapHandler(handler))
}
