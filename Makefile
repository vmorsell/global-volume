LAMBDA_DIR := cmd/websocket
BUILD_DIR := build
CDK_DIR := infra/cdk
CDK_ARGS := --profile globalvolume

.PHONY: all build deploy clean run-local

all: build

build:
	@mkdir -p $(BUILD_DIR)
	GOOS=linux GOARCH=arm64 go build -o $(BUILD_DIR)/bootstrap cmd/wslambda/main.go
	cd web && npm run build


deploy:
	cd $(CDK_DIR) && cdk deploy $(CDK_ARGS)

clean:
	rm -rf $(BUILD_DIR)
	cd $(CDK_DIR) && rm -rf cdk.out

run-local:
	go run cmd/wsserver/main.go