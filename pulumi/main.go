package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/pulumi/pulumi-aws/sdk/go/aws/apigateway"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/dynamodb"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/iam"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/lambda"
	"github.com/pulumi/pulumi-aws/sdk/go/aws/sqs"
	"github.com/pulumi/pulumi/sdk/go/pulumi"
	"github.com/pulumi/pulumi/sdk/go/pulumi/config"
	"github.com/retgits/pulumi-helpers/builder"
	gw "github.com/retgits/pulumi-helpers/gateway"
	"github.com/retgits/pulumi-helpers/sampolicies"
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

// GenericConfig contains the key-value pairs for the configuration of AWS in this stack
type GenericConfig struct {
	// The AWS region used
	Region string

	// The DSN used to connect to Sentry
	SentryDSN string `json:"sentrydsn"`

	// The AWS AccountID to use
	AccountID string `json:"accountid"`
}

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Get the region
		region, found := ctx.GetConfig("aws:region")
		if !found {
			return fmt.Errorf("region not found")
		}

		// Read the configuration data from Pulumi.<stack>.yaml
		conf := config.New(ctx, "awsconfig")

		// Create a new Tags object with the data from the configuration
		var tags Tags
		conf.RequireObject("tags", &tags)

		// Create a new GenericConfig object with the data from the configuration
		var genericConfig GenericConfig
		conf.RequireObject("generic", &genericConfig)
		genericConfig.Region = region

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
			buildFactory := builder.NewFactory().WithFolder(fnFolder)
			buildFactory.MustBuild()
			buildFactory.MustZip()
		}

		// Create a factory to get policies from
		iamFactory := sampolicies.NewFactory().WithAccountID(genericConfig.AccountID).WithPartition("aws").WithRegion(genericConfig.Region)

		// Lookup the DynamoDB table
		dynamoTable, err := dynamodb.LookupTable(ctx, &dynamodb.LookupTableArgs{
			Name: fmt.Sprintf("%s-acmeserverless-dynamodb", ctx.Stack()),
		})
		if err != nil {
			return err
		}
		if dynamoTable == nil {
			return fmt.Errorf("unable to find dynamodb table %s-acmeserverless-dynamodb", ctx.Stack())
		}

		// Lookup the SQS queues
		paymentResponseQueue, err := sqs.LookupQueue(ctx, &sqs.LookupQueueArgs{
			Name: fmt.Sprintf("%s-acmeserverless-sqs-payment-response", ctx.Stack()),
		})
		if err != nil {
			return err
		}

		paymentRequestQueue, err := sqs.LookupQueue(ctx, &sqs.LookupQueueArgs{
			Name: fmt.Sprintf("%s-acmeserverless-sqs-payment-request", ctx.Stack()),
		})
		if err != nil {
			return err
		}

		shipmentResponseQueue, err := sqs.LookupQueue(ctx, &sqs.LookupQueueArgs{
			Name: fmt.Sprintf("%s-acmeserverless-sqs-shipment-response", ctx.Stack()),
		})
		if err != nil {
			return err
		}

		shipmentRequestQueue, err := sqs.LookupQueue(ctx, &sqs.LookupQueueArgs{
			Name: fmt.Sprintf("%s-acmeserverless-sqs-shipment-request", ctx.Stack()),
		})
		if err != nil {
			return err
		}

		// dynamoPolicy is a policy template, derived from AWS SAM, to allow apps
		// to connect to and execute command on Amazon DynamoDB
		iamFactory.ClearPolicies()
		iamFactory.AddDynamoDBCrudPolicy(dynamoTable.Name)
		dynamoPolicy, err := iamFactory.GetPolicyStatement()
		if err != nil {
			return err
		}

		// Add OrderAll function
		roleArgs := &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(sampolicies.AssumeRoleLambda()),
			Description:      pulumi.String("Role for the Order Service (lambda-order-all) of the ACME Serverless Fitness Shop"),
			Tags:             pulumi.Map(tagMap),
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
			Policy: pulumi.String(dynamoPolicy),
		})
		if err != nil {
			return err
		}

		// All functions will have the same environment variables, with the exception
		// of the function name
		variables := make(map[string]pulumi.StringInput)
		variables["REGION"] = pulumi.String(genericConfig.Region)
		variables["SENTRY_DSN"] = pulumi.String(genericConfig.SentryDSN)
		variables["VERSION"] = tags.Version
		variables["STAGE"] = pulumi.String(ctx.Stack())
		variables["TABLE"] = pulumi.String(dynamoTable.Name)

		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-order-all", ctx.Stack()))
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

		orderAllFunction, err := lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-order-all", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		ctx.Export("lambda-order-all::Arn", orderAllFunction.Arn)

		// Add OrderUsers function
		roleArgs = &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(sampolicies.AssumeRoleLambda()),
			Description:      pulumi.String("Role for the Order Service (lambda-order-users) of the ACME Serverless Fitness Shop"),
			Tags:             pulumi.Map(tagMap),
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
			Policy: pulumi.String(dynamoPolicy),
		})
		if err != nil {
			return err
		}

		// Create the All function
		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-order-users", ctx.Stack()))
		environment = lambda.FunctionEnvironmentArgs{
			Variables: pulumi.StringMap(variables),
		}

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

		orderUsersFunction, err := lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-order-users", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		ctx.Export("lambda-order-users::Arn", orderUsersFunction.Arn)

		// Add Order SQS Add function
		// policyString is a policy template, derived from AWS SAM, to allow apps
		// to connect to and execute command on Amazon DynamoDB and SQS
		iamFactory.ClearPolicies()
		iamFactory.AddDynamoDBCrudPolicy(dynamoTable.Name)
		iamFactory.AddSQSSendMessagePolicy(paymentRequestQueue.Name)
		policies, err := iamFactory.GetPolicyStatement()
		if err != nil {
			return err
		}

		roleArgs = &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(sampolicies.AssumeRoleLambda()),
			Description:      pulumi.String("Role for the Order Service (lambda-order-sqs-add) of the ACME Serverless Fitness Shop"),
			Tags:             pulumi.Map(tagMap),
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
			Policy: pulumi.String(policies),
		})
		if err != nil {
			return err
		}

		variables["RESPONSEQUEUE"] = pulumi.String(paymentRequestQueue.Arn)
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

		orderAddFunction, err := lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-order-sqs-add", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		ctx.Export("lambda-order-sqs-add::Arn", orderAddFunction.Arn)

		// Add Order SQS Ship function
		roleArgs = &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(sampolicies.AssumeRoleLambda()),
			Description:      pulumi.String("Role for the Order Service (lambda-order-sqs-update) of the ACME Serverless Fitness Shop"),
			Tags:             pulumi.Map(tagMap),
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

		iamFactory.ClearPolicies()
		iamFactory.AddDynamoDBCrudPolicy(dynamoTable.Name)
		iamFactory.AddSQSPollerPolicy(shipmentResponseQueue.Name)
		policies, err = iamFactory.GetPolicyStatement()
		if err != nil {
			return err
		}

		_, err = iam.NewRolePolicy(ctx, "ACMEServerlessOrderSQSPolicy-lambda-order-sqs-update", &iam.RolePolicyArgs{
			Name:   pulumi.String("ACMEServerlessPaymentSQSPolicy-lambda-order-sqs-update"),
			Role:   role.Name,
			Policy: pulumi.String(policies),
		})
		if err != nil {
			return err
		}

		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-order-sqs-update", ctx.Stack()))
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

		orderUpdateFunction, err := lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-order-sqs-update", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		_, err = lambda.NewEventSourceMapping(ctx, fmt.Sprintf("%s-lambda-order-sqs-update", ctx.Stack()), &lambda.EventSourceMappingArgs{
			BatchSize:      pulumi.Int(1),
			Enabled:        pulumi.Bool(true),
			FunctionName:   orderUpdateFunction.Arn,
			EventSourceArn: pulumi.String(shipmentResponseQueue.Arn),
		})
		if err != nil {
			return err
		}

		ctx.Export("lambda-order-sqs-update::Arn", orderUpdateFunction.Arn)

		// Add Order SQS Update function
		// Create the IAM policy for the function.
		roleArgs = &iam.RoleArgs{
			AssumeRolePolicy: pulumi.String(sampolicies.AssumeRoleLambda()),
			Description:      pulumi.String("Role for the Order Service (lambda-order-sqs-ship) of the ACME Serverless Fitness Shop"),
			Tags:             pulumi.Map(tagMap),
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
		iamFactory.ClearPolicies()
		iamFactory.AddSQSSendMessagePolicy(shipmentRequestQueue.Name)
		iamFactory.AddSQSPollerPolicy(paymentResponseQueue.Name)
		policies, err = iamFactory.GetPolicyStatement()
		if err != nil {
			return err
		}

		_, err = iam.NewRolePolicy(ctx, "ACMEServerlessPaymentSQSPolicy-lambda-order-sqs-ship", &iam.RolePolicyArgs{
			Name:   pulumi.String("ACMEServerlessPaymentSQSPolicy-lambda-order-sqs-ship"),
			Role:   role.Name,
			Policy: pulumi.String(policies),
		})
		if err != nil {
			return err
		}

		// Create the environment variables for the Lambda function
		variables["FUNCTION_NAME"] = pulumi.String(fmt.Sprintf("%s-lambda-order-sqs-shipment", ctx.Stack()))
		variables["RESPONSEQUEUE"] = pulumi.String(shipmentRequestQueue.Arn)

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

		orderShipFunction, err := lambda.NewFunction(ctx, fmt.Sprintf("%s-lambda-order-sqs-ship", ctx.Stack()), functionArgs)
		if err != nil {
			return err
		}

		_, err = lambda.NewEventSourceMapping(ctx, fmt.Sprintf("%s-lambda-order-sqs-ship", ctx.Stack()), &lambda.EventSourceMappingArgs{
			BatchSize:      pulumi.Int(1),
			Enabled:        pulumi.Bool(true),
			FunctionName:   orderShipFunction.Arn,
			EventSourceArn: pulumi.String(paymentResponseQueue.Arn),
		})
		if err != nil {
			return err
		}

		ctx.Export("lambda-order-sqs-ship::Arn", orderShipFunction.Arn)

		// Create the API Gateway Policy
		iamFactory.ClearPolicies()
		iamFactory.AddAssumeRoleLambda()
		iamFactory.AddExecuteAPI()
		policies, err = iamFactory.GetPolicyStatement()
		if err != nil {
			return err
		}

		// Read the OpenAPI specification
		bytes, err := ioutil.ReadFile("../api/openapi.json")
		if err != nil {
			return err
		}

		// Create an API Gateway
		gateway, err := apigateway.NewRestApi(ctx, "OrderService", &apigateway.RestApiArgs{
			Name:        pulumi.String("OrderService"),
			Description: pulumi.String("ACME Serverless Fitness Shop - Order"),
			Tags:        pulumi.Map(tagMap),
			Policy:      pulumi.String(policies),
			Body:        pulumi.StringPtr(string(bytes)),
		})
		if err != nil {
			return err
		}

		gatewayURL := gateway.ID().ToStringOutput().ApplyString(func(id string) string {
			resource := gw.MustGetGatewayResource(ctx, id, "/order/all")

			_, err = apigateway.NewIntegration(ctx, "OrderAllAPIIntegration", &apigateway.IntegrationArgs{
				HttpMethod:            pulumi.String("GET"),
				IntegrationHttpMethod: pulumi.String("POST"),
				ResourceId:            pulumi.String(resource.Id),
				RestApi:               gateway.ID(),
				Type:                  pulumi.String("AWS_PROXY"),
				Uri:                   orderAllFunction.InvokeArn,
			})
			if err != nil {
				fmt.Println(err)
			}

			_, err = lambda.NewPermission(ctx, "OrderAllAPIPermission", &lambda.PermissionArgs{
				Action:    pulumi.String("lambda:InvokeFunction"),
				Function:  orderAllFunction.Name,
				Principal: pulumi.String("apigateway.amazonaws.com"),
				SourceArn: pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/GET/order/all", genericConfig.Region, genericConfig.AccountID, gateway.ID()),
			})
			if err != nil {
				fmt.Println(err)
			}

			resource = gw.MustGetGatewayResource(ctx, id, "/order/{userid}")

			_, err = apigateway.NewIntegration(ctx, "UserOrdersAPIIntegration", &apigateway.IntegrationArgs{
				HttpMethod:            pulumi.String("GET"),
				IntegrationHttpMethod: pulumi.String("POST"),
				ResourceId:            pulumi.String(resource.Id),
				RestApi:               gateway.ID(),
				Type:                  pulumi.String("AWS_PROXY"),
				Uri:                   orderUsersFunction.InvokeArn,
			})
			if err != nil {
				fmt.Println(err)
			}

			_, err = lambda.NewPermission(ctx, "UserOrdersAPIPermission", &lambda.PermissionArgs{
				Action:    pulumi.String("lambda:InvokeFunction"),
				Function:  orderUsersFunction.Name,
				Principal: pulumi.String("apigateway.amazonaws.com"),
				SourceArn: pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/GET/order/*", genericConfig.Region, genericConfig.AccountID, gateway.ID()),
			})
			if err != nil {
				fmt.Println(err)
			}

			resource = gw.MustGetGatewayResource(ctx, id, "/order/add/{userid}")

			_, err = apigateway.NewIntegration(ctx, "OrderAddAPIIntegration", &apigateway.IntegrationArgs{
				HttpMethod:            pulumi.String("POST"),
				IntegrationHttpMethod: pulumi.String("POST"),
				ResourceId:            pulumi.String(resource.Id),
				RestApi:               gateway.ID(),
				Type:                  pulumi.String("AWS_PROXY"),
				Uri:                   orderAddFunction.InvokeArn,
			})
			if err != nil {
				fmt.Println(err)
			}

			_, err = lambda.NewPermission(ctx, "OrderAddAPIPermission", &lambda.PermissionArgs{
				Action:    pulumi.String("lambda:InvokeFunction"),
				Function:  orderAddFunction.Name,
				Principal: pulumi.String("apigateway.amazonaws.com"),
				SourceArn: pulumi.Sprintf("arn:aws:execute-api:%s:%s:%s/*/POST/order/add/*", genericConfig.Region, genericConfig.AccountID, gateway.ID()),
			})
			if err != nil {
				fmt.Println(err)
			}

			return fmt.Sprintf("https://%s.execute-api.%s.amazonaws.com/prod/", id, genericConfig.Region)
		})

		ctx.Export("Gateway::URL", gatewayURL)
		return nil
	})
}
