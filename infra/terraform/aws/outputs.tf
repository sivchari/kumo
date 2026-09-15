output "endpoint_url" {
  description = "AWS SDK endpoint URL for kumo."
  value       = "${local.endpoint_scheme}://${local.endpoint_host}${local.endpoint_port}"
}

output "alb_dns_name" {
  description = "ALB DNS name."
  value       = aws_lb.this.dns_name
}

output "alb_security_group_id" {
  description = "Security group id attached to the ALB."
  value       = aws_security_group.alb.id
}

output "ecs_cluster_name" {
  description = "ECS cluster name."
  value       = aws_ecs_cluster.this.name
}

output "ecs_service_name" {
  description = "ECS service name."
  value       = aws_ecs_service.kumo.name
}

output "efs_file_system_id" {
  description = "EFS file system id backing KUMO_DATA_DIR."
  value       = try(aws_efs_file_system.data[0].id, null)
}

output "task_execution_role_arn" {
  description = "ECS task execution role ARN."
  value       = aws_iam_role.task_execution.arn
}

output "task_role_arn" {
  description = "ECS task role ARN."
  value       = aws_iam_role.task.arn
}
