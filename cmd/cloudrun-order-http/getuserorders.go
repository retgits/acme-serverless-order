package main

import (
	"net/http"

	"github.com/valyala/fasthttp"
)

// GetUserOrders adds an item to the cart of a user
func GetUserOrders(ctx *fasthttp.RequestCtx) {
	// Create the key attributes
	userID := ctx.UserValue("userid").(string)

	orders, err := db.UserOrders(userID)
	if err != nil {
		ErrorHandler(ctx, "GetUserOrders", "UserOrders", err)
		return
	}

	payload, err := orders.Marshal()
	if err != nil {
		ErrorHandler(ctx, "GetUserOrders", "Marshal", err)
		return
	}

	ctx.SetStatusCode(http.StatusOK)
	ctx.Write(payload)
}
