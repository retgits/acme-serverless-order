// Package datastore contains the interfaces that the Order service
// in the ACME Serverless Fitness Shop needs to store and retrieve data.
// In order to add a new service, the Manager interface
// needs to be implemented.
package datastore

import (
	order "github.com/retgits/acme-serverless-order"
	shipment "github.com/retgits/acme-serverless-shipment"
)

// Manager is the interface that describes the methods the
// data store needs to implement to be able to work with
// the ACME Serverless Fitness Shop.
type Manager interface {
	AddOrder(o order.Order) (order.Order, error)
	AllOrders() (order.Orders, error)
	UserOrders(userID string) (order.Orders, error)
	UpdateStatus(s shipment.ShipmentData) (order.Order, error)
}
