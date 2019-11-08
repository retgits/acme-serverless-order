package main

import (
	"encoding/json"
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
	"github.com/kelseyhightower/envconfig"
	"github.com/retgits/creditcard"

	"github.com/gofrs/uuid"
	wflambda "github.com/wavefronthq/wavefront-lambda-go"
)

var wfAgent = wflambda.NewWavefrontAgent(&wflambda.WavefrontConfig{})

// config is the struct that is used to keep track of all environment variables
type config struct {
	AWSRegion     string `required:"true" split_words:"true" envconfig:"REGION"`
	DynamoDBTable string `required:"true" split_words:"true" envconfig:"TABLENAME"`
	PaymentQueue  string `required:"true" split_words:"true" envconfig:"PAYMENT_QUEUE"`
}

type PaymentRequest struct {
	OrderID string          `json:"orderID"`
	Card    creditcard.Card `json:"card"`
	Total   string          `json:"total"`
}

func (r *PaymentRequest) Marshal() (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func UnmarshalOrder(data string) (Order, error) {
	var r Order
	err := json.Unmarshal([]byte(data), &r)
	return r, err
}

type Order struct {
	OrderID   string   `json:"_id"`
	Status    *string  `json:"status,omitempty"`
	UserID    *string  `json:"userid,omitempty"`
	Firstname *string  `json:"firstname,omitempty"`
	Lastname  *string  `json:"lastname,omitempty"`
	Address   *Address `json:"address,omitempty"`
	Email     *string  `json:"email,omitempty"`
	Delivery  string   `json:"delivery"`
	Card      *Card    `json:"card,omitempty"`
	Cart      []Cart   `json:"cart"`
	Total     *string  `json:"total,omitempty"`
}

type Address struct {
	Street  *string `json:"street,omitempty"`
	City    *string `json:"city,omitempty"`
	Zip     *string `json:"zip,omitempty"`
	State   *string `json:"state,omitempty"`
	Country *string `json:"country,omitempty"`
}

type Card struct {
	Type     *string `json:"type,omitempty"`
	Number   *string `json:"number,omitempty"`
	ExpMonth *string `json:"expMonth,omitempty"`
	ExpYear  *string `json:"expYear,omitempty"`
	Ccv      *string `json:"ccv,omitempty"`
}

type Cart struct {
	ID          *string `json:"id,omitempty"`
	Description *string `json:"description,omitempty"`
	Quantity    *string `json:"quantity,omitempty"`
	Price       *string `json:"price,omitempty"`
}

var c config

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	response := events.APIGatewayProxyResponse{}

	// Get configuration set using environment variables
	err := envconfig.Process("", &c)
	if err != nil {
		errormessage := fmt.Sprintf("error starting function: %s", err.Error())
		log.Println(errormessage)
		response.StatusCode = http.StatusInternalServerError
		response.Body = errormessage
		return response, err
	}

	// Create an AWS session
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(c.AWSRegion),
	}))

	// Create a DynamoDB session
	dbs := dynamodb.New(awsSession)

	// Create the key attributes
	orderID := uuid.Must(uuid.NewV4()).String()
	userID := request.PathParameters["userid"]
	order, err := UnmarshalOrder(request.Body)
	if err != nil {
		errormessage := fmt.Sprintf("error unmarshalling order: %s", err.Error())
		log.Println(errormessage)
		response.StatusCode = http.StatusInternalServerError
		response.Body = errormessage
		return response, err
	}

	// Create a map of DynamoDB Attribute Values containing the table keys
	km := make(map[string]*dynamodb.AttributeValue)
	km["ID"] = &dynamodb.AttributeValue{
		S: aws.String(orderID),
	}
	km["User"] = &dynamodb.AttributeValue{
		S: aws.String(userID),
	}

	em := make(map[string]*dynamodb.AttributeValue)
	em[":content"] = &dynamodb.AttributeValue{
		S: aws.String(request.Body),
	}
	em[":status"] = &dynamodb.AttributeValue{
		S: aws.String("pending payment"),
	}

	uii := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(c.DynamoDBTable),
		Key:                       km,
		ExpressionAttributeValues: em,
		UpdateExpression:          aws.String("SET OrderStatus = :orderstatus, OrderString = :orderstring"),
	}

	_, err = dbs.UpdateItem(uii)
	if err != nil {
		errormessage := fmt.Sprintf("error updating dynamodb: %s", err.Error())
		log.Println(errormessage)
		response.StatusCode = http.StatusInternalServerError
		response.Body = errormessage
		return response, err
	}

	status := OrderStatus{
		OrderID: orderID,
		Userid:  userID,
		Payment: Payment{
			Message: "pending payment",
			Success: "false",
		},
	}

	payload, err := status.Marshal()
	if err != nil {
		errormessage := fmt.Sprintf("error creating response: %s", err.Error())
		log.Println(errormessage)
		response.StatusCode = http.StatusInternalServerError
		response.Body = errormessage
		return response, err
	}

	// Create a SQS session
	sqsService := sqs.New(awsSession)

	expMonth, _ := strconv.Atoi(*order.Card.ExpMonth)
	expYear, _ := strconv.Atoi(*order.Card.ExpYear)

	pr := PaymentRequest{
		OrderID: orderID,
		Total:   *order.Total,
		Card: creditcard.Card{
			Type:        *order.Card.Type,
			Number:      *order.Card.Number,
			ExpiryMonth: expMonth,
			ExpiryYear:  expYear,
			CVV:         *order.Card.Ccv,
		},
	}

	smiPayload, err := pr.Marshal()
	if err != nil {
		errormessage := fmt.Sprintf("error creating payment payload: %s", err.Error())
		log.Println(errormessage)
		response.StatusCode = http.StatusInternalServerError
		response.Body = errormessage
		return response, err
	}

	sendMessageInput := &sqs.SendMessageInput{
		QueueUrl:    aws.String(c.PaymentQueue),
		MessageBody: aws.String(smiPayload),
	}

	_, err = sqsService.SendMessage(sendMessageInput)
	if err != nil {
		errormessage := fmt.Sprintf("error sending payment payload: %s", err.Error())
		log.Println(errormessage)
		response.StatusCode = http.StatusInternalServerError
		response.Body = errormessage
		return response, err
	}

	response.StatusCode = http.StatusOK
	response.Body = string(payload)

	return response, nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(wfAgent.WrapHandler(handler))
}

func (r *OrderStatus) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type OrderStatus struct {
	OrderID string  `json:"order_id"`
	Payment Payment `json:"payment"`
	Userid  string  `json:"userid"`
}

type Payment struct {
	Amount        string `json:"amount"`
	Message       string `json:"message"`
	Success       string `json:"success"`
	TransactionID string `json:"transactionID"`
}
