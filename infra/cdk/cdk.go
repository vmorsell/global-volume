package main

import (
	"fmt"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	awsapigatewayv2 "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2"
	apigwint "github.com/aws/aws-cdk-go/awscdk/v2/awsapigatewayv2integrations"
	awscertificatemanager "github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	awsdynamodb "github.com/aws/aws-cdk-go/awscdk/v2/awsdynamodb"
	awsiam "github.com/aws/aws-cdk-go/awscdk/v2/awsiam"
	awslambda "github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

func NewGlobalVolumeStack(scope constructs.Construct, id string, props *awscdk.StackProps) awscdk.Stack {
	stack := awscdk.NewStack(scope, &id, props)

	table := awsdynamodb.NewTable(stack, jsii.String("ConnectionsTable"), &awsdynamodb.TableProps{
		PartitionKey: &awsdynamodb.Attribute{
			Name: jsii.String("pk"),
			Type: awsdynamodb.AttributeType_STRING,
		},
		RemovalPolicy: awscdk.RemovalPolicy_DESTROY,
	})

	fn := awslambda.NewFunction(stack, jsii.String("WebsocketHandler"), &awslambda.FunctionProps{
		Runtime:      awslambda.Runtime_PROVIDED_AL2023(),
		Architecture: awslambda.Architecture_ARM_64(),
		Handler:      jsii.String("bootstrap"),
		Code:         awslambda.Code_FromAsset(jsii.String("../../build"), nil),
		Environment: &map[string]*string{
			"CONNECTIONS_TABLE": table.TableName(),
		},
	})

	table.GrantReadWriteData(fn)

	api := awsapigatewayv2.NewWebSocketApi(stack, jsii.String("VolumeApi"), &awsapigatewayv2.WebSocketApiProps{
		ApiName:                  jsii.String("VolumeWebsocketApi"),
		RouteSelectionExpression: jsii.String("$request.body.action"),
		ConnectRouteOptions: &awsapigatewayv2.WebSocketRouteOptions{
			Integration: apigwint.NewWebSocketLambdaIntegration(
				jsii.String("ConnectIntegration"),
				fn,
				&apigwint.WebSocketLambdaIntegrationProps{},
			),
		},
		DisconnectRouteOptions: &awsapigatewayv2.WebSocketRouteOptions{
			Integration: apigwint.NewWebSocketLambdaIntegration(
				jsii.String("DisconnectIntegration"),
				fn,
				&apigwint.WebSocketLambdaIntegrationProps{},
			),
		},
	})
	api.AddRoute(jsii.String("getState"), &awsapigatewayv2.WebSocketRouteOptions{
		Integration: apigwint.NewWebSocketLambdaIntegration(
			jsii.String("GetStateIntegration"),
			fn,
			&apigwint.WebSocketLambdaIntegrationProps{},
		),
	})
	api.AddRoute(jsii.String("reqVolumeChange"), &awsapigatewayv2.WebSocketRouteOptions{
		Integration: apigwint.NewWebSocketLambdaIntegration(
			jsii.String("ReqVolumeChangeIntegration"),
			fn,
			&apigwint.WebSocketLambdaIntegrationProps{},
		),
	})

	cert := awscertificatemanager.NewCertificate(stack, jsii.String("WsApiCertificate"), &awscertificatemanager.CertificateProps{
		DomainName: jsii.String("api.globalvolu.me"),
		Validation: awscertificatemanager.CertificateValidation_FromDns(nil),
	})

	customDomain := awsapigatewayv2.NewDomainName(stack, jsii.String("CustomDomain"), &awsapigatewayv2.DomainNameProps{
		DomainName:  jsii.String("api.globalvolu.me"),
		Certificate: cert,
	})

	awsapigatewayv2.NewWebSocketStage(stack, jsii.String("ProdStage"), &awsapigatewayv2.WebSocketStageProps{
		WebSocketApi: api,
		StageName:    jsii.String("$default"),
		AutoDeploy:   jsii.Bool(true),
		DomainMapping: &awsapigatewayv2.DomainMappingOptions{
			DomainName: customDomain,
			MappingKey: jsii.String("ws"),
		},
	})

	postArn := fmt.Sprintf(
		"arn:aws:execute-api:%s:%s:%s/$default/POST/@connections/*",
		*stack.Region(), *stack.Account(), *api.ApiId(),
	)
	fn.AddToRolePolicy(awsiam.NewPolicyStatement(&awsiam.PolicyStatementProps{
		Actions:   &[]*string{jsii.String("execute-api:ManageConnections")},
		Resources: &[]*string{jsii.String(postArn)},
	}))

	awscdk.NewCfnOutput(stack, jsii.String("WSApiURL"), &awscdk.CfnOutputProps{
		Value:       api.ApiEndpoint(),
		Description: jsii.String("WebSocket API URL"),
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewGlobalVolumeStack(app, "GlobalVolumeStack", &awscdk.StackProps{})
	app.Synth(nil)
}
