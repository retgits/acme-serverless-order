package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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

	si := &dynamodb.ScanInput{
		TableName: aws.String(c.DynamoDBTable),
	}

	so, err := dbs.Scan(si)
	if err != nil {
		errormessage := fmt.Sprintf("error querrying dynamodb: %s", err.Error())
		log.Println(errormessage)
		response.StatusCode = http.StatusInternalServerError
		response.Body = errormessage
		return response, err
	}

	response.StatusCode = http.StatusOK

	orders := make(Orders, len(so.Items))

	for idx, order := range so.Items {
		str := order["OrderString"].S
		o, err := UnmarshalOrder(*str)
		if err != nil {
			fmt.Println(err.Error())
		}
		o.Status = order["OrderStatus"].S
		orders[idx] = o
	}

	payload, err := orders.Marshal()
	if err != nil {
		errormessage := fmt.Sprintf("error preparing output: %s", err.Error())
		log.Println(errormessage)
		response.StatusCode = http.StatusInternalServerError
		response.Body = errormessage
		return response, err
	}
	response.Body = string(payload)

	return response, nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(wfAgent.WrapHandler(handler))
}

type Orders []Order

func (r *Orders) Marshal() ([]byte, error) {
	return json.Marshal(r)
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
