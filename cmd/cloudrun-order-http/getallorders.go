package main

import (
	"net/http"

	"github.com/valyala/fasthttp"
)

// GetAllOrders adds an item to the cart of a user
func GetAllOrders(ctx *fasthttp.RequestCtx) {
	orders, err := db.AllOrders()
	if err != nil {
		ErrorHandler(ctx, "GetAllOrders", "AllOrders", err)
		return
	}

	payload, err := orders.Marshal()
	if err != nil {
		ErrorHandler(ctx, "GetAllOrders", "Marshal", err)
		return
	}

	ctx.SetStatusCode(http.StatusOK)
	ctx.Write(payload)
}
