# Order

> An order service, because what is a shop without actual orders to be shipped?

The Shipping service is part of the [ACME Fitness Serverless Shop](https://github.com/retgits/acme-serverless). The goal of this specific service is, as the name implies, to ship products using a wide variety of shipping suppliers.

## Prerequisites

* [Go (at least Go 1.12)](https://golang.org/dl/)
* [An AWS Account](https://portal.aws.amazon.com/billing/signup)
* The _vuln_ targets for Make and Mage rely on the [Snyk](http://snyk.io/) CLI

## Eventing Options

The order service has a few different eventing platforms available:

* [Amazon EventBridge](https://aws.amazon.com/eventbridge/)
* [Amazon Simple Queue Service](https://aws.amazon.com/sqs/)
* [Amazon API Gateway](https://aws.amazon.com/api-gateway/)

For all options there is a Lambda function ready to be deployed where events arrive from and are sent to that particular eventing mechanism. You can find the code in `./cmd/lambda-order-<event platform>`. There is a ready made test function available as well that sends a message to the eventing platform. The code for the tester can be found in `./cmd/test-<event platform>`. The messages the testing app sends, are located under the [`test`](./test) folder.

There are four Lambda functions of the order service are triggered by [Amazon API Gateway](https://aws.amazon.com/api-gateway/):

* [lambda-order-all](./cmd/lambda-order-all)
* [lambda-order-users](./cmd/lambda-order-users)
* [lambda-order-sqs-add](./cmd/lambda-order-sqs-add)
* [lambda-order-eventbridge-add](./cmd/lambda-order-eventbridge-add)

## Data Stores

The order service supports the following data stores:

* [Amazon DynamoDB](https://aws.amazon.com/dynamodb/): The scripts to both create and seed the DynamoDB can be found in the [acme-serverless](https://github.com/retgits/acme-serverless) repo.

## Using Amazon EventBridge

### Prerequisites for EventBridge

* [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html) installed and configured
* [Custom EventBus](https://docs.aws.amazon.com/eventbridge/latest/userguide/create-event-bus.html) configured, the name of the configured event bus should be set as the `feature` parameter in the `template.yaml` file.

### Build and deploy for EventBridge

Clone this repository

```bash
git clone https://github.com/retgits/acme-serverless-order
cd acme-serverless-order
```

Get the Go Module dependencies

```bash
go get ./...
```

Change directories to the [deploy/cloudformation](./deploy/cloudformation) folder

```bash
cd ./deploy/cloudformation
```

If your event bus is not called _acmeserverless_, update the name of the `feature` parameter in the `template.yaml` file. Now you can build and deploy the Lambda function:

```bash
make build TYPE=eventbridge
make deploy TYPE=eventbridge
```

### Testing EventBridge

You can test the function from the [AWS Lambda Console](https://console.aws.amazon.com/lambda/home) using the test data from the files in [eventbridge](./test/eventbridge/). To send a message to the event bus, you can use the app in `./cmd/test-eventbridge` and run

```bash
go run main.go -event=<any of the files existing in test/eventbridge> -location=<location on disk of the test/eventbridge folder> -bus=<name of the custom bus>
```

### Prerequisites for SQS

* [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/cli-chap-install.html) installed and configured

### Build and deploy for SQS

Clone this repository

```bash
git clone https://github.com/retgits/acme-serverless-order
cd acme-serverless-order
```

Get the Go Module dependencies

```bash
go get ./...
```

Change directories to the [deploy/cloudformation](./deploy/cloudformation) folder

```bash
cd ./deploy/cloudformation
```

Now you can build and deploy the Lambda function:

```bash
make build TYPE=sqs
make deploy TYPE=sqs
```

### Testing SQS

To send a message to an SQS queue using the test data from the files in [sqs](./test/sqs/), you can use the app in `./cmd/test-sqs` and run

```bash
go run main.go -event=<any of the files existing in test/sqs> -location=<location on disk of the test/sqs folder> -queue=<name of the sqs queue>
```

If you want to test from the [AWS Lambda Console](https://console.aws.amazon.com/lambda/home), you'll have to wrap the test data in a SQS record envelop:

```json
{
  "Records": [
    {
      "messageId": "19dd0b57-b21e-4ac1-bd88-01bbb068cb78",
      "receiptHandle": "MessageReceiptHandle",
      "body": "", // This is where the data, an escaped JSON string, should be pasted
      "attributes": {
        "ApproximateReceiveCount": "1",
        "SentTimestamp": "1523232000000",
        "SenderId": "123456789012",
        "ApproximateFirstReceiveTimestamp": "1523232000001"
      },
      "messageAttributes": {},
      "md5OfBody": "7b270e59b47ff90a553787216d55d91d",
      "eventSource": "aws:sqs",
      "eventSourceARN": "arn:aws:sqs:us-east-1:123456789012:MyQueue",
      "awsRegion": "us-east-1"
    }
  ]
}
```

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

## Using Make

The Makefiles in the [Cloudformation](./deploy/cloudformation) directory have a few a bunch of options available:

| Target  | Description                                                |
|---------|------------------------------------------------------------|
| build   | Build the executable for Lambda                            |
| clean   | Remove all generated files                                 |
| deploy  | Deploy the app to AWS                                      |
| destroy | Deletes the CloudFormation stack and all created resources |
| help    | Displays the help for each target (this message)           |
| vuln    | Scans the Go.mod file for known vulnerabilities using Snyk |

The targets `build` and `deploy` need a variable **`TYPE`** set to either `eventbridge` or `sqs` to build and deploy the correct Lambda functions.

## Using Mage

If you want to "go all Go" (_pun intended_) and write plain-old go functions to build and deploy, you can use [Mage](https://magefile.org/). Mage is a make/rake-like build tool using Go so Mage automatically uses the functions you create as Makefile-like runnable targets.

### Prerequisites for Mage

To use Mage, you'll need to install it first:

```bash
go get -u -d github.com/magefile/mage
cd $GOPATH/src/github.com/magefile/mage
go run bootstrap.go
```

Instructions curtesy of Mage

### Targets

The Magefile in this repository has a bunch of targets available:

| Target | Description                                                                                              |
|--------|----------------------------------------------------------------------------------------------------------|
| build  | compiles the individual commands in the cmd folder, along with their dependencies.                       |
| clean  | removes object files from package source directories.                                                    |
| deploy | packages, deploys, and returns all outputs of your stack.                                                |
| deps   | resolves and downloads dependencies to the current development module and then builds and installs them. |
| test   | 'Go test' automates testing the packages named by the import paths.                                      |
| vuln   | uses Snyk to test for any known vulnerabilities in go.mod.                                               |

The targets `build` and `deploy` need a variable **`TYPE`** set to either `eventbridge` or `sqs` to build and deploy the correct Lambda functions.

## Contributing

[Pull requests](https://github.com/retgits/acme-serverless-order/pulls) are welcome. For major changes, please open [an issue](https://github.com/retgits/acme-serverless-order/issues) first to discuss what you would like to change.

Please make sure to update tests as appropriate.

## License

See the [LICENSE](./LICENSE) file in the repository