<p align="center">
  <img src="assets/kumo.jpg" alt="kumo logo" width="480">
  <br><br>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/sivchari/kumo" alt="Go Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/github/license/sivchari/kumo" alt="License"></a>
  <a href="https://github.com/sivchari/kumo/releases"><img src="https://img.shields.io/github/v/release/sivchari/kumo" alt="Release"></a>
  <a href="https://github.com/sivchari/kumo/actions/workflows/lint.yaml"><img src="https://github.com/sivchari/kumo/actions/workflows/lint.yaml/badge.svg" alt="Lint"></a>
  <a href="https://github.com/sivchari/kumo/actions/workflows/integration-test.yaml"><img src="https://github.com/sivchari/kumo/actions/workflows/integration-test.yaml/badge.svg" alt="Integration Tests"></a>
</p>

<p align="center">A lightweight AWS service emulator written in Go.<br>Works as both a CI/CD testing tool and a local development server with optional data persistence.</p>

## Features

- **No authentication required** - Perfect for CI environments
- **Single binary** - Easy to distribute and deploy
- **Docker support** - Run as a container
- **Lightweight** - Fast startup, minimal resource usage
- **AWS SDK v2 compatible** - Works seamlessly with Go AWS SDK v2
- **Optional data persistence** - Survive restarts with `KUMO_DATA_DIR`

## Supported Services (81 services)

<!-- BEGIN SERVICES: auto-generated from each service's Meta(); run `make readme` to update. Do not edit by hand. -->
### Storage
| Service | Description |
|---------|-------------|
| DynamoDB | NoSQL database |
| DynamoDB Streams | DynamoDB change data capture |
| EBS | Block storage |
| ElastiCache | In-memory caching |
| Glacier | Archive storage |
| MemoryDB | Redis-compatible database |
| S3 | Object storage |
| S3 Control | S3 account-level operations |
| S3 Tables | S3 table buckets |

### Compute
| Service | Description |
|---------|-------------|
| Batch | Batch computing |
| EC2 | Virtual machines |
| Elastic Beanstalk | Application deployment |
| Lambda | Serverless functions |

### Container
| Service | Description |
|---------|-------------|
| ECR | Container registry |
| ECS | Container orchestration |
| EKS | Kubernetes service |

### Database
| Service | Description |
|---------|-------------|
| DocumentDB | MongoDB-compatible database |
| Neptune | Graph database |
| RDS | Relational database service |
| Redshift | Data warehousing |

### Messaging & Integration
| Service | Description |
|---------|-------------|
| EventBridge | Event bus |
| Firehose | Data delivery |
| Kinesis | Real-time streaming |
| MQ | Message broker (ActiveMQ/RabbitMQ) |
| MSK (Kafka) | Managed streaming for Kafka |
| Pipes | Event-driven integration |
| SNS | Pub/Sub messaging |
| SQS | Message queuing |

### Security & Identity
| Service | Description |
|---------|-------------|
| ACM | Certificate management |
| Cognito | User authentication |
| IAM | Identity and access management |
| KMS | Key management |
| Macie | Data security and privacy |
| STS | Security token service |
| Secrets Manager | Secret storage |
| Security Lake | Security data lake |

### Monitoring & Logging
| Service | Description |
|---------|-------------|
| CloudTrail | API audit logging |
| CloudWatch | Metrics and alarms |
| CloudWatch Logs | Log management |
| X-Ray | Distributed tracing |

### Networking & Content Delivery
| Service | Description |
|---------|-------------|
| API Gateway | API management (REST API) |
| API Gateway v2 | API management (HTTP/WebSocket API) |
| App Mesh | Service mesh |
| CloudFront | CDN |
| ELBv2 | Load balancing |
| Global Accelerator | Network acceleration |
| Location | Location-based services |
| Route 53 | DNS service |
| Route 53 Resolver | DNS resolver |

### Application Integration
| Service | Description |
|---------|-------------|
| Amplify | Full-stack application hosting |
| AppSync | GraphQL API |
| Pinpoint SMS Voice v2 | SMS messaging |
| SES | Email service |
| SES v2 | Email service (v2 API) |
| Scheduler | Task scheduling |
| Step Functions | Workflow orchestration |

### Management & Configuration
| Service | Description |
|---------|-------------|
| Backup | Centralized backup service |
| Cloud Control API | Unified CRUD API for cloud resources |
| CloudFormation | Infrastructure as code |
| CodeConnections | Source code connections |
| Config | Resource configuration |
| Organizations | Multi-account management |
| SSM | Systems Manager |
| Service Quotas | Service limit management |

### Analytics & ML
| Service | Description |
|---------|-------------|
| Athena | SQL query service |
| Comprehend | NLP service |
| Data Exchange | Data marketplace |
| Entity Resolution | Entity matching |
| Forecast | Time-series forecasting |
| Glue | ETL service |
| Rekognition | Image/video analysis |
| SageMaker | Machine learning |

### Developer Tools
| Service | Description |
|---------|-------------|
| CodeGuru Profiler | Application profiling |
| CodeGuru Reviewer | Automated code review |

### Other Services
| Service | Description |
|---------|-------------|
| Cost Explorer | Cost analysis |
| DLM | Data lifecycle manager |
| Directory Service | Microsoft AD |
| EMR Serverless | Big data processing |
| FinSpace | Financial data management |
| GameLift | Game server hosting |
| Resilience Hub | Application resilience |
<!-- END SERVICES -->

## Quick Start

### Docker

```bash
docker run -p 4566:4566 ghcr.io/sivchari/kumo:latest
```

With data persistence:

```bash
docker run -p 4566:4566 \
  -e KUMO_DATA_DIR=/data \
  -v kumo-data:/data \
  ghcr.io/sivchari/kumo:latest
```

### Binary

```bash
# Build
make build

# Run
./bin/kumo

# Run with data persistence
KUMO_DATA_DIR=./data ./bin/kumo
```

### Docker Compose

```yaml
services:
  kumo:
    image: ghcr.io/sivchari/kumo:latest
    ports:
      - "4566:4566"
```

With data persistence:

```yaml
services:
  kumo:
    image: ghcr.io/sivchari/kumo:latest
    ports:
      - "4566:4566"
    environment:
      - KUMO_DATA_DIR=/data
    volumes:
      - kumo-data:/data

volumes:
  kumo-data:
```

## Usage Examples

### S3

```go
package main

import (
    "context"
    "strings"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/s3"
)

func main() {
    cfg, _ := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion("us-east-1"),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
    )

    client := s3.NewFromConfig(cfg, func(o *s3.Options) {
        o.BaseEndpoint = aws.String("http://localhost:4566")
        o.UsePathStyle = true
    })

    // Create bucket
    client.CreateBucket(context.TODO(), &s3.CreateBucketInput{
        Bucket: aws.String("my-bucket"),
    })

    // Put object
    client.PutObject(context.TODO(), &s3.PutObjectInput{
        Bucket: aws.String("my-bucket"),
        Key:    aws.String("hello.txt"),
        Body:   strings.NewReader("Hello, World!"),
    })
}
```

### SQS

```go
package main

import (
    "context"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/sqs"
)

func main() {
    cfg, _ := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion("us-east-1"),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
    )

    client := sqs.NewFromConfig(cfg, func(o *sqs.Options) {
        o.BaseEndpoint = aws.String("http://localhost:4566")
    })

    // Create queue
    result, _ := client.CreateQueue(context.TODO(), &sqs.CreateQueueInput{
        QueueName: aws.String("my-queue"),
    })

    // Send message
    client.SendMessage(context.TODO(), &sqs.SendMessageInput{
        QueueUrl:    result.QueueUrl,
        MessageBody: aws.String("Hello from SQS!"),
    })

    // Receive message
    messages, _ := client.ReceiveMessage(context.TODO(), &sqs.ReceiveMessageInput{
        QueueUrl: result.QueueUrl,
    })

    for _, msg := range messages.Messages {
        fmt.Println(*msg.Body)
    }
}
```

### DynamoDB

```go
package main

import (
    "context"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb"
    "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func main() {
    cfg, _ := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion("us-east-1"),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
    )

    client := dynamodb.NewFromConfig(cfg, func(o *dynamodb.Options) {
        o.BaseEndpoint = aws.String("http://localhost:4566")
    })

    // Create table
    client.CreateTable(context.TODO(), &dynamodb.CreateTableInput{
        TableName: aws.String("users"),
        KeySchema: []types.KeySchemaElement{
            {AttributeName: aws.String("id"), KeyType: types.KeyTypeHash},
        },
        AttributeDefinitions: []types.AttributeDefinition{
            {AttributeName: aws.String("id"), AttributeType: types.ScalarAttributeTypeS},
        },
        BillingMode: types.BillingModePayPerRequest,
    })

    // Put item
    client.PutItem(context.TODO(), &dynamodb.PutItemInput{
        TableName: aws.String("users"),
        Item: map[string]types.AttributeValue{
            "id":   &types.AttributeValueMemberS{Value: "user-1"},
            "name": &types.AttributeValueMemberS{Value: "Alice"},
        },
    })
}
```

### Secrets Manager

```go
package main

import (
    "context"
    "fmt"

    "github.com/aws/aws-sdk-go-v2/aws"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/credentials"
    "github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

func main() {
    cfg, _ := config.LoadDefaultConfig(context.TODO(),
        config.WithRegion("us-east-1"),
        config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
    )

    client := secretsmanager.NewFromConfig(cfg, func(o *secretsmanager.Options) {
        o.BaseEndpoint = aws.String("http://localhost:4566")
    })

    // Create secret
    client.CreateSecret(context.TODO(), &secretsmanager.CreateSecretInput{
        Name:         aws.String("my-secret"),
        SecretString: aws.String(`{"username":"admin","password":"secret123"}`),
    })

    // Get secret
    result, _ := client.GetSecretValue(context.TODO(), &secretsmanager.GetSecretValueInput{
        SecretId: aws.String("my-secret"),
    })

    fmt.Println(*result.SecretString)
}
```

## Configuration

Environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `KUMO_HOST` | `0.0.0.0` | Server bind address |
| `KUMO_PORT` | `4566` | Server port |
| `KUMO_LOG_LEVEL` | `info` | Log level (debug, info, warn, error) |
| `KUMO_DATA_DIR` | (unset) | Directory for persistent storage. When unset, data is in-memory only. |

## Logging

kumo logs all requests with structured fields. The log level controls the amount of detail:

### INFO (default)

Each request is logged with method, path, status, duration, and API action name:

```
level=INFO msg=request method=POST path=/ status=200 duration=61µs request_id=... target=secretsmanager.CreateSecret
level=INFO msg=request method=PUT path=/my-bucket pattern=/{bucket} status=200 duration=30µs request_id=...
```

- `target` -- appears for JSON/Query protocol services (Secrets Manager, DynamoDB, SQS, etc.)
- `action` -- appears for Query protocol services (EC2, SNS, etc.) when Action is in the URL query string

### DEBUG

In addition to INFO output, the full request body is logged:

```
level=DEBUG msg="request body" request_id=... body={"Name":"my-secret","SecretString":"..."}
```

Enable with:

```bash
KUMO_LOG_LEVEL=debug ./bin/kumo
```

## Data Persistence

By default kumo runs as a pure in-memory emulator -- all data is lost when the process stops. This is ideal for CI/CD pipelines where each test run starts from a clean state.

For local development, set `KUMO_DATA_DIR` to enable persistent storage:

```bash
KUMO_DATA_DIR=./data ./bin/kumo
```

When enabled:

- On startup, each service loads its previous state from `$KUMO_DATA_DIR/{service}.json`.
- On graceful shutdown (SIGTERM/SIGINT), each service saves its current state.
- The data directory is created automatically if it does not exist.
- Writes are atomic (tmp file + rename) to prevent corruption on crash.
- Ephemeral state (SQS in-flight messages, S3 multipart uploads) is not persisted.

```
$KUMO_DATA_DIR/
  s3.json
  sqs.json
  dynamodb.json
  iam.json
  ...
```

## kumo-specific Endpoints

kumo provides additional endpoints under the `/kumo/` prefix for testing purposes. These are not part of any AWS API but are useful for verifying application behavior in tests.

| Method | Path | Description |
|--------|------|-------------|
| GET | `/kumo/ses/v2/sent-emails` | Retrieve a list of emails sent via the SES v2 `SendEmail` API |
| GET | `/kumo/pinpointsmsvoicev2/sent-messages` | Retrieve a list of SMS messages sent via the Pinpoint SMS Voice v2 `SendTextMessage` API |

### Example: Retrieving sent emails

```bash
curl http://localhost:4566/kumo/ses/v2/sent-emails
```

Response:

```json
{
  "SentEmails": [
    {
      "MessageId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      "FromEmailAddress": "sender@example.com",
      "Destination": {
        "ToAddresses": ["recipient@example.com"]
      },
      "Subject": "Hello",
      "Body": "Hello, World!",
      "SentAt": "2025-01-01T00:00:00Z"
    }
  ]
}
```

### Example: Retrieving sent SMS messages

```bash
curl http://localhost:4566/kumo/pinpointsmsvoicev2/sent-messages
```

Response:

```json
{
  "SentTextMessages": [
    {
      "MessageId": "xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx",
      "DestinationPhoneNumber": "+1234567890",
      "OriginationIdentity": "+0987654321",
      "MessageBody": "Hello!",
      "SentAt": "2025-01-01T00:00:00Z"
    }
  ]
}
```

## Development

```bash
# Run tests
make test

# Run integration tests
make test-integration

# Lint
make lint

# Build
make build
```

## Contributing

Contributions are welcome! Please see the issues for planned features and improvements.

## License

MIT License
