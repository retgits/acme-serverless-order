// Package dynamodb leverages Amazon DynamoDB, a key-value and document database that delivers single-digit millisecond
// performance at any scale to store data.
package dynamodb

import (
	"fmt"
	"log"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/gofrs/uuid"
	acmeserverless "github.com/retgits/acme-serverless"
	"github.com/retgits/acme-serverless-order/internal/datastore"
)

// The pointer to DynamoDB provides the API operation methods for making requests to Amazon DynamoDB.
// This specifically creates a single instance of the dynamoDB service which can be reused if the
// container stays warm.
var dbs *dynamodb.DynamoDB

// manager is an empty struct that implements the methods of the
// Manager interface.
type manager struct{}

// init creates the connection to dynamoDB. If the environment variable
// DYNAMO_URL is set, the connection is made to that URL instead of
// relying on the AWS SDK to provide the URL
func init() {
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	}))

	if len(os.Getenv("DYNAMO_URL")) > 0 {
		awsSession.Config.Endpoint = aws.String(os.Getenv("DYNAMO_URL"))
	}

	dbs = dynamodb.New(awsSession)
}

// New creates a new datastore manager using Amazon DynamoDB as backend
func New() datastore.Manager {
	return manager{}
}

// AddOrder stores a new order in Amazon DynamoDB
func (m manager) AddOrder(o acmeserverless.Order) (acmeserverless.Order, error) {
	// Generate and assign a new orderID
	o.OrderID = uuid.Must(uuid.NewV4()).String()
	o.Status = aws.String("Pending Payment")

	// Marshal the newly updated product struct
	payload, err := o.Marshal()
	if err != nil {
		return o, fmt.Errorf("error marshalling order: %s", err.Error())
	}

	// Create a map of DynamoDB Attribute Values containing the table keys
	km := make(map[string]*dynamodb.AttributeValue)
	km["PK"] = &dynamodb.AttributeValue{
		S: aws.String("ORDER"),
	}
	km["SK"] = &dynamodb.AttributeValue{
		S: aws.String(o.OrderID),
	}

	// Create a map of DynamoDB Attribute Values containing the table data elements
	em := make(map[string]*dynamodb.AttributeValue)
	em[":keyid"] = &dynamodb.AttributeValue{
		S: aws.String(o.UserID),
	}
	em[":payload"] = &dynamodb.AttributeValue{
		S: aws.String(string(payload)),
	}

	uii := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(os.Getenv("TABLE")),
		Key:                       km,
		ExpressionAttributeValues: em,
		UpdateExpression:          aws.String("SET Payload = :payload, KeyID = :keyid"),
	}

	_, err = dbs.UpdateItem(uii)
	if err != nil {
		return o, fmt.Errorf("error updating dynamodb: %s", err.Error())
	}

	return o, nil
}

// AllOrders retrieves all orders from DynamoDB
func (m manager) AllOrders() (acmeserverless.Orders, error) {
	// Create a map of DynamoDB Attribute Values containing the table keys
	// for the access pattern PK = ORDER
	km := make(map[string]*dynamodb.AttributeValue)
	km[":type"] = &dynamodb.AttributeValue{
		S: aws.String("ORDER"),
	}

	// Create the QueryInput
	qi := &dynamodb.QueryInput{
		TableName:                 aws.String(os.Getenv("TABLE")),
		KeyConditionExpression:    aws.String("PK = :type"),
		ExpressionAttributeValues: km,
	}

	qo, err := dbs.Query(qi)
	if err != nil {
		return nil, err
	}

	orders := make(acmeserverless.Orders, len(qo.Items))

	for idx, ord := range qo.Items {
		str := ord["OrderString"].S
		o, err := acmeserverless.UnmarshalOrder(*str)
		if err != nil {
			log.Println(fmt.Sprintf("error unmarshalling order data: %s", err.Error()))
			continue
		}
		orders[idx] = o
	}

	return orders, nil
}

// UserOrders retrieves orders for a single user from DynamoDB based on the userID
func (m manager) UserOrders(userID string) (acmeserverless.Orders, error) {
	// Create a map of DynamoDB Attribute Values containing the table keys
	// for the access pattern PK = USER KeyID = ID
	km := make(map[string]*dynamodb.AttributeValue)
	km[":type"] = &dynamodb.AttributeValue{
		S: aws.String("ORDER"),
	}
	km[":userid"] = &dynamodb.AttributeValue{
		S: aws.String(userID),
	}

	// Create the QueryInput
	qi := &dynamodb.QueryInput{
		TableName:                 aws.String(os.Getenv("TABLE")),
		KeyConditionExpression:    aws.String("PK = :type"),
		FilterExpression:          aws.String("KeyID = :username"),
		ExpressionAttributeValues: km,
	}

	// Execute the DynamoDB query
	qo, err := dbs.Query(qi)
	if err != nil {
		return acmeserverless.Orders{}, err
	}

	orders := make(acmeserverless.Orders, len(qo.Items))

	for idx, ord := range qo.Items {
		str := ord["Payload"].S
		o, err := acmeserverless.UnmarshalOrder(*str)
		if err != nil {
			log.Println(fmt.Sprintf("error unmarshalling order data: %s", err.Error()))
			continue
		}
		orders[idx] = o
	}

	return orders, nil
}

// UpdateStatus sets thew new OrderStatus for a specific order
func (m manager) UpdateStatus(s acmeserverless.ShipmentData) (acmeserverless.Order, error) {
	// Create a map of DynamoDB Attribute Values containing the table keys
	// for the access pattern PK = ORDER SK = ID
	km := make(map[string]*dynamodb.AttributeValue)
	km[":type"] = &dynamodb.AttributeValue{
		S: aws.String("ORDER"),
	}
	km[":id"] = &dynamodb.AttributeValue{
		S: aws.String(s.OrderNumber),
	}

	// Create the QueryInput
	qi := &dynamodb.QueryInput{
		TableName:                 aws.String(os.Getenv("TABLE")),
		KeyConditionExpression:    aws.String("PK = :type AND SK = :id"),
		ExpressionAttributeValues: km,
	}

	qo, err := dbs.Query(qi)
	if err != nil {
		return acmeserverless.Order{}, err
	}

	// Create an order struct from the data
	str := *qo.Items[0]["Payload"].S
	ord, err := acmeserverless.UnmarshalOrder(str)
	if err != nil {
		return acmeserverless.Order{}, err
	}

	ord.Status = &s.Status

	// Marshal the newly updated product struct
	payload, err := ord.Marshal()
	if err != nil {
		return ord, fmt.Errorf("error marshalling order: %s", err.Error())
	}

	em := make(map[string]*dynamodb.AttributeValue)
	em[":payload"] = &dynamodb.AttributeValue{
		S: aws.String(string(payload)),
	}

	uii := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(os.Getenv("TABLE")),
		Key:                       km,
		ExpressionAttributeValues: em,
		UpdateExpression:          aws.String("SET Payload = :payload"),
		ReturnValues:              aws.String("ALL_NEW"),
	}

	uio, err := dbs.UpdateItem(uii)
	if err != nil {
		return acmeserverless.Order{}, fmt.Errorf("error updating dynamodb: %s", err.Error())
	}

	return acmeserverless.UnmarshalOrder(*uio.Attributes["OrderString"].S)
}
