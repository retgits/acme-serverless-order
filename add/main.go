package main

import (
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/gofrs/uuid"
	"github.com/kelseyhightower/envconfig"
	"github.com/retgits/creditcard"
	"github.com/retgits/order"
	wflambda "github.com/wavefronthq/wavefront-lambda-go"
)

var wfAgent = wflambda.NewWavefrontAgent(&wflambda.WavefrontConfig{})

// config is the struct that is used to keep track of all environment variables
type config struct {
	AWSRegion     string `required:"true" split_words:"true" envconfig:"REGION"`
	DynamoDBTable string `required:"true" split_words:"true" envconfig:"TABLENAME"`
	PaymentQueue  string `required:"true" split_words:"true" envconfig:"PAYMENT_QUEUE"`
}

var c config

func logError(stage string, err error) (events.APIGatewayProxyResponse, error) {
	errormessage := fmt.Sprintf("error %s: %s", stage, err.Error())
	log.Println(errormessage)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusInternalServerError,
		Body:       errormessage,
	}, err
}

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	response := events.APIGatewayProxyResponse{}

	// Get configuration set using environment variables
	err := envconfig.Process("", &c)
	if err != nil {
		return logError("starting function", err)
	}

	// Create an AWS session
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(c.AWSRegion),
	}))

	// Create a DynamoDB session
	dbs := dynamodb.New(awsSession)

	// Update the order with an OrderID
	ord, err := order.UnmarshalOrder(request.Body)
	if err != nil {
		return logError("unmarshalling order", err)
	}
	ord.OrderID = uuid.Must(uuid.NewV4()).String()

	// Marshal the newly updated order struct
	ordPl, err := ord.Marshal()
	if err != nil {
		return logError("marshalling order", err)
	}

	// Create a map of DynamoDB Attribute Values containing the table keys
	km := make(map[string]*dynamodb.AttributeValue)
	km["ID"] = &dynamodb.AttributeValue{
		S: aws.String(ord.OrderID),
	}

	em := make(map[string]*dynamodb.AttributeValue)
	em[":content"] = &dynamodb.AttributeValue{
		S: aws.String(ordPl),
	}
	em[":status"] = &dynamodb.AttributeValue{
		S: aws.String("pending payment"),
	}
	em[":user"] = &dynamodb.AttributeValue{
		S: aws.String(ord.UserID),
	}

	uii := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(c.DynamoDBTable),
		Key:                       km,
		ExpressionAttributeValues: em,
		UpdateExpression:          aws.String("SET OrderStatus = :status, OrderString = :content, UserID = :user"),
	}

	_, err = dbs.UpdateItem(uii)
	if err != nil {
		return logError("updating dynamodb", err)
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
		return logError("creating orderstatus", err)
	}

	// Create a SQS session
	sqsService := sqs.New(awsSession)

	expMonth, _ := strconv.Atoi(ord.Card.ExpMonth)
	expYear, _ := strconv.Atoi(ord.Card.ExpYear)

	pr := order.PaymentRequest{
		OrderID: ord.OrderID,
		Total:   ord.Total,
		Card: creditcard.Card{
			Type:        ord.Card.Type,
			Number:      ord.Card.Number,
			ExpiryMonth: expMonth,
			ExpiryYear:  expYear,
			CVV:         ord.Card.Ccv,
		},
	}

	smiPayload, err := pr.Marshal()
	if err != nil {
		return logError("marshalling paymentrequest", err)
	}

	sendMessageInput := &sqs.SendMessageInput{
		QueueUrl:    aws.String(c.PaymentQueue),
		MessageBody: aws.String(smiPayload),
	}

	_, err = sqsService.SendMessage(sendMessageInput)
	if err != nil {
		return logError("sending paymentmessage", err)
	}

	response.StatusCode = http.StatusOK
	response.Body = string(payload)

	return response, nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(wfAgent.WrapHandler(handler))
}
