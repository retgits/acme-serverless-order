package main

import (
	"encoding/json"
	"log"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/kelseyhightower/envconfig"

	wflambda "github.com/wavefronthq/wavefront-lambda-go"
)

var wfAgent = wflambda.NewWavefrontAgent(&wflambda.WavefrontConfig{})

// config is the struct that is used to keep track of all environment variables
type config struct {
	AWSRegion     string `required:"true" split_words:"true" envconfig:"REGION"`
	DynamoDBTable string `required:"true" split_words:"true" envconfig:"TABLENAME"`
	ShippingQueue string `required:"true" split_words:"true" envconfig:"SHIPPING_QUEUE"`
}

// Response is the output message that the Lambda function receives from payment. It will be a JSON string payload.
type PaymentResponse struct {
	Success       bool   `json:"success"`
	Status        int    `json:"status"`
	Message       string `json:"message"`
	Amount        string `json:"amount,omitempty"`
	TransactionID string `json:"transactionID"`
	OrderID       string `json:"orderID"`
}

func UnmarshalPaymentResponse(data []byte) (PaymentResponse, error) {
	var r PaymentResponse
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

	// Create a SQS session
	sqsService := sqs.New(awsSession)
	// Create a DynamoDB session
	dbs := dynamodb.New(awsSession)

	for _, record := range request.Records {
		msg, err := UnmarshalPaymentResponse([]byte(record.Body))
		if err != nil {
			log.Printf("error unmarshaling request: %s", err.Error())
			break
		}

		// Create a map of DynamoDB Attribute Values containing the table keys
		km := make(map[string]*dynamodb.AttributeValue)
		km["ID"] = &dynamodb.AttributeValue{
			S: aws.String(msg.OrderID),
		}

		status := "payment successful - pending shipment"
		if !msg.Success {
			status = msg.Message
		}

		em := make(map[string]*dynamodb.AttributeValue)
		em[":status"] = &dynamodb.AttributeValue{
			S: aws.String(status),
		}

		uii := &dynamodb.UpdateItemInput{
			TableName:                 aws.String(c.DynamoDBTable),
			Key:                       km,
			ExpressionAttributeValues: em,
			UpdateExpression:          aws.String("SET OrderStatus = :status"),
			ReturnValues:              aws.String("ALL_NEW"),
		}

		uio, err := dbs.UpdateItem(uii)
		if err != nil {
			log.Printf("error updating dynamodb: %s", err.Error())
			return err
		}

		if msg.Success {
			sendMessageInput := &sqs.SendMessageInput{
				QueueUrl:    aws.String(c.ShippingQueue),
				MessageBody: uio.Attributes["OrderString"].S,
			}

			_, err = sqsService.SendMessage(sendMessageInput)
			if err != nil {
				log.Printf("error while sending response message: %s", err.Error())
				break
			}
		}
	}

	return nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(wfAgent.WrapHandler(handler))
}
