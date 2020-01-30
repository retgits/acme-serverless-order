package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	order "github.com/retgits/acme-serverless-order"
	"github.com/retgits/acme-serverless-order/internal/datastore/dynamodb"
)

func main() {
	os.Setenv("REGION", "us-west-2")
	os.Setenv("TABLE", "Order")

	data, err := ioutil.ReadFile("./data.json")
	if err != nil {
		log.Println(err)
	}

	var orders order.Orders

	err = json.Unmarshal(data, &orders)
	if err != nil {
		log.Println(err)
	}

	dynamoStore := dynamodb.New()

	for _, ord := range orders {
		ord, err = dynamoStore.AddOrder(ord)
		if err != nil {
			log.Println(err)
		}
	}
}
