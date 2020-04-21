package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gofrs/uuid"
	acmeserverless "github.com/retgits/acme-serverless"
	"github.com/valyala/fasthttp"
)

// AddOrder adds an item to the cart of a user
func AddOrder(ctx *fasthttp.RequestCtx) {
	ord, err := acmeserverless.UnmarshalOrder(string(ctx.Request.Body()))
	if err != nil {
		ErrorHandler(ctx, "AddOrder", "UnmarshalOrder", err)
		return
	}
	ord.OrderID = uuid.Must(uuid.NewV4()).String()

	ord, err = db.AddOrder(ord)
	if err != nil {
		ErrorHandler(ctx, "AddOrder", "AddOrder", err)
		return
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

	// Create payment payload
	payload, err := prEvent.Marshal()
	if err != nil {
		ErrorHandler(ctx, "AddOrder", "Marshal", err)
		return
	}

	// Send to Payment
	req, err := http.NewRequest("POST", os.Getenv("PAYMENT_URL"), bytes.NewReader(payload))
	if err != nil {
		ErrorHandler(ctx, "AddOrder", "NewRequest", err)
		return
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add("host", os.Getenv("PAYMENT_HOST"))

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		ErrorHandler(ctx, "AddOrder", "DefaultClient.Do", err)
		return
	}

	defer res.Body.Close()
	body, _ := ioutil.ReadAll(res.Body)

	if res.StatusCode != 200 {
		ErrorHandler(ctx, "AddOrder", "Payment", fmt.Errorf(string(body)))
		return
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

	payload, err = status.Marshal()
	if err != nil {
		ErrorHandler(ctx, "AddOrder", "Marshal", err)
		return
	}

	req, err = http.NewRequest("POST", os.Getenv("SHIPMENT_URL"), bytes.NewReader(payload))
	if err != nil {
		ErrorHandler(ctx, "AddOrder", "NewRequest", err)
		return
	}

	req.Header.Add("content-type", "application/json")
	req.Header.Add("host", os.Getenv("SHIPMENT_HOST"))

	_, err = http.DefaultClient.Do(req)
	if err != nil {
		ErrorHandler(ctx, "AddOrder", "DefaultClient.Do", err)
		return
	}

	ctx.SetStatusCode(http.StatusOK)
	ctx.Write(payload)
}
