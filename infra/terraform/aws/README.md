# sivchari/kumo AWS Terraform

This deploys `sivchari/kumo` as a single ECS/Fargate service behind an Application Load Balancer.
The task mounts EFS at `KUMO_DATA_DIR=/data` so emulator state survives task restarts.

Default posture:

- ECS/Fargate service running `ghcr.io/sivchari/kumo:latest`
- ALB listener on port `4566`
- internal ALB by default
- ALB ingress limited to RFC1918 CIDR blocks by default
- EFS file system, mount targets, and access point owned by UID/GID `10000`
- CloudWatch Logs retention of 14 days
- `/health` target-group health check

kumo has no built-in authentication. Keep `alb_internal=true` unless this is an isolated throwaway
environment, and prefer VPN, peering, or private DNS for clients.

## Plan

```sh
terraform -chdir=infra/terraform/aws init
terraform -chdir=infra/terraform/aws plan \
  -var='vpc_id=vpc-...' \
  -var='alb_subnet_ids=["subnet-private-a","subnet-private-c"]' \
  -var='private_subnet_ids=["subnet-private-a","subnet-private-c"]'
```

With DNS and TLS:

```sh
terraform -chdir=infra/terraform/aws plan \
  -var='vpc_id=vpc-...' \
  -var='alb_subnet_ids=["subnet-private-a","subnet-private-c"]' \
  -var='private_subnet_ids=["subnet-private-a","subnet-private-c"]' \
  -var='route53_zone_id=Z...' \
  -var='record_name=kumo.internal.example.com' \
  -var='listener_port=443' \
  -var='certificate_arn=arn:aws:acm:ap-northeast-1:123456789012:certificate/...'
```

The `endpoint_url` output can be used as `AWS_ENDPOINT_URL`:

```sh
export AWS_ENDPOINT_URL="$(terraform -chdir=infra/terraform/aws output -raw endpoint_url)"
export AWS_ACCESS_KEY_ID=test
export AWS_SECRET_ACCESS_KEY=test
export AWS_REGION=us-east-1
```

## Local kumo compatibility check

When applying this module to a local kumo endpoint rather than real AWS, disable the resources
that kumo does not emulate:

```sh
AWS_ENDPOINT_URL=http://127.0.0.1:4566 \
AWS_ACCESS_KEY_ID=test \
AWS_SECRET_ACCESS_KEY=test \
AWS_REGION=us-east-1 \
AWS_EC2_METADATA_DISABLED=true \
tofu -chdir=infra/terraform/aws plan -refresh=false \
  -var='vpc_id=vpc-kumo' \
  -var='alb_subnet_ids=["subnet-kumo-a","subnet-kumo-b"]' \
  -var='private_subnet_ids=["subnet-kumo-a","subnet-kumo-b"]' \
  -var='enable_efs=false' \
  -var='attach_task_execution_policy=false'
```

## Operations

- Keep `desired_count=1` while using shared EFS persistence. kumo persists service state as JSON
  files on shutdown, so multiple writers can overwrite each other.
- Private ECS tasks need NAT or an ECR mirror to pull `ghcr.io/sivchari/kumo:latest`.
- Set `image` to an ECR image digest for reproducible production-like environments.
- Set `enable_pprof=true` to expose the optional pprof listener inside the task. Do not route it
  through the ALB unless you add separate network controls.
- Set `enable_execute_command=true` only for break-glass debugging.
