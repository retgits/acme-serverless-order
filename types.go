package order

import (
	"encoding/json"

	"github.com/retgits/creditcard"
)

type PaymentRequest struct {
	OrderID string          `json:"orderID"`
	Card    creditcard.Card `json:"card"`
	Total   string          `json:"total"`
}

func (r *PaymentRequest) Marshal() (string, error) {
	b, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

type Orders []Order

func (r *Orders) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

type Order struct {
	OrderID   string   `json:"_id"`
	Status    *string  `json:"status,omitempty"`
	UserID    string   `json:"userid,omitempty"`
	Firstname *string  `json:"firstname,omitempty"`
	Lastname  *string  `json:"lastname,omitempty"`
	Address   *Address `json:"address,omitempty"`
	Email     *string  `json:"email,omitempty"`
	Delivery  string   `json:"delivery"`
	Card      Card     `json:"card,omitempty"`
	Cart      []Cart   `json:"cart"`
	Total     string   `json:"total,omitempty"`
}

type Address struct {
	Street  *string `json:"street,omitempty"`
	City    *string `json:"city,omitempty"`
	Zip     *string `json:"zip,omitempty"`
	State   *string `json:"state,omitempty"`
	Country *string `json:"country,omitempty"`
}

type Card struct {
	Type     string `json:"type,omitempty"`
	Number   string `json:"number,omitempty"`
	ExpMonth string `json:"expMonth,omitempty"`
	ExpYear  string `json:"expYear,omitempty"`
	Ccv      string `json:"ccv,omitempty"`
}

type Cart struct {
	ID          *string `json:"id,omitempty"`
	Description *string `json:"description,omitempty"`
	Quantity    *string `json:"quantity,omitempty"`
	Price       *string `json:"price,omitempty"`
}

func (r *Order) Marshal() (string, error) {
	s, err := json.Marshal(r)
	if err != nil {
		return "", err
	}
	return string(s), nil
}

func UnmarshalOrder(data string) (Order, error) {
	var r Order
	err := json.Unmarshal([]byte(data), &r)
	return r, err
}

type OrderStatus struct {
	OrderID string        `json:"order_id"`
	Payment PaymentStatus `json:"payment"`
	Userid  string        `json:"userid"`
}

type PaymentStatus struct {
	Amount        string `json:"amount"`
	Message       string `json:"message"`
	Success       string `json:"success"`
	TransactionID string `json:"transactionID"`
}

func (r *OrderStatus) Marshal() ([]byte, error) {
	return json.Marshal(r)
}

// Response is the output message that the Lambda function receives from payment. It will be a JSON string payload.
type PaymentResponse struct {
	Success       bool   `json:"success"`
	Status        int    `json:"status"`
	Message       string `json:"message"`
	Amount        string `json:"amount,omitempty"`
	TransactionID string `json:"transactionID"`
	OrderID       string `json:"orderID"`
}

func UnmarshalPaymentResponse(data []byte) (PaymentResponse, error) {
	var r PaymentResponse
	err := json.Unmarshal(data, &r)
	return r, err
}

type Shipment struct {
	TrackingNumber string `json:"trackingNumber"`
	OrderNumber    string `json:"orderNumber"`
	Status         string `json:"status"`
}

// UnmarshalShipment takes a byte array and turns that into a Shipment
func UnmarshalShipment(data []byte) (Shipment, error) {
	var r Shipment
	err := json.Unmarshal(data, &r)
	return r, err
}
