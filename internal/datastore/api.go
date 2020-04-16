// Package datastore contains the interfaces that the Order service
// in the ACME Serverless Fitness Shop needs to store and retrieve data.
// In order to add a new service, the Manager interface
// needs to be implemented.
package datastore

import (
	acmeserverless "github.com/retgits/acme-serverless"
)

// Manager is the interface that describes the methods the
// data store needs to implement to be able to work with
// the ACME Serverless Fitness Shop.
type Manager interface {
	AddOrder(o acmeserverless.Order) (acmeserverless.Order, error)
	AllOrders() (acmeserverless.Orders, error)
	UserOrders(userID string) (acmeserverless.Orders, error)
	UpdateStatus(s acmeserverless.ShipmentData) (acmeserverless.Order, error)
}
