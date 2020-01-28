package datastore

import "github.com/retgits/acme-serverless-order"

// Manager ...
type Manager interface {
	AddOrder(o order.Order) (order.Order, error)
	AllOrders() (order.Orders, error)
	UserOrders(userID string) (order.Orders, error)
	UpdateStatus(s order.ShipmentStatus) (order.Order, error)
}
