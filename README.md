# globalvolu.me

One volume knob to rule them all. Change volume on any device, and it syncs to everyone connected.

## How it works

WebSocket service that broadcasts volume changes to all connected clients. Built with Go, runs on AWS Lambda with API Gateway WebSocket and DynamoDB for state.

## Local dev

```bash
make run-local
```

Starts a local WebSocket server on port 8080. Clients can connect to `ws://localhost:8080/ws`.

## Deploy

```bash
make build
make deploy
```

Deploys to AWS using CDK. You'll need AWS credentials configured and CDK installed.

## Clients

- **Web**: Open `web/index.html` or deploy the built files
- **Python**: `cd clients/python && uv run client.py`

Both connect to `wss://api.globalvolu.me/ws` by default.

## Architecture

- Lambda handles WebSocket events (connect, disconnect, volume changes)
- DynamoDB stores current volume and active connections
- API Gateway WebSocket routes messages to Lambda
- Rate limited to prevent abuse (2 volume changes/sec per connection, 10 global connections)

## License

MIT
