module github.com/retgits/acme-serverless-order

replace github.com/wavefronthq/wavefront-lambda-go => github.com/retgits/wavefront-lambda-go v0.0.0-20200402174306-c89629d5856a

go 1.13

require (
	github.com/aws/aws-lambda-go v1.15.0
	github.com/aws/aws-sdk-go v1.29.19
	github.com/getsentry/sentry-go v0.5.1
	github.com/gofrs/uuid v3.2.0+incompatible
	github.com/pulumi/pulumi v1.13.0
	github.com/pulumi/pulumi-aws v1.27.0
	github.com/retgits/acme-serverless-payment v0.0.0-20200312222942-f6670cccf0d1
	github.com/retgits/acme-serverless-shipment v0.0.0-20200312223419-2fc202224df3
	github.com/retgits/creditcard v0.6.0
	github.com/retgits/pulumi-helpers v0.1.7
	github.com/wavefronthq/wavefront-lambda-go v0.0.0-20190812171804-d9475d6695cc
)
