package dynamodb

import (
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/gofrs/uuid"
	order "github.com/retgits/acme-serverless-order"
	"github.com/retgits/acme-serverless-order/internal/datastore"
)

type manager struct{}

func New() datastore.Manager {
	return manager{}
}

func (m manager) AddOrder(o order.Order) (order.Order, error) {
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	}))

	o.OrderID = uuid.Must(uuid.NewV4()).String()

	payload, err := o.Marshal()
	if err != nil {
		return o, fmt.Errorf("error marshalling order: %s", err.Error())
	}

	dbs := dynamodb.New(awsSession)

	// Create a map of DynamoDB Attribute Values containing the table keys
	km := make(map[string]*dynamodb.AttributeValue)
	km["ID"] = &dynamodb.AttributeValue{
		S: aws.String(o.OrderID),
	}

	em := make(map[string]*dynamodb.AttributeValue)
	em[":content"] = &dynamodb.AttributeValue{
		S: aws.String(payload),
	}
	em[":status"] = &dynamodb.AttributeValue{
		S: aws.String("pending payment"),
	}
	em[":user"] = &dynamodb.AttributeValue{
		S: aws.String(o.UserID),
	}

	uii := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(os.Getenv("TABLE")),
		Key:                       km,
		ExpressionAttributeValues: em,
		UpdateExpression:          aws.String("SET OrderStatus = :status, OrderString = :content, UserID = :user"),
	}

	_, err = dbs.UpdateItem(uii)
	if err != nil {
		return o, fmt.Errorf("error updating dynamodb: %s", err.Error())
	}

	return o, nil
}

func (m manager) AllOrders() (order.Orders, error) {
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	}))

	dbs := dynamodb.New(awsSession)

	si := &dynamodb.ScanInput{
		TableName: aws.String(os.Getenv("TABLE")),
	}

	so, err := dbs.Scan(si)
	if err != nil {
		return nil, fmt.Errorf("error querrying dynamodb: %s", err.Error())
	}

	orders := make(order.Orders, len(so.Items))

	for idx, ord := range so.Items {
		str := ord["OrderString"].S
		o, err := order.UnmarshalOrder(*str)
		if err != nil {
			fmt.Println(err.Error())
		}
		o.Status = ord["OrderStatus"].S
		orders[idx] = o
	}

	return orders, nil
}

func (m manager) UserOrders(userID string) (order.Orders, error) {
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	}))

	dbs := dynamodb.New(awsSession)

	// Create a map of DynamoDB Attribute Values containing the table keys
	km := make(map[string]*dynamodb.AttributeValue)
	km[":userid"] = &dynamodb.AttributeValue{
		S: aws.String(userID),
	}

	si := &dynamodb.ScanInput{
		TableName:                 aws.String(os.Getenv("TABLE")),
		ExpressionAttributeValues: km,
		FilterExpression:          aws.String("UserID = :userid"),
	}

	so, err := dbs.Scan(si)
	if err != nil {
		return nil, fmt.Errorf("error querrying dynamodb: %s", err.Error())
	}

	orders := make(order.Orders, len(so.Items))

	for idx, ord := range so.Items {
		str := ord["OrderString"].S
		o, err := order.UnmarshalOrder(*str)
		if err != nil {
			fmt.Println(err.Error())
		}
		o.Status = ord["OrderStatus"].S
		orders[idx] = o
	}

	return orders, nil
}

func (m manager) UpdateStatus(s order.ShipmentStatus) (order.Order, error) {
	awsSession := session.Must(session.NewSession(&aws.Config{
		Region: aws.String(os.Getenv("REGION")),
	}))

	dbs := dynamodb.New(awsSession)

	// Create a map of DynamoDB Attribute Values containing the table keys
	km := make(map[string]*dynamodb.AttributeValue)
	km["ID"] = &dynamodb.AttributeValue{
		S: aws.String(s.OrderNumber),
	}

	em := make(map[string]*dynamodb.AttributeValue)
	em[":status"] = &dynamodb.AttributeValue{
		S: aws.String(s.Status),
	}

	uii := &dynamodb.UpdateItemInput{
		TableName:                 aws.String(os.Getenv("TABLE")),
		Key:                       km,
		ExpressionAttributeValues: em,
		UpdateExpression:          aws.String("SET OrderStatus = :status"),
		ReturnValues:              aws.String("ALL_NEW"),
	}

	uio, err := dbs.UpdateItem(uii)
	if err != nil {
		return order.Order{}, fmt.Errorf("error updating dynamodb: %s", err.Error())
	}

	return order.UnmarshalOrder(*uio.Attributes["OrderString"].S)
}
