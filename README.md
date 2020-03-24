# Order

> An order service, because what is a shop without actual orders to be shipped?

The Shipping service is part of the [ACME Fitness Serverless Shop](https://github.com/retgits/acme-serverless). The goal of this specific service is to interact with the catalog, front-end, and make calls to the order services.

## Prerequisites

* [Go (at least Go 1.12)](https://golang.org/dl/)
* [An AWS account](https://portal.aws.amazon.com/billing/signup)
* [A Pulumi account](https://app.pulumi.com/signup)
* [A Sentry.io account](https://sentry.io) if you want to enable tracing and error reporting

## Deploying

### With Pulumi (using SQS for eventing)

To deploy the Order Service you'll need a [Pulumi account](https://app.pulumi.com/signup). Once you have your Pulumi account and configured the [Pulumi CLI](https://www.pulumi.com/docs/get-started/aws/install-pulumi/), you can initialize a new stack using the Pulumi templates in the [pulumi](./pulumi) folder.

```bash
cd pulumi
pulumi stack init <your pulumi org>/acmeserverless-order/dev
```

Pulumi is configured using a file called `Pulumi.dev.yaml`. A sample configuration is available in the Pulumi directory. You can rename [`Pulumi.dev.yaml.sample`](./pulumi/Pulumi.dev.yaml.sample) to `Pulumi.dev.yaml` and update the variables accordingly. Alternatively, you can change variables directly in the [main.go](./pulumi/main.go) file in the pulumi directory. The configuration contains:

```yaml
config:
  aws:region: us-west-2 ## The region you want to deploy to
  awsconfig:generic:
    sentrydsn: ## The DSN to connect to Sentry
    accountid: ## Your AWS Account ID
  awsconfig:tags:
    author: retgits ## The author, you...
    feature: acmeserverless
    team: vcs ## The team you're on
    version: 0.2.0 ## The version
```

To create the Pulumi stack, and create the Order service, run `pulumi up`.

If you want to keep track of the resources in Pulumi, you can add tags to your stack as well.

```bash
pulumi stack tag set app:name acmeserverless
pulumi stack tag set app:feature acmeserverless-order
pulumi stack tag set app:domain order
```

### With CloudFormation (using EventBridge for eventing)

Clone this repository

```bash
git clone https://github.com/retgits/acme-serverless-order
cd acme-serverless-order
```

Get the Go Module dependencies

```bash
go get ./...
```

Change directories to the [cloudformation](./cloudformation) folder

```bash
cd ./cloudformation
```

If your event bus is not called _acmeserverless_, update the name of the `feature` parameter in the `template.yaml` file. Now you can build and deploy the Lambda function:

```bash
make build
make deploy
```

## Testing

To test, you can use the SQS or EventBridge test apps in the [acme-serverless](https://github.com/retgits/acme-serverless) repo.

## API

### `GET /order/all`

Get all orders in the system

```bash
curl --request GET \
  --url https://<id>.execute-api.us-west-2.amazonaws.com/Prod/order/all
```

```json
[
    {
        "userid": "8888",
        "firstname": "John",
        "lastname": "Blaze",
        "address": {
            "street": "20 Riding Lane Av",
            "city": "San Francisco",
            "zip": "10201",
            "state": "CA",
            "country": "USA"
        },
        "email": "jblaze@marvel.com",
        "delivery": "UPS/FEDEX",
        "card": {
            "type": "amex/visa/mastercard/bahubali",
            "number": "34983479798",
            "expMonth": "12",
            "expYear": "21",
            "ccv": "123"
        },
        "cart": [
            {
                "id": "1234",
                "description": "redpants",
                "quantity": "1",
                "price": "4"
            },
            {
                "id": "5678",
                "description": "bluepants",
                "quantity": "1",
                "price": "4"
            }
        ],
        "total": "100"
    }
]
```

### `GET /order/:userid`

Get all orders for a specific userid

```bash
curl --request GET \
  --url https://<id>.execute-api.us-west-2.amazonaws.com/Prod/order/8888
```

```json
[
    {
        "userid": "8888",
        "firstname": "John",
        "lastname": "Blaze",
        "address": {
            "street": "20 Riding Lane Av",
            "city": "San Francisco",
            "zip": "10201",
            "state": "CA",
            "country": "USA"
        },
        "email": "jblaze@marvel.com",
        "delivery": "UPS/FEDEX",
        "card": {
            "type": "amex/visa/mastercard/bahubali",
            "number": "34983479798",
            "expMonth": "12",
            "expYear": "21",
            "ccv": "123"
        },
        "cart": [
            {
                "id": "1234",
                "description": "redpants",
                "quantity": "1",
                "price": "4"
            },
            {
                "id": "5678",
                "description": "bluepants",
                "quantity": "1",
                "price": "4"
            }
        ],
        "total": "100"
    }
]
```

### `POST /order/add/:userid`

Add order for a specific user and run payment

```bash
curl --request GET \
  --url https://<id>.execute-api.us-west-2.amazonaws.com/Prod/order/add/8888 \
  --header 'content-type: application/json' \
  --data '{
    "userid": "8888",
    "firstname": "John",
    "lastname": "Blaze",
    "address": {
        "street": "20 Riding Lane Av",
        "city": "San Francisco",
        "zip": "10201",
        "state": "CA",
        "country": "USA"
    },
    "email": "jblaze@marvel.com",
    "delivery": "UPS/FEDEX",
    "card": {
        "type": "amex/visa/mastercard/bahubali",
        "number": "34983479798",
        "expMonth": "12",
        "expYear": "21",
        "ccv": "123"
    },
    "cart": [
        {
            "id": "1234",
            "description": "redpants",
            "quantity": "1",
            "price": "4"
        },
        {
            "id": "5678",
            "description": "bluepants",
            "quantity": "1",
            "price": "4"
        }
    ],
    "total": "100"
}'
```

The payload to add an order is

```json
{
    "userid": "8888",
    "firstname": "John",
    "lastname": "Blaze",
    "address": {
        "street": "20 Riding Lane Av",
        "city": "San Francisco",
        "zip": "10201",
        "state": "CA",
        "country": "USA"
    },
    "email": "jblaze@marvel.com",
    "delivery": "UPS/FEDEX",
    "card": {
        "type": "amex/visa/mastercard/bahubali",
        "number": "34983479798",
        "expMonth": "12",
        "expYear": "21",
        "ccv": "123"
    },
    "cart": [
        {
            "id": "1234",
            "description": "redpants",
            "quantity": "1",
            "price": "4"
        },
        {
            "id": "5678",
            "description": "bluepants",
            "quantity": "1",
            "price": "4"
        }
    ],
    "total": "100"
}
```

When the payment is successful an HTTP/200 message is returned

```json
{
    "order_id": "ea5ed52e-3c4e-11e9-9aff-e62b216188c4",
    "payment": {
        "amount": "",
        "message": "pending payment",
        "success": "false",
        "transactionID": ""
    },
    "userid": "7892"
}
```

## Events

The events for all of ACME Serverless Fitness Shop are structured as

```json
{
    "metadata": { // Metadata for all services
        "domain": "Order", // Domain represents the the event came from (like Payment or Order)
        "source": "CLI", // Source represents the function the event came from (like ValidateCreditCard or SubmitOrder)
        "type": "ShipmentRequested", // Type respresents the type of event this is (like CreditCardValidated)
        "status": "success" // Status represents the current status of the event (like Success)
    },
    "data": {} // The actual payload of the event
}
```

### Add an order

These are events that are emitted by the `lambda-<eventing option>-add` functions:

```json
{
    "metadata": {
        "domain": "Order",
        "source": "AddOrder",
        "type": "PaymentRequested",
        "status": "success"
    },
    "data": {
        "orderID": "12345",
        "card": {
            "Type": "Visa",
            "Number": "4222222222222",
            "ExpiryYear": 2022,
            "ExpiryMonth": 12,
            "CVV": "123"
        },
        "total": "123"
    }
}
```

### Ship an order

These are events that trigger the `lambda-<eventing option>-ship` functions:

```json
{
    "metadata": {
        "domain": "Payment",
        "source": "ValidateCreditCard",
        "type": "CreditCardValidated",
        "status": "success"
    },
    "data": {
        "success": "true",
        "status": 200,
        "message": "transaction successful",
        "amount": 123,
        "transactionID": "3f846704-af12-4ea9-a98c-8d7b37e10b54"
    }
}
```

After updating the Order storage, the following event is sent:

```json
{
    "metadata": {
        "domain": "Order",
        "source": "ShipOrder",
        "type": "ShipmentRequested",
        "status": "success"
    },
    "data": {
        "_id": "12345",
        "delivery": "UPS/FEDEX"
    }
}
```

### Update order status

These are events that trigger the `lambda-<eventing option>-update` functions:

After the shipment has been sent:

```json
{
    "metadata": {
        "domain": "Shipment",
        "source": "SendShipment",
        "type": "SentShipment",
        "status": "success"
    },
    "data": {
        "trackingNumber": "6bc3b96b-19e9-42d3-bff8-91a2c431d1d6",
        "orderNumber": "12345",
        "status": "shipped - pending delivery"
    }
}
```

After delivery:

```json
{
    "metadata": {
        "domain": "Shipment",
        "source": "SendShipment",
        "type": "DeliveredShipment",
        "status": "success"
    },
    "data": {
        "trackingNumber": "6bc3b96b-19e9-42d3-bff8-91a2c431d1d6",
        "orderNumber": "12345",
        "status": "delivered"
    }
}
```

## Troubleshooting

In case the API Gateway responds with `{"message":"Forbidden"}`, there is likely an issue with the deployment of the API Gateway. To solve this problem, you can use the AWS CLI. To confirm this, run `aws apigateway get-deployments --rest-api-id <rest-api-id>`. If that returns no deployments, you can create a deployment for the *prod* stage with `aws apigateway create-deployment --rest-api-id <rest-api-id> --stage-name prod --stage-description 'Prod Stage' --description 'deployment to the prod stage'`.

## Contributing

[Pull requests](https://github.com/retgits/acme-serverless-order/pulls) are welcome. For major changes, please open [an issue](https://github.com/retgits/acme-serverless-order/issues) first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License

See the [LICENSE](./LICENSE) file in the repository