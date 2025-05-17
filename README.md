# Global Volume Server

This project provides a WebSocket server for managing global volumes.

## Features

- WebSocket-based communication
- Volume control for multiple clients
- Lightweight and fast

## Requirements

- Go 1.16 or later
- AWS account (for deployment)

## Installation

1. Clone the repository:

```
git clone https://github.com/your-repo/global-volume-server.git
cd global-volume-server
```

2. Build the server:

```
go build -o global-volume-server
```

3. Run the server:

```
./global-volume-server
```

## Deployment (AWS EC2 with CDK)

### Prerequisites
- [AWS CLI](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html) configured with your credentials
- [AWS CDK v2](https://docs.aws.amazon.com/cdk/v2/guide/getting_started.html) installed
- An EC2 key pair created in your AWS account (see below)

### Creating an EC2 Key Pair
1. Go to the [AWS EC2 Console](https://console.aws.amazon.com/ec2/)
2. In the left menu, click **Key Pairs** under **Network & Security**
3. Click **Create key pair**
4. Enter a name (e.g., `global-volume-key`), select **pem** format, and click **Create key pair**
5. Download and save the `.pem` file securely (you will need it to SSH)

### Deploying the Server
1. (Optional) Set your AWS CLI profile:
   ```sh
   export AWS_PROFILE=your-profile
   ```
2. Deploy the stack with your EC2 key pair name:
   ```sh
   cd infra/cdk
   cdk deploy -c keyName=your-keypair [--profile your-profile]
   ```
   - Replace `your-keypair` with the name of your EC2 key pair (not the .pem file).
   - The `--profile` flag is optional if you want to use a specific AWS CLI profile.

3. After deployment, find the public IP of your EC2 instance in the AWS Console.
4. SSH into your instance:
   ```sh
   ssh -i /path/to/your-keypair.pem ec2-user@<public-ip>
   ```
5. The server will be running on port 8080.

### Notes
- The stack uses the default VPC in the region set by your AWS CLI or profile.
- Port 8080 is open to the world (adjust the security group in the stack if needed).
- The CDK CLI automatically sets the AWS account and region based on your credentials/profile.

## License

This project is licensed under the MIT License.
