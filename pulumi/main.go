package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/pulumi/pulumi-aws/sdk/go/aws/apigateway"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/lambda"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
	"github.com/pulumi/pulumi/sdk/go/pulumi/config"
)

const (
	// The shell to use
	shell = "sh"

	// The flag for the shell to read commands from a string
	shellFlag = "-c"
)

// Tags are key-value pairs to apply to the resources created by this stack
type Tags struct {
	// Author is the person who created the code, or performed the deployment
	Author pulumi.String

	// Feature is the project that this resource belongs to
	Feature pulumi.String

	// Team is the team that is responsible to manage this resource
	Team pulumi.String

	// Version is the version of the code for this resource
	Version pulumi.String
}

// LambdaConfig contains the key-value pairs for the configuration of AWS Lambda in this stack
type LambdaConfig struct {
	// The DSN used to connect to Sentry
	SentryDSN string `json:"sentrydsn"`

	// The ARN for the DynamoDB table
	DynamoARN string `json:"dynamoarn"`

	// The AWS region used
	Region string `json:"region"`

	// The AWS AccountID used
	AccountID string `json:"accountid"`

	// The SQS queue to send responses to
	PaymentResponseQueue string `json:"paymentresponsequeue"`

	// The SQS queue to receives messages from
	PaymentRequestQueue string `json:"paymentrequestqueue"`

	// The SQS queue to send responses to
	ShipmentResponseQueue string `json:"shipmentresponsequeue"`

	// The SQS queue to receives messages from
	ShipmentRequestQueue string `json:"shipmentrequestqueue"`
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Read the configuration data from Pulumi.<stack>.yaml
		conf := config.New(ctx, "awsconfig")

		// Create a new Tags object with the data from the configuration
		var tags Tags
		conf.RequireObject("tags", &tags)

		// Create a new DynamoConfig object with the data from the configuration
		var lambdaConfig LambdaConfig
		conf.RequireObject("lambda", &lambdaConfig)

		// Create a map[string]pulumi.Input of the tags
		// the first four tags come from the configuration file
		// the last two are derived from this deployment
		tagMap := make(map[string]pulumi.Input)
		tagMap["Author"] = tags.Author
		tagMap["Feature"] = tags.Feature
		tagMap["Team"] = tags.Team
		tagMap["Version"] = tags.Version
		tagMap["ManagedBy"] = pulumi.String("Pulumi")
		tagMap["Stage"] = pulumi.String(ctx.Stack())

		// functions are the functions that need to be deployed
		functions := []string{
			"lambda-order-all",
			"lambda-order-users",
			"lambda-order-sqs-add",
			"lambda-order-sqs-ship",
			"lambda-order-sqs-update",
		}

		// Compile and zip the AWS Lambda functions
		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		for _, fnName := range functions {
			// Find the working folder
			fnFolder := path.Join(wd, "..", "cmd", fnName)

			// Run go build
			if err := run(fnFolder, "GOOS=linux GOARCH=amd64 go build"); err != nil {
				fmt.Printf("Error building code: %s", err.Error())
				os.Exit(1)
			}

			// Zip up the binary
			if err := run(fnFolder, fmt.Sprintf("zip ./%s.zip ./%s", fnName, fnName)); err != nil {
				fmt.Printf("Error creating zipfile: %s", err.Error())
				os.Exit(1)
			}
		}

		// Create an API Gateway
		gateway, err := apigateway.NewRestApi(ctx, "OrderService", &apigateway.RestApiArgs{
			Name:        pulumi.String("OrderService"),
			Description: pulumi.String("ACME Serverless Fitness Shop - Order"),
			Tags:        pulumi.Map(tagMap),
			Policy:      pulumi.String(`{ "Version": "2012-10-17", "Statement": [ { "Action": "sts:AssumeRole", "Principal": { "Service": "lambda.amazonaws.com" }, "Effect": "Allow", "Sid": "" },{ "Action": "execute-api:Invoke", "Resource":"execute-api:/*", "Principal": "*", "Effect": "Allow", "Sid": "" } ] }`),
		})
		if err != nil {
			return err
		}

		// Create the parent resources in the API Gateway
		orderResource, err := apigateway.NewResource(ctx, "OrderAPIResource", &apigateway.ResourceArgs{
			RestApi:  gateway.ID(),
			PathPart: pulumi.String("order"),
			ParentId: gateway.RootResourceId,
		})
		if err != nil {
			return err
		}

		// Create the parent resources in the API Gateway
		orderAddResource, err := apigateway.NewResource(ctx, "OrderAddAPIResource", &apigateway.ResourceArgs{
			RestApi:  gateway.ID(),
			PathPart: pulumi.String("add"),
			ParentId: orderResource.ID(),
		})
		if err != nil {
			return err
		}

		// dynamoCRUDPolicyString is a policy template, derived from AWS SAM, to allow apps
		// to connect to and execute command on Amazon DynamoDB
		dynamoCRUDPolicyString := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Action": [
						"dynamodb:GetItem",
						"dynamodb:DeleteItem",
						"dynamodb:PutItem",
						"dynamodb:Scan",
						"dynamodb:Query",
						"dynamodb:UpdateItem",
						"dynamodb:BatchWriteItem",
						"dynamodb:BatchGetItem",
						"dynamodb:DescribeTable",
						"dynamodb:ConditionCheckItem"
					],
					"Effect": "Allow",
					"Resource": "%s"
				}
			]
		}`, lambdaConfig.DynamoARN)

		// Add OrderAll function
		roleArgs := &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
				"Version": "2012-10-17",
				"Statement": [
				{
					"Action": "sts:AssumeRole",
					"Principal": {
						"Service": "lambda.amazonaws.com"
					},
					"Effect": "Allow",
					"Sid": ""
				}
				]
			}`),
			Description: pulumi.String("Role for the Order Service (lambda-order-all) of the ACME Serverless Fitness Shop"),
			Tags:        pulumi.Map(tagMap),
		}

		role, err := iam.NewRole(ctx, "ACMEServerlessOrderRole-lambda-order-all", roleArgs)
		if err != nil {
			return err
		}

		// Attach the AWSLambdaBasicExecutionRole so the function can create Log groups in CloudWatch
		_, err = iam.NewRolePolicyAttachment(ctx, "AWSLambdaBasicExecutionRole-lambda-order-all", &iam.RolePolicyAttachmentArgs{
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
			Role:      role.Name,
		})
		if err != nil {
			return err
		}

		// Add the DynamoDB policy
		_, err = iam.NewRolePolicy(ctx, "ACMEServerlessOrderPolicy-lambda-order-all", &iam.RolePolicyArgs{
			Name:   pulumi.String("ACMEServerlessOrderPolicy-lambda-order-all"),
			Role:   role.Name,
			Policy: pulumi.String(dynamoCRUDPolicyString),
		})
		if err != nil {
			return err
		}

		// All functions will have the same environment variables, with the exception
		// of the function name
		variables := make(map[string]pulumi.StringInput)
		variables["REGION"] = pulumi.String(lambdaConfig.Region)
		variables["SENTRY_DSN"] = pulumi.String(lambdaConfig.SentryDSN)
		variables["VERSION"] = tags.Version
		variables["STAGE"] = pulumi.String(ctx.Stack())
		parts := strings.Split(lambdaConfig.DynamoARN, "/")
		variables["TABLE"] = pulumi.String(parts[1])

		environment := lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap(variables),
		}

		// Create the All function
		functionArgs := &lambda.FunctionArgs{
			Description: pulumi.String("A Lambda function to get all orders"),
			Runtime:     pulumi.String("go1.x"),
			Name:        pulumi.String(fmt.Sprintf("%s-lambda-order-all", ctx.Stack())),
			MemorySize:  pulumi.Int(256),
			Timeout:     pulumi.Int(10),
			Handler:     pulumi.String("lambda-order-all"),
			Environment: environment,
			Code:        pulumi.NewFileArchive("../cmd/lambda-order-all/lambda-order-all.zip"),
			Role:        role.Arn,
			Tags:        pulumi.Map(tagMap),
		}
		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-order-all", ctx.Stack()))

		function, err := lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-order-all", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		// Create the parent resources in the API Gateway
		resource, err := apigateway.NewResource(ctx, "OrderAllAPIResource", &apigateway.ResourceArgs{
			RestApi:  gateway.ID(),
			PathPart: pulumi.String("all"),
			ParentId: orderResource.ID(),
		})
		if err != nil {
			return err
		}

		_, err = apigateway.NewMethod(ctx, "OrderAllAPIGetMethod", &apigateway.MethodArgs{
			HttpMethod:    pulumi.String("GET"),
			Authorization: pulumi.String("NONE"),
			RestApi:       gateway.ID(),
			ResourceId:    resource.ID(),
		}, pulumi.DependsOn([]pulumi.Resource{gateway, resource}))
		if err != nil {
			return err
		}

		_, err = apigateway.NewIntegration(ctx, "OrderAllAPIIntegration", &apigateway.IntegrationArgs{
			HttpMethod:            pulumi.String("GET"),
			IntegrationHttpMethod: pulumi.String("POST"),
			ResourceId:            resource.ID(),
			RestApi:               gateway.ID(),
			Type:                  pulumi.String("AWS_PROXY"),
			Uri:                   function.InvokeArn,
		}, pulumi.DependsOn([]pulumi.Resource{gateway, resource, function}))
		if err != nil {
			return err
		}

		_, err = lambda.NewPermission(ctx, "OrderAllAPIPermission", &lambda.PermissionArgs{
			Action:    pulumi.String("lambda:InvokeFunction"),
			Function:  function.Name,
			Principal: pulumi.String("apigateway.amazonaws.com"),
			SourceArn: pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/GET/order/all", lambdaConfig.Region, lambdaConfig.AccountID, gateway.ID()),
		}, pulumi.DependsOn([]pulumi.Resource{gateway, resource, function}))
		if err != nil {
			return err
		}

		ctx.Export("lambda-order-all::Arn", function.Arn)

		// Add OrderUsers function
		roleArgs = &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
				"Version": "2012-10-17",
				"Statement": [
				{
					"Action": "sts:AssumeRole",
					"Principal": {
						"Service": "lambda.amazonaws.com"
					},
					"Effect": "Allow",
					"Sid": ""
				}
				]
			}`),
			Description: pulumi.String("Role for the Order Service (lambda-order-users) of the ACME Serverless Fitness Shop"),
			Tags:        pulumi.Map(tagMap),
		}

		role, err = iam.NewRole(ctx, "ACMEServerlessOrderRole-lambda-order-users", roleArgs)
		if err != nil {
			return err
		}

		// Attach the AWSLambdaBasicExecutionRole so the function can create Log groups in CloudWatch
		_, err = iam.NewRolePolicyAttachment(ctx, "AWSLambdaBasicExecutionRole-lambda-order-users", &iam.RolePolicyAttachmentArgs{
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
			Role:      role.Name,
		})
		if err != nil {
			return err
		}

		// Add the DynamoDB policy
		_, err = iam.NewRolePolicy(ctx, "ACMEServerlessOrderPolicy-lambda-order-users", &iam.RolePolicyArgs{
			Name:   pulumi.String("ACMEServerlessOrderPolicy-lambda-order-users"),
			Role:   role.Name,
			Policy: pulumi.String(dynamoCRUDPolicyString),
		})
		if err != nil {
			return err
		}

		// Create the All function
		functionArgs = &lambda.FunctionArgs{
			Description: pulumi.String("A Lambda function to get all orders for a user"),
			Runtime:     pulumi.String("go1.x"),
			Name:        pulumi.String(fmt.Sprintf("%s-lambda-order-users", ctx.Stack())),
			MemorySize:  pulumi.Int(256),
			Timeout:     pulumi.Int(10),
			Handler:     pulumi.String("lambda-order-users"),
			Environment: environment,
			Code:        pulumi.NewFileArchive("../cmd/lambda-order-users/lambda-order-users.zip"),
			Role:        role.Arn,
			Tags:        pulumi.Map(tagMap),
		}
		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-order-users", ctx.Stack()))

		function, err = lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-order-users", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		// Create the parent resources in the API Gateway
		resource, err = apigateway.NewResource(ctx, "UserOrdersAPIResource", &apigateway.ResourceArgs{
			RestApi:  gateway.ID(),
			PathPart: pulumi.String("{userid}"),
			ParentId: orderResource.ID(),
		})
		if err != nil {
			return err
		}

		_, err = apigateway.NewMethod(ctx, "UserOrdersAPIGetMethod", &apigateway.MethodArgs{
			HttpMethod:    pulumi.String("GET"),
			Authorization: pulumi.String("NONE"),
			RestApi:       gateway.ID(),
			ResourceId:    resource.ID(),
		}, pulumi.DependsOn([]pulumi.Resource{gateway, resource}))
		if err != nil {
			return err
		}

		_, err = apigateway.NewIntegration(ctx, "UserOrdersAPIIntegration", &apigateway.IntegrationArgs{
			HttpMethod:            pulumi.String("GET"),
			IntegrationHttpMethod: pulumi.String("POST"),
			ResourceId:            resource.ID(),
			RestApi:               gateway.ID(),
			Type:                  pulumi.String("AWS_PROXY"),
			Uri:                   function.InvokeArn,
		}, pulumi.DependsOn([]pulumi.Resource{gateway, resource, function}))
		if err != nil {
			return err
		}

		_, err = lambda.NewPermission(ctx, "UserOrdersAPIPermission", &lambda.PermissionArgs{
			Action:    pulumi.String("lambda:InvokeFunction"),
			Function:  function.Name,
			Principal: pulumi.String("apigateway.amazonaws.com"),
			SourceArn: pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/GET/order/*", lambdaConfig.Region, lambdaConfig.AccountID, gateway.ID()),
		}, pulumi.DependsOn([]pulumi.Resource{gateway, resource, function}))
		if err != nil {
			return err
		}

		ctx.Export("lambda-order-users::Arn", function.Arn)

		// Add Order SQS Add function
		// policyString is a policy template, derived from AWS SAM, to allow apps
		// to connect to and execute command on Amazon DynamoDB and SQS
		policyString := fmt.Sprintf(`{
	"Version": "2012-10-17",
	"Statement": [
		{
			"Action": [
				"dynamodb:GetItem",
				"dynamodb:DeleteItem",
				"dynamodb:PutItem",
				"dynamodb:Scan",
				"dynamodb:Query",
				"dynamodb:UpdateItem",
				"dynamodb:BatchWriteItem",
				"dynamodb:BatchGetItem",
				"dynamodb:DescribeTable",
				"dynamodb:ConditionCheckItem"
			],
			"Effect": "Allow",
			"Resource": "%s"
		},{
			"Action": [
				"sqs:SendMessage*"
			],
			"Effect": "Allow",
			"Resource": "%s"
		}
	]
}`, lambdaConfig.DynamoARN, lambdaConfig.PaymentRequestQueue)

		roleArgs = &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
		"Version": "2012-10-17",
		"Statement": [
		{
			"Action": "sts:AssumeRole",
			"Principal": {
				"Service": "lambda.amazonaws.com"
			},
			"Effect": "Allow",
			"Sid": ""
		}
		]
	}`),
			Description: pulumi.String("Role for the Order Service (lambda-order-sqs-add) of the ACME Serverless Fitness Shop"),
			Tags:        pulumi.Map(tagMap),
		}

		role, err = iam.NewRole(ctx, "ACMEServerlessOrderRole-lambda-order-sqs-add", roleArgs)
		if err != nil {
			return err
		}

		// Attach the AWSLambdaBasicExecutionRole so the function can create Log groups in CloudWatch
		_, err = iam.NewRolePolicyAttachment(ctx, "AWSLambdaBasicExecutionRole-lambda-order-sqs-add", &iam.RolePolicyAttachmentArgs{
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
			Role:      role.Name,
		})
		if err != nil {
			return err
		}

		// Add the policy
		_, err = iam.NewRolePolicy(ctx, "ACMEServerlessOrderPolicy-lambda-order-sqs-add", &iam.RolePolicyArgs{
			Name:   pulumi.String("ACMEServerlessOrderPolicy-lambda-order-sqs-add"),
			Role:   role.Name,
			Policy: pulumi.String(policyString),
		})
		if err != nil {
			return err
		}

		variables["RESPONSEQUEUE"] = pulumi.String(lambdaConfig.PaymentRequestQueue)
		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-order-sqs-add", ctx.Stack()))

		environment = lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap(variables),
		}

		functionArgs = &lambda.FunctionArgs{
			Description: pulumi.String("A Lambda function to add orders"),
			Runtime:     pulumi.String("go1.x"),
			Name:        pulumi.String(fmt.Sprintf("%s-lambda-order-sqs-add", ctx.Stack())),
			MemorySize:  pulumi.Int(256),
			Timeout:     pulumi.Int(10),
			Handler:     pulumi.String("lambda-order-sqs-add"),
			Environment: environment,
			Code:        pulumi.NewFileArchive("../cmd/lambda-order-sqs-add/lambda-order-sqs-add.zip"),
			Role:        role.Arn,
			Tags:        pulumi.Map(tagMap),
		}

		function, err = lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-order-sqs-add", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		// Create the parent resources in the API Gateway
		resource, err = apigateway.NewResource(ctx, "AddOrderAPIResource", &apigateway.ResourceArgs{
			RestApi:  gateway.ID(),
			PathPart: pulumi.String("{userid}"),
			ParentId: orderAddResource.ID(),
		})
		if err != nil {
			return err
		}

		_, err = apigateway.NewMethod(ctx, "OrderAddAPIGetMethod", &apigateway.MethodArgs{
			HttpMethod:    pulumi.String("POST"),
			Authorization: pulumi.String("NONE"),
			RestApi:       gateway.ID(),
			ResourceId:    resource.ID(),
		}, pulumi.DependsOn([]pulumi.Resource{gateway, resource}))
		if err != nil {
			return err
		}

		_, err = apigateway.NewIntegration(ctx, "OrderAddAPIIntegration", &apigateway.IntegrationArgs{
			HttpMethod:            pulumi.String("POST"),
			IntegrationHttpMethod: pulumi.String("POST"),
			ResourceId:            resource.ID(),
			RestApi:               gateway.ID(),
			Type:                  pulumi.String("AWS_PROXY"),
			Uri:                   function.InvokeArn,
		}, pulumi.DependsOn([]pulumi.Resource{gateway, resource, function}))
		if err != nil {
			return err
		}

		_, err = lambda.NewPermission(ctx, "OrderAddAPIPermission", &lambda.PermissionArgs{
			Action:    pulumi.String("lambda:InvokeFunction"),
			Function:  function.Name,
			Principal: pulumi.String("apigateway.amazonaws.com"),
			SourceArn: pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/POST/order/add/*", lambdaConfig.Region, lambdaConfig.AccountID, gateway.ID()),
		}, pulumi.DependsOn([]pulumi.Resource{gateway, resource, function}))
		if err != nil {
			return err
		}

		ctx.Export("lambda-order-sqs-add::Arn", function.Arn)

		// Add Order SQS Ship function
		roleArgs = &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
				"Version": "2012-10-17",
				"Statement": [
				{
					"Action": "sts:AssumeRole",
					"Principal": {
						"Service": "lambda.amazonaws.com"
					},
					"Effect": "Allow",
					"Sid": ""
				}
				]
			}`),
			Description: pulumi.String("Role for the Order Service (lambda-order-sqs-update) of the ACME Serverless Fitness Shop"),
			Tags:        pulumi.Map(tagMap),
		}

		role, err = iam.NewRole(ctx, "ACMEServerlessOrderRole-lambda-order-sqs-update", roleArgs)
		if err != nil {
			return err
		}

		_, err = iam.NewRolePolicyAttachment(ctx, "AWSLambdaBasicExecutionRole-lambda-order-sqs-update", &iam.RolePolicyAttachmentArgs{
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
			Role:      role.Name,
		})
		if err != nil {
			return err
		}

		policyString = fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [
				{
					"Action": [
						"dynamodb:GetItem",
						"dynamodb:DeleteItem",
						"dynamodb:PutItem",
						"dynamodb:Scan",
						"dynamodb:Query",
						"dynamodb:UpdateItem",
						"dynamodb:BatchWriteItem",
						"dynamodb:BatchGetItem",
						"dynamodb:DescribeTable",
						"dynamodb:ConditionCheckItem"
					],
					"Effect": "Allow",
					"Resource": "%s"
				},{
					"Action": [
						"sqs:ReceiveMessage",
						"sqs:DeleteMessage",
						"sqs:GetQueueAttributes"
					],
					"Effect": "Allow",
					"Resource": "%s"
				}
			]
		}`, lambdaConfig.DynamoARN, lambdaConfig.ShipmentResponseQueue)

		_, err = iam.NewRolePolicy(ctx, "ACMEServerlessOrderSQSPolicy-lambda-order-sqs-update", &iam.RolePolicyArgs{
			Name:   pulumi.String("ACMEServerlessPaymentSQSPolicy-lambda-order-sqs-update"),
			Role:   role.Name,
			Policy: pulumi.String(policyString),
		})
		if err != nil {
			return err
		}

		variables["REGION"] = pulumi.String(lambdaConfig.Region)
		variables["SENTRY_DSN"] = pulumi.String(lambdaConfig.SentryDSN)
		variables["VERSION"] = tags.Version
		variables["STAGE"] = pulumi.String(ctx.Stack())

		environment = lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap(variables),
		}

		functionArgs = &lambda.FunctionArgs{
			Description: pulumi.String("A Lambda function to update orders"),
			Runtime:     pulumi.String("go1.x"),
			Name:        pulumi.String(fmt.Sprintf("%s-lambda-order-sqs-update", ctx.Stack())),
			MemorySize:  pulumi.Int(256),
			Timeout:     pulumi.Int(10),
			Handler:     pulumi.String("lambda-order-sqs-update"),
			Environment: environment,
			Code:        pulumi.NewFileArchive("../cmd/lambda-order-sqs-update/lambda-order-sqs-update.zip"),
			Role:        role.Arn,
			Tags:        pulumi.Map(tagMap),
		}
		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-order-sqs-update", ctx.Stack()))

		function, err = lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-order-sqs-update", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		_, err = lambda.NewEventSourceMapping(ctx, fmt.Sprintf("%s-lambda-payment", ctx.Stack()), &lambda.EventSourceMappingArgs{
			BatchSize:      pulumi.Int(1),
			Enabled:        pulumi.Bool(true),
			FunctionName:   function.Arn,
			EventSourceArn: pulumi.String(lambdaConfig.ShipmentResponseQueue),
		})
		if err != nil {
			return err
		}

		ctx.Export("lambda-order-sqs-update::Arn", function.Arn)

		// Add Order SQS Update function
		// Create the IAM policy for the function.
		roleArgs = &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(`{
		"Version": "2012-10-17",
		"Statement": [
		{
			"Action": "sts:AssumeRole",
			"Principal": {
				"Service": "lambda.amazonaws.com"
			},
			"Effect": "Allow",
			"Sid": ""
		}
		]
	}`),
			Description: pulumi.String("Role for the Order Service (lambda-order-sqs-ship) of the ACME Serverless Fitness Shop"),
			Tags:        pulumi.Map(tagMap),
		}

		role, err = iam.NewRole(ctx, "ACMEServerlessOrderRole-lambda-order-sqs-ship", roleArgs)
		if err != nil {
			return err
		}

		_, err = iam.NewRolePolicyAttachment(ctx, "AWSLambdaBasicExecutionRole-lambda-order-sqs-ship", &iam.RolePolicyAttachmentArgs{
			PolicyArn: pulumi.String("arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"),
			Role:      role.Name,
		})
		if err != nil {
			return err
		}

		// Add a policy document to allow the function to use SQS as event source
		policyString = fmt.Sprintf(`{
		"Version": "2012-10-17",
		"Statement": [
			{
				"Action": [
					"sqs:SendMessage*"
				],
				"Effect": "Allow",
				"Resource": "%s"
			},
			{
				"Action": [
					"sqs:ReceiveMessage",
					"sqs:DeleteMessage",
					"sqs:GetQueueAttributes"
				],
				"Effect": "Allow",
				"Resource": "%s"
			}
		]
	}`, lambdaConfig.ShipmentRequestQueue, lambdaConfig.PaymentResponseQueue)

		_, err = iam.NewRolePolicy(ctx, "ACMEServerlessPaymentSQSPolicy-lambda-order-sqs-ship", &iam.RolePolicyArgs{
			Name:   pulumi.String("ACMEServerlessPaymentSQSPolicy-lambda-order-sqs-ship"),
			Role:   role.Name,
			Policy: pulumi.String(policyString),
		})
		if err != nil {
			return err
		}

		// Create the environment variables for the Lambda function
		variables["REGION"] = pulumi.String(lambdaConfig.Region)
		variables["SENTRY_DSN"] = pulumi.String(lambdaConfig.SentryDSN)
		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-order-sqs-shipment", ctx.Stack()))
		variables["VERSION"] = tags.Version
		variables["STAGE"] = pulumi.String(ctx.Stack())
		variables["RESPONSEQUEUE"] = pulumi.String(lambdaConfig.ShipmentRequestQueue)

		environment = lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap(variables),
		}

		// Create the AWS Lambda function
		functionArgs = &lambda.FunctionArgs{
			Description: pulumi.String("A Lambda function to send orders to the shipment service"),
			Runtime:     pulumi.String("go1.x"),
			Name:        pulumi.String(fmt.Sprintf("%s-lambda-order-sqs-ship", ctx.Stack())),
			MemorySize:  pulumi.Int(256),
			Timeout:     pulumi.Int(10),
			Handler:     pulumi.String("lambda-order-sqs-ship"),
			Environment: environment,
			Code:        pulumi.NewFileArchive("../cmd/lambda-order-sqs-ship/lambda-order-sqs-ship.zip"),
			Role:        role.Arn,
			Tags:        pulumi.Map(tagMap),
		}

		function, err = lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-order-sqs-ship", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		_, err = lambda.NewEventSourceMapping(ctx, fmt.Sprintf("%s-lambda-order-sqs-ship", ctx.Stack()), &lambda.EventSourceMappingArgs{
			BatchSize:      pulumi.Int(1),
			Enabled:        pulumi.Bool(true),
			FunctionName:   function.Arn,
			EventSourceArn: pulumi.String(lambdaConfig.PaymentResponseQueue),
		})
		if err != nil {
			return err
		}

		ctx.Export("lambda-order-sqs-ship::Arn", function.Arn)

		return nil
	})
}

// run creates a Cmd struct to execute the named program with the given arguments.
// After that, it starts the specified command and waits for it to complete.
func run(folder string, args string) error {
	cmd := exec.Command(shell, shellFlag, args)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = folder
	return cmd.Run()
}
