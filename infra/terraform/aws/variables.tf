variable "name" {
  description = "Name prefix for kumo AWS resources."
  type        = string
  default     = "kumo"

  validation {
    condition     = can(regex("^[a-zA-Z0-9-]+$", var.name)) && length(var.name) <= 26
    error_message = "name must contain only letters, digits, and hyphens, and be 26 characters or fewer."
  }
}

variable "region" {
  description = "AWS region."
  type        = string
  default     = "ap-northeast-1"
}

variable "vpc_id" {
  description = "VPC id that contains the ALB and ECS subnets."
  type        = string
}

variable "alb_subnet_ids" {
  description = "Subnet ids for the ALB. Use private subnets when alb_internal is true."
  type        = list(string)
}

variable "private_subnet_ids" {
  description = "Private subnet ids for ECS tasks and EFS mount targets."
  type        = list(string)
}

variable "image" {
  description = "Container image for kumo."
  type        = string
  default     = "ghcr.io/sivchari/kumo:latest"
}

variable "desired_count" {
  description = "Desired ECS task count. Keep this at 1 when KUMO_DATA_DIR persists to EFS."
  type        = number
  default     = 1

  validation {
    condition     = var.desired_count >= 1
    error_message = "desired_count must be at least 1."
  }
}

variable "cpu" {
  description = "Fargate task CPU units."
  type        = number
  default     = 512
}

variable "memory" {
  description = "Fargate task memory in MiB."
  type        = number
  default     = 1024
}

variable "ephemeral_storage_gib" {
  description = "Fargate ephemeral storage size in GiB."
  type        = number
  default     = 21

  validation {
    condition     = var.ephemeral_storage_gib >= 21 && var.ephemeral_storage_gib <= 200
    error_message = "ephemeral_storage_gib must be between 21 and 200."
  }
}

variable "listener_port" {
  description = "ALB listener port exposed for AWS SDK endpoint URLs."
  type        = number
  default     = 4566

  validation {
    condition     = var.listener_port > 0 && var.listener_port <= 65535
    error_message = "listener_port must be a valid TCP port."
  }
}

variable "certificate_arn" {
  description = "Optional ACM certificate ARN. When set, the ALB listener uses HTTPS."
  type        = string
  default     = ""
}

variable "ssl_policy" {
  description = "ALB SSL policy used when certificate_arn is set."
  type        = string
  default     = "ELBSecurityPolicy-TLS13-1-2-2021-06"
}

variable "alb_internal" {
  description = "Whether the ALB is internal. kumo has no built-in auth, so the safe default is internal."
  type        = bool
  default     = true
}

variable "allowed_cidr_blocks" {
  description = "CIDR blocks allowed to reach the ALB listener."
  type        = list(string)
  default     = ["10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16"]
}

variable "assign_public_ip" {
  description = "Assign public IPs to ECS tasks. Prefer false with private subnets and NAT or an ECR mirror."
  type        = bool
  default     = false
}

variable "route53_zone_id" {
  description = "Optional Route53 hosted zone id for an alias record."
  type        = string
  default     = ""
}

variable "record_name" {
  description = "Optional Route53 record name, for example kumo.internal.example.com."
  type        = string
  default     = ""
}

variable "log_level" {
  description = "KUMO_LOG_LEVEL value."
  type        = string
  default     = "info"

  validation {
    condition     = contains(["debug", "info", "warn", "error"], var.log_level)
    error_message = "log_level must be debug, info, warn, or error."
  }
}

variable "data_dir" {
  description = "KUMO_DATA_DIR value mounted from EFS."
  type        = string
  default     = "/data"

  validation {
    condition     = can(regex("^/[A-Za-z0-9._/-]+$", var.data_dir))
    error_message = "data_dir must be an absolute container path."
  }
}

variable "enable_efs" {
  description = "Create and mount EFS for persistent KUMO_DATA_DIR. Disable for local kumo compatibility tests."
  type        = bool
  default     = true
}

variable "init_dir" {
  description = "Optional KUMO_INIT_DIR value. Mount/init contents must be provided by the image or another volume."
  type        = string
  default     = ""
}

variable "enable_pprof" {
  description = "Enable kumo's optional pprof endpoint through KUMO_PPROF."
  type        = bool
  default     = false
}

variable "pprof_addr" {
  description = "KUMO_PPROF_ADDR value when enable_pprof is true."
  type        = string
  default     = ":6060"
}

variable "enable_ptrace" {
  description = "Add SYS_PTRACE to the container. Keep false unless profiling tooling requires it."
  type        = bool
  default     = false
}

variable "extra_environment" {
  description = "Additional plain-text environment variables for the kumo container."
  type        = map(string)
  default     = {}
}

variable "log_retention_days" {
  description = "CloudWatch Logs retention in days."
  type        = number
  default     = 14
}

variable "enable_container_insights" {
  description = "Enable ECS container insights."
  type        = bool
  default     = true
}

variable "enable_execute_command" {
  description = "Enable ECS Exec for the kumo service."
  type        = bool
  default     = false
}

variable "attach_task_execution_policy" {
  description = "Attach the AWS-managed ECS task execution policy. Disable when planning/applying against emulators that do not seed AWS-managed IAM policies."
  type        = bool
  default     = true
}

variable "wait_for_steady_state" {
  description = "Whether Terraform waits for the ECS service to reach steady state."
  type        = bool
  default     = false
}

variable "efs_transition_to_ia" {
  description = "EFS lifecycle policy transition_to_ia value."
  type        = string
  default     = "AFTER_30_DAYS"
}

variable "efs_backup_policy_enabled" {
  description = "Enable automatic AWS Backup backups for the EFS file system."
  type        = bool
  default     = true
}

variable "tags" {
  description = "Additional tags."
  type        = map(string)
  default     = {}
}
