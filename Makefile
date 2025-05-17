LAMBDA_DIR := cmd/websocket
CDK_DIR := infra/cdk
BUILD_DIR := dist
CDK_ARGS := --profile globalvolume

.PHONY: all build synth deploy clean

all: build deploy

build-aws:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/bootstrap cmd/wslambda/main.go

deploy-aws:
	cd $(CDK_DIR) && cdk deploy --require-approval never $(CDK_ARGS)

clean-aws:
	rm -rf $(BUILD_DIR)
	cd $(CDK_DIR) && rm -rf cdk.out

run-local:
	go run cmd/wsserver/main.go