package main

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	awsapigatewayv2 "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	apigwint "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	awscertificatemanager "github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	awscloudwatch "github.com/aws/aws-cdk-go/awscdk/v2/awscloudwatch"
	awsdynamodb "github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	awsiam "github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	awslambda "github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

const (
	resourceNameTable        = "ConnectionsTable"
	resourceNameFunction     = "WebsocketHandler"
	resourceNameAPI          = "VolumeApi"
	resourceNameCertificate  = "WsApiCertificate"
	resourceNameCustomDomain = "CustomDomain"
	resourceNameStage        = "ProdStage"
	resourceNameOutputAPIURL = "WSApiURL"

	integrationNameConnect             = "ConnectIntegration"
	integrationNameDisconnect          = "DisconnectIntegration"
	integrationNameGetVolume           = "GetVolumeIntegration"
	integrationNameGetConnectedClients = "GetConnectedClientsCountIntegration"
	integrationNameRequestVolumeChange = "ReqVolumeChangeIntegration"

	routeKeyConnect             = "$connect"
	routeKeyDisconnect          = "$disconnect"
	routeKeyGetVolume           = "getVolume"
	routeKeyGetConnectedClients = "getConnectedClientsCount"
	routeKeyRequestVolumeChange = "reqVolumeChange"

	apiName                    = "VolumeWebsocketApi"
	routeSelectionExpression   = "$request.body.action"
	stageName                  = "$default"
	domainName                 = "api.globalvolu.me"
	domainMappingKey           = "ws"
	lambdaHandler              = "bootstrap"
	lambdaCodePath             = "../../build"
	envVarConnectionsTable     = "CONNECTIONS_TABLE"
	iamActionManageConnections = "execute-api:ManageConnections"
	iamResourcePattern         = "arn:aws:execute-api:%s:%s:%s/$default/POST/@connections/*"
)

func NewGlobalVolumeStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, props)

	table := createDynamoDBTable(stack)
	function := createLambdaFunction(stack, table)
	api := createWebSocketAPI(stack, function)
	createCustomDomain(stack, api)
	grantAPIPermissions(stack, function, api)
	createCloudWatchAlarms(stack, function, table, api)

	createOutputs(stack, api)

	return stack
}

func createDynamoDBTable(stack awscdk.Stack) awsdynamodb.Table {
	return awsdynamodb.NewTable(stack, jsii.String(resourceNameTable), &awsdynamodb.TableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})
}

func createLambdaFunction(stack awscdk.Stack, table awsdynamodb.Table) awslambda.Function {
	fn := awslambda.NewFunction(stack, jsii.String(resourceNameFunction), &awslambda.FunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
		Architecture: awslambda.Architecture_ARM_64(),
		Handler:      jsii.String(lambdaHandler),
		Code:         awslambda.Code_FromAsset(jsii.String(lambdaCodePath), nil),
		Environment: &map[string]*string{
			envVarConnectionsTable: table.TableName(),
		},
	})

	table.GrantReadWriteData(fn)

	return fn
}

func createWebSocketAPI(stack awscdk.Stack, function awslambda.Function) awsapigatewayv2.WebSocketApi {
	api := awsapigatewayv2.NewWebSocketApi(stack, jsii.String(resourceNameAPI), &awsapigatewayv2.WebSocketApiProps{
		ApiName:                  jsii.String(apiName),
		RouteSelectionExpression: jsii.String(routeSelectionExpression),
		ConnectRouteOptions: &awsapigatewayv2.WebSocketRouteOptions{
			Integration: apigwint.NewWebSocketLambdaIntegration(
				jsii.String(integrationNameConnect),
				function,
				&apigwint.WebSocketLambdaIntegrationProps{},
			),
		},
		DisconnectRouteOptions: &awsapigatewayv2.WebSocketRouteOptions{
			Integration: apigwint.NewWebSocketLambdaIntegration(
				jsii.String(integrationNameDisconnect),
				function,
				&apigwint.WebSocketLambdaIntegrationProps{},
			),
		},
	})

	addRoute(api, routeKeyGetVolume, integrationNameGetVolume, function)
	addRoute(api, routeKeyGetConnectedClients, integrationNameGetConnectedClients, function)
	addRoute(api, routeKeyRequestVolumeChange, integrationNameRequestVolumeChange, function)

	return api
}

func addRoute(api awsapigatewayv2.WebSocketApi, routeKey, integrationName string, function awslambda.Function) {
	api.AddRoute(jsii.String(routeKey), &awsapigatewayv2.WebSocketRouteOptions{
		Integration: apigwint.NewWebSocketLambdaIntegration(
			jsii.String(integrationName),
			function,
			&apigwint.WebSocketLambdaIntegrationProps{},
		),
	})
}

func createCustomDomain(stack awscdk.Stack, api awsapigatewayv2.WebSocketApi) {
	cert := awscertificatemanager.NewCertificate(stack, jsii.String(resourceNameCertificate), &awscertificatemanager.CertificateProps{
		DomainName: jsii.String(domainName),
		Validation: awscertificatemanager.CertificateValidation_FromDns(nil),
	})

	customDomain := awsapigatewayv2.NewDomainName(stack, jsii.String(resourceNameCustomDomain), &awsapigatewayv2.DomainNameProps{
		DomainName:  jsii.String(domainName),
		Certificate: cert,
	})

	awsapigatewayv2.NewWebSocketStage(stack, jsii.String(resourceNameStage), &awsapigatewayv2.WebSocketStageProps{
		WebSocketApi: api,
		StageName:    jsii.String(stageName),
		AutoDeploy:   jsii.Bool(true),
		DomainMapping: &awsapigatewayv2.DomainMappingOptions{
			DomainName: customDomain,
			MappingKey: jsii.String(domainMappingKey),
		},
	})
}

func grantAPIPermissions(stack awscdk.Stack, function awslambda.Function, api awsapigatewayv2.WebSocketApi) {
	postArn := fmt.Sprintf(
		iamResourcePattern,
		*stack.Region(),
		*stack.Account(),
		*api.ApiId(),
	)

	function.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   &[]*string{jsii.String(iamActionManageConnections)},
		Resources: &[]*string{jsii.String(postArn)},
	}))
}

func createCloudWatchAlarms(stack awscdk.Stack, function awslambda.Function, table awsdynamodb.Table, api awsapigatewayv2.WebSocketApi) {
	lambdaInvocations := function.MetricInvocations(&awscloudwatch.MetricOptions{
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
		Statistic: jsii.String("Sum"),
	})
	awscloudwatch.NewAlarm(stack, jsii.String("HighLambdaInvocations"), &awscloudwatch.AlarmProps{
		Metric:            lambdaInvocations,
		Threshold:         jsii.Number(1000),
		EvaluationPeriods: jsii.Number(1),
		AlarmDescription:  jsii.String("Alert when Lambda invocations exceed 1000 per minute (potential abuse)"),
	})

	dynamoWriteUnits := table.MetricConsumedWriteCapacityUnits(&awscloudwatch.MetricOptions{
		Period:    awscdk.Duration_Minutes(jsii.Number(1)),
		Statistic: jsii.String("Sum"),
	})
	awscloudwatch.NewAlarm(stack, jsii.String("HighDynamoWriteUnits"), &awscloudwatch.AlarmProps{
		Metric:            dynamoWriteUnits,
		Threshold:         jsii.Number(500),
		EvaluationPeriods: jsii.Number(1),
		AlarmDescription:  jsii.String("Alert when DynamoDB write units exceed 500 per minute (potential abuse)"),
	})

	lambdaErrors := function.MetricErrors(&awscloudwatch.MetricOptions{
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		Statistic: jsii.String("Sum"),
	})
	lambdaInvocations5Min := function.MetricInvocations(&awscloudwatch.MetricOptions{
		Period:    awscdk.Duration_Minutes(jsii.Number(5)),
		Statistic: jsii.String("Sum"),
	})
	errorRate := awscloudwatch.NewMathExpression(&awscloudwatch.MathExpressionProps{
		Expression: jsii.String("errors / invocations * 100"),
		UsingMetrics: &map[string]awscloudwatch.IMetric{
			"errors":      lambdaErrors,
			"invocations": lambdaInvocations5Min,
		},
	})
	awscloudwatch.NewAlarm(stack, jsii.String("HighLambdaErrorRate"), &awscloudwatch.AlarmProps{
		Metric:            errorRate,
		Threshold:         jsii.Number(10),
		EvaluationPeriods: jsii.Number(1),
		AlarmDescription:  jsii.String("Alert when Lambda error rate exceeds 10%"),
	})
}

func createOutputs(stack awscdk.Stack, api awsapigatewayv2.WebSocketApi) {
	awscdk.NewCfnOutput(stack, jsii.String(resourceNameOutputAPIURL), &awscdk.CfnOutputProps{
		Value:       api.ApiEndpoint(),
		Description: jsii.String("WebSocket API URL"),
	})
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)
	NewGlobalVolumeStack(app, "GlobalVolumeStack", &awscdk.StackProps{})
	app.Synth(nil)
}
