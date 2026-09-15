locals {
  tags = merge(var.tags, {
    Project   = var.name
    Component = "kumo"
    ManagedBy = "terraform"
  })

  listener_protocol = var.certificate_arn == "" ? "HTTP" : "HTTPS"
  endpoint_scheme   = var.certificate_arn == "" ? "http" : "https"
  endpoint_host     = var.route53_zone_id != "" && var.record_name != "" ? var.record_name : aws_lb.this.dns_name
  endpoint_port     = contains([80, 443], var.listener_port) ? "" : ":${var.listener_port}"

  base_environment = [
    { name = "KUMO_HOST", value = "0.0.0.0" },
    { name = "KUMO_PORT", value = "4566" },
    { name = "KUMO_LOG_LEVEL", value = var.log_level }
  ]

  data_environment = var.enable_efs ? [
    { name = "KUMO_DATA_DIR", value = var.data_dir }
  ] : []

  optional_environment = concat(
    var.init_dir == "" ? [] : [{ name = "KUMO_INIT_DIR", value = var.init_dir }],
    var.enable_pprof ? [
      { name = "KUMO_PPROF", value = "true" },
      { name = "KUMO_PPROF_ADDR", value = var.pprof_addr }
    ] : []
  )

  extra_environment = [
    for key, value in var.extra_environment : {
      name  = key
      value = value
    }
  ]

  environment = concat(local.base_environment, local.data_environment, local.optional_environment, local.extra_environment)
}

resource "aws_ecs_cluster" "this" {
  name = var.name
  tags = local.tags

  setting {
    name  = "containerInsights"
    value = var.enable_container_insights ? "enabled" : "disabled"
  }
}

data "aws_iam_policy_document" "ecs_tasks_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["ecs-tasks.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "task_execution" {
  name               = "${var.name}-task-execution"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
  tags               = local.tags
}

resource "aws_iam_role_policy_attachment" "task_execution" {
  count      = var.attach_task_execution_policy ? 1 : 0
  role       = aws_iam_role.task_execution.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AmazonECSTaskExecutionRolePolicy"
}

resource "aws_iam_role" "task" {
  name               = "${var.name}-task"
  assume_role_policy = data.aws_iam_policy_document.ecs_tasks_assume.json
  tags               = local.tags
}

data "aws_iam_policy_document" "ecs_exec" {
  statement {
    actions = [
      "ssmmessages:CreateControlChannel",
      "ssmmessages:CreateDataChannel",
      "ssmmessages:OpenControlChannel",
      "ssmmessages:OpenDataChannel"
    ]
    resources = ["*"]
  }
}

resource "aws_iam_role_policy" "ecs_exec" {
  count  = var.enable_execute_command ? 1 : 0
  name   = "${var.name}-ecs-exec"
  role   = aws_iam_role.task.id
  policy = data.aws_iam_policy_document.ecs_exec.json
}

resource "aws_cloudwatch_log_group" "kumo" {
  name              = "/ecs/${var.name}/kumo"
  retention_in_days = var.log_retention_days
  tags              = local.tags
}

resource "aws_security_group" "alb" {
  name        = "${var.name}-alb"
  description = "kumo ALB"
  vpc_id      = var.vpc_id
  tags        = local.tags

  ingress {
    protocol    = "tcp"
    from_port   = var.listener_port
    to_port     = var.listener_port
    cidr_blocks = var.allowed_cidr_blocks
  }

  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "tasks" {
  name        = "${var.name}-tasks"
  description = "kumo ECS tasks"
  vpc_id      = var.vpc_id
  tags        = local.tags

  ingress {
    protocol        = "tcp"
    from_port       = 4566
    to_port         = 4566
    security_groups = [aws_security_group.alb.id]
  }

  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_security_group" "efs" {
  count       = var.enable_efs ? 1 : 0
  name        = "${var.name}-efs"
  description = "kumo EFS"
  vpc_id      = var.vpc_id
  tags        = local.tags

  ingress {
    protocol        = "tcp"
    from_port       = 2049
    to_port         = 2049
    security_groups = [aws_security_group.tasks.id]
  }

  egress {
    protocol    = "-1"
    from_port   = 0
    to_port     = 0
    cidr_blocks = ["0.0.0.0/0"]
  }
}

resource "aws_efs_file_system" "data" {
  count          = var.enable_efs ? 1 : 0
  creation_token = "${var.name}-data"
  encrypted      = true
  tags           = local.tags

  lifecycle_policy {
    transition_to_ia = var.efs_transition_to_ia
  }
}

resource "aws_efs_backup_policy" "data" {
  count          = var.enable_efs ? 1 : 0
  file_system_id = aws_efs_file_system.data[0].id

  backup_policy {
    status = var.efs_backup_policy_enabled ? "ENABLED" : "DISABLED"
  }
}

resource "aws_efs_mount_target" "data" {
  for_each        = var.enable_efs ? toset(var.private_subnet_ids) : toset([])
  file_system_id  = aws_efs_file_system.data[0].id
  subnet_id       = each.value
  security_groups = [aws_security_group.efs[0].id]
}

resource "aws_efs_access_point" "data" {
  count          = var.enable_efs ? 1 : 0
  file_system_id = aws_efs_file_system.data[0].id
  tags           = local.tags

  posix_user {
    uid = 10000
    gid = 10000
  }

  root_directory {
    path = "/kumo"

    creation_info {
      owner_uid   = 10000
      owner_gid   = 10000
      permissions = "0750"
    }
  }
}

resource "aws_lb" "this" {
  name               = var.name
  internal           = var.alb_internal
  load_balancer_type = "application"
  subnets            = var.alb_subnet_ids
  security_groups    = [aws_security_group.alb.id]
  tags               = local.tags
}

resource "aws_lb_target_group" "kumo" {
  name        = "${var.name}-kumo"
  port        = 4566
  protocol    = "HTTP"
  target_type = "ip"
  vpc_id      = var.vpc_id
  tags        = local.tags

  health_check {
    enabled             = true
    path                = "/health"
    matcher             = "200"
    interval            = 10
    timeout             = 5
    healthy_threshold   = 2
    unhealthy_threshold = 3
  }
}

resource "aws_lb_listener" "kumo" {
  load_balancer_arn = aws_lb.this.arn
  port              = var.listener_port
  protocol          = local.listener_protocol
  ssl_policy        = var.certificate_arn == "" ? null : var.ssl_policy
  certificate_arn   = var.certificate_arn == "" ? null : var.certificate_arn

  default_action {
    type             = "forward"
    target_group_arn = aws_lb_target_group.kumo.arn
  }
}

resource "aws_route53_record" "kumo" {
  count   = var.route53_zone_id != "" && var.record_name != "" ? 1 : 0
  zone_id = var.route53_zone_id
  name    = var.record_name
  type    = "A"

  alias {
    name                   = aws_lb.this.dns_name
    zone_id                = aws_lb.this.zone_id
    evaluate_target_health = true
  }
}

resource "aws_ecs_task_definition" "kumo" {
  family                   = var.name
  network_mode             = "awsvpc"
  requires_compatibilities = ["FARGATE"]
  cpu                      = tostring(var.cpu)
  memory                   = tostring(var.memory)
  execution_role_arn       = aws_iam_role.task_execution.arn
  task_role_arn            = aws_iam_role.task.arn

  ephemeral_storage {
    size_in_gib = var.ephemeral_storage_gib
  }

  dynamic "volume" {
    for_each = var.enable_efs ? [1] : []

    content {
      name = "data"

      efs_volume_configuration {
        file_system_id     = aws_efs_file_system.data[0].id
        transit_encryption = "ENABLED"

        authorization_config {
          access_point_id = aws_efs_access_point.data[0].id
          iam             = "DISABLED"
        }
      }
    }
  }

  container_definitions = jsonencode([
    {
      name                   = "kumo"
      image                  = var.image
      essential              = true
      user                   = "10000:10000"
      readonlyRootFilesystem = true
      command                = ["--host", "0.0.0.0", "--port", "4566"]
      portMappings = [
        {
          containerPort = 4566
          protocol      = "tcp"
        }
      ]
      environment = local.environment
      mountPoints = var.enable_efs ? [
        {
          sourceVolume  = "data"
          containerPath = var.data_dir
          readOnly      = false
        }
      ] : []
      linuxParameters = {
        initProcessEnabled = true
        capabilities = {
          drop = ["ALL"]
          add  = var.enable_ptrace ? ["SYS_PTRACE"] : []
        }
      }
      logConfiguration = {
        logDriver = "awslogs"
        options = {
          awslogs-group         = aws_cloudwatch_log_group.kumo.name
          awslogs-region        = var.region
          awslogs-stream-prefix = "kumo"
        }
      }
    }
  ])

  tags = local.tags
}

resource "aws_ecs_service" "kumo" {
  name                   = var.name
  cluster                = aws_ecs_cluster.this.id
  task_definition        = aws_ecs_task_definition.kumo.arn
  desired_count          = var.desired_count
  launch_type            = "FARGATE"
  enable_execute_command = var.enable_execute_command
  wait_for_steady_state  = var.wait_for_steady_state
  propagate_tags         = "SERVICE"
  tags                   = local.tags

  deployment_minimum_healthy_percent = 0
  deployment_maximum_percent         = 100

  network_configuration {
    subnets          = var.private_subnet_ids
    security_groups  = [aws_security_group.tasks.id]
    assign_public_ip = var.assign_public_ip
  }

  load_balancer {
    target_group_arn = aws_lb_target_group.kumo.arn
    container_name   = "kumo"
    container_port   = 4566
  }

  depends_on = [
    aws_lb_listener.kumo,
    aws_efs_mount_target.data
  ]
}
