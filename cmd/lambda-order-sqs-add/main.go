package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/getsentry/sentry-go"
	"github.com/gofrs/uuid"
	acmeserverless "github.com/retgits/acme-serverless"
	"github.com/retgits/acme-serverless-order/internal/datastore/dynamodb"
	"github.com/retgits/acme-serverless-order/internal/emitter/sqs"
	wflambda "github.com/wavefronthq/wavefront-lambda-go"
)

// handler handles the API Gateway events and returns an error if anything goes wrong.
func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// Initiialize a connection to Sentry to capture errors and traces
	sentry.Init(sentry.ClientOptions{
		Dsn: os.Getenv("SENTRY_DSN"),
		Transport: &sentry.HTTPSyncTransport{
			Timeout: time.Second * 3,
		},
		ServerName:  os.Getenv("FUNCTION_NAME"),
		Release:     os.Getenv("VERSION"),
		Environment: os.Getenv("STAGE"),
	})

	// Create headers if they don't exist and add
	// the CORS required headers, otherwise the response
	// will not be accepted by browsers.
	headers := request.Headers
	if headers == nil {
		headers = make(map[string]string)
	}
	headers["Access-Control-Allow-Origin"] = "*"

	// Update the order with an OrderID
	ord, err := acmeserverless.UnmarshalOrder(request.Body)
	if err != nil {
		return handleError("unmarshal", headers, err)
	}
	ord.OrderID = uuid.Must(uuid.NewV4()).String()

	dynamoStore := dynamodb.New()
	ord, err = dynamoStore.AddOrder(ord)
	if err != nil {
		return handleError("store", headers, err)
	}

	prEvent := acmeserverless.PaymentRequestedEvent{
		Metadata: acmeserverless.Metadata{
			Domain: acmeserverless.OrderDomain,
			Source: "AddOrder",
			Type:   acmeserverless.PaymentRequestedEventName,
			Status: acmeserverless.DefaultSuccessStatus,
		},
		Data: acmeserverless.PaymentRequestDetails{
			OrderID: ord.OrderID,
			Card:    ord.Card,
			Total:   ord.Total,
		},
	}

	// Send a breadcrumb to Sentry with the payment request
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  acmeserverless.PaymentRequestedEventName,
		Timestamp: time.Now(),
		Level:     sentry.LevelInfo,
		Data:      acmeserverless.ToSentryMap(prEvent.Data),
	})

	em := sqs.New()
	err = em.SendPaymentRequestedEvent(prEvent)
	if err != nil {
		return handleError("request payment", headers, err)
	}

	status := acmeserverless.OrderStatus{
		OrderID: ord.OrderID,
		UserID:  ord.UserID,
		Payment: acmeserverless.CreditCardValidationDetails{
			Message: "pending payment",
			Success: false,
		},
	}

	// Send a breadcrumb to Sentry with the shipment request
	sentry.AddBreadcrumb(&sentry.Breadcrumb{
		Category:  acmeserverless.PaymentRequestedEventName,
		Timestamp: time.Now(),
		Level:     sentry.LevelInfo,
		Data:      acmeserverless.ToSentryMap(status.Payment),
	})

	payload, err := status.Marshal()
	if err != nil {
		return handleError("response", headers, err)
	}

	response := events.APIGatewayProxyResponse{
		StatusCode: http.StatusOK,
		Body:       string(payload),
		Headers:    headers,
	}

	return response, nil
}

// handleError takes the activity where the error occured and the error object and sends a message to sentry.
// The original error, together with the appropriate API Gateway Proxy Response, is returned so it can be thrown.
func handleError(area string, headers map[string]string, err error) (events.APIGatewayProxyResponse, error) {
	sentry.CaptureException(fmt.Errorf("error %s: %s", area, err.Error()))
	msg := fmt.Sprintf("error %s: %s", area, err.Error())
	log.Println(msg)
	return events.APIGatewayProxyResponse{
		StatusCode: http.StatusBadRequest,
		Body:       msg,
		Headers:    headers,
	}, nil
}

// The main method is executed by AWS Lambda and points to the handler
func main() {
	lambda.Start(wflambda.Wrapper(handler))
}
