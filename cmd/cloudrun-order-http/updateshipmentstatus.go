package main

import (
	"fmt"
	"net/http"

	"github.com/getsentry/sentry-go"
	acmeserverless "github.com/retgits/acme-serverless"
	"github.com/valyala/fasthttp"
)

// UpdateShipmentStatus adds an item to the cart of a user
func UpdateShipmentStatus(ctx *fasthttp.RequestCtx) {
	req, err := acmeserverless.UnmarshalShipmentSent(ctx.Request.Body())
	if err != nil {
		ErrorHandler(ctx, "UpdateOrderStatus", "UnmarshalShipmentSent", err)
		return
	}

	_, err = db.UpdateStatus(req.Data)
	if err != nil {
		ErrorHandler(ctx, "UpdateOrderStatus", "UpdateStatus", err)
		return
	}

	msg := fmt.Sprintf("shipment status successfully updated for order [%s]", req.Data.OrderNumber)

	sentry.CaptureMessage(msg)

	ctx.SetStatusCode(http.StatusOK)
	ctx.Write([]byte(msg))
}
