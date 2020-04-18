// Package mongodb leverages cross-platform document-oriented database program. Classified as a
// NoSQL database program, MongoDB uses JSON-like documents with schema.
package mongodb

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/gofrs/uuid"
	acmeserverless "github.com/retgits/acme-serverless"
	"github.com/retgits/acme-serverless-order/internal/datastore"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// The pointer to MongoDB provides the API operation methods for making requests to MongoDB.
// This specifically creates a single instance of the MongoDB service which can be reused if the
// container stays warm.
var dbs *mongo.Collection

// manager is an empty struct that implements the methods of the
// Manager interface.
type manager struct{}

// init creates the connection to MongoDB.
func init() {
	username := os.Getenv("MONGO_USERNAME")
	password := os.Getenv("MONGO_PASSWORD")
	hostname := os.Getenv("MONGO_HOSTNAME")
	port := os.Getenv("MONGO_PORT")

	connString := fmt.Sprintf("mongodb://%s:%s@%s:%s", username, password, hostname, port)
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(connString))
	if err != nil {
		log.Fatalf("error connecting to MongoDB: %s", err.Error())
	}
	dbs = client.Database("acmeserverless").Collection("order")
}

// New creates a new datastore manager using Amazon DynamoDB as backend
func New() datastore.Manager {
	return manager{}
}

func ptrString(p string) *string {
	return &p
}

// AddOrder stores a new order in Amazon DynamoDB
func (m manager) AddOrder(o acmeserverless.Order) (acmeserverless.Order, error) {
	// Generate and assign a new orderID
	o.OrderID = uuid.Must(uuid.NewV4()).String()
	o.Status = ptrString("Pending Payment")

	// Marshal the newly updated product struct
	payload, err := o.Marshal()
	if err != nil {
		return o, fmt.Errorf("error marshalling order: %s", err.Error())
	}

	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	_, err = dbs.InsertOne(ctx, bson.D{{"SK", o.OrderID}, {"KeyID", o.UserID}, {"PK", "ORDER"}, {"Payload", string(payload)}})

	return o, nil
}

// AllOrders retrieves all orders from DynamoDB
func (m manager) AllOrders() (acmeserverless.Orders, error) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	cursor, err := dbs.Find(ctx, bson.D{})
	if err != nil {
		log.Fatal(err)
	}

	var results []bson.M

	if err = cursor.All(ctx, &results); err != nil {
		log.Fatal(err)
	}

	orders := make(acmeserverless.Orders, len(results))

	for idx, ord := range results {
		o, err := acmeserverless.UnmarshalOrder(ord["Payload"].(string))
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
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)
	cursor, err := dbs.Find(ctx, bson.D{{"KeyID", userID}})
	if err != nil {
		log.Fatal(err)
	}

	var results []bson.M

	if err = cursor.All(ctx, &results); err != nil {
		log.Fatal(err)
	}

	orders := make(acmeserverless.Orders, len(results))

	for idx, ord := range results {
		o, err := acmeserverless.UnmarshalOrder(ord["Payload"].(string))
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
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	res := dbs.FindOne(ctx, bson.D{{"SK", s.OrderNumber}})

	raw, err := res.DecodeBytes()
	if err != nil {
		return acmeserverless.Order{}, fmt.Errorf("unable to decode bytes: %s", err.Error())
	}

	payload := raw.Lookup("Payload").StringValue()

	// Return an error if no order was found
	if len(payload) < 5 {
		return acmeserverless.Order{}, fmt.Errorf("no order found with id %s", s.OrderNumber)
	}

	// Create an order struct from the data
	ord, err := acmeserverless.UnmarshalOrder(payload)
	if err != nil {
		return acmeserverless.Order{}, err
	}

	ord.Status = &s.Status

	// Marshal the newly updated product struct
	newOrder, err := ord.Marshal()
	if err != nil {
		return ord, fmt.Errorf("error marshalling order: %s", err.Error())
	}

	_, err = dbs.UpdateOne(ctx, bson.D{{"SK", ord.OrderID}}, bson.D{{"$set", bson.D{{"Payload", string(newOrder)}}}})

	return ord, err
}
