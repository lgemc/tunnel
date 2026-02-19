# IAM role for Lambda functions
resource "aws_iam_role" "lambda_execution" {
  name = "${var.project_name}-lambda-execution-${var.environment}"

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Action = "sts:AssumeRole"
        Effect = "Allow"
        Principal = {
          Service = "lambda.amazonaws.com"
        }
      }
    ]
  })
}

# IAM policy for Lambda functions
resource "aws_iam_role_policy" "lambda_policy" {
  name = "${var.project_name}-lambda-policy-${var.environment}"
  role = aws_iam_role.lambda_execution.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      {
        Effect = "Allow"
        Action = [
          "logs:CreateLogGroup",
          "logs:CreateLogStream",
          "logs:PutLogEvents"
        ]
        Resource = "arn:aws:logs:*:*:*"
      },
      {
        Effect = "Allow"
        Action = [
          "dynamodb:GetItem",
          "dynamodb:PutItem",
          "dynamodb:UpdateItem",
          "dynamodb:DeleteItem",
          "dynamodb:Query",
          "dynamodb:Scan"
        ]
        Resource = [
          aws_dynamodb_table.clients.arn,
          aws_dynamodb_table.tunnels.arn,
          aws_dynamodb_table.domains.arn,
          aws_dynamodb_table.pending_requests.arn,
          "${aws_dynamodb_table.tunnels.arn}/index/*"
        ]
      },
      {
        Effect = "Allow"
        Action = [
          "execute-api:ManageConnections"
        ]
        Resource = "${aws_apigatewayv2_api.websocket_api.execution_arn}/*"
      },
      {
        # Allow all Lambdas to read/write staged request & response bodies in S3.
        Effect = "Allow"
        Action = [
          "s3:PutObject",
          "s3:GetObject",
          "s3:DeleteObject"
        ]
        Resource = "${aws_s3_bucket.uploads.arn}/*"
      },
    ]
  })
}

# CloudWatch log groups for Lambda functions
resource "aws_cloudwatch_log_group" "register_client" {
  name              = "/aws/lambda/${aws_lambda_function.register_client.function_name}"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "create_tunnel" {
  name              = "/aws/lambda/${aws_lambda_function.create_tunnel.function_name}"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "delete_tunnel" {
  name              = "/aws/lambda/${aws_lambda_function.delete_tunnel.function_name}"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "list_tunnels" {
  name              = "/aws/lambda/${aws_lambda_function.list_tunnels.function_name}"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "authorize_connection" {
  name              = "/aws/lambda/${aws_lambda_function.authorize_connection.function_name}"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "tunnel_connect" {
  name              = "/aws/lambda/${aws_lambda_function.tunnel_connect.function_name}"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "tunnel_disconnect" {
  name              = "/aws/lambda/${aws_lambda_function.tunnel_disconnect.function_name}"
  retention_in_days = 7
}

resource "aws_cloudwatch_log_group" "tunnel_proxy" {
  name              = "/aws/lambda/${aws_lambda_function.tunnel_proxy.function_name}"
  retention_in_days = 7
}

# Lambda functions - Control Plane
resource "aws_lambda_function" "register_client" {
  function_name = "${var.project_name}-register-client-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = var.lambda_timeout
  memory_size   = var.lambda_memory_size

  filename         = data.archive_file.register_client_placeholder.output_path
  source_code_hash = data.archive_file.register_client_placeholder.output_base64sha256

  environment {
    variables = {
      CLIENTS_TABLE = aws_dynamodb_table.clients.name
      ENVIRONMENT   = var.environment
    }
  }
}

resource "aws_lambda_function" "create_tunnel" {
  function_name = "${var.project_name}-create-tunnel-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = var.lambda_timeout
  memory_size   = var.lambda_memory_size

  filename         = data.archive_file.create_tunnel_placeholder.output_path
  source_code_hash = data.archive_file.create_tunnel_placeholder.output_base64sha256

  environment {
    variables = {
      CLIENTS_TABLE        = aws_dynamodb_table.clients.name
      TUNNELS_TABLE        = aws_dynamodb_table.tunnels.name
      DOMAINS_TABLE        = aws_dynamodb_table.domains.name
      DOMAIN_NAME          = var.domain_name
      WEBSOCKET_API_URL    = aws_apigatewayv2_api.websocket_api.api_endpoint
      WEBSOCKET_API_STAGE  = aws_apigatewayv2_stage.websocket_api.name
      ENVIRONMENT          = var.environment
    }
  }
}

resource "aws_lambda_function" "delete_tunnel" {
  function_name = "${var.project_name}-delete-tunnel-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = var.lambda_timeout
  memory_size   = var.lambda_memory_size

  filename         = data.archive_file.delete_tunnel_placeholder.output_path
  source_code_hash = data.archive_file.delete_tunnel_placeholder.output_base64sha256

  environment {
    variables = {
      CLIENTS_TABLE = aws_dynamodb_table.clients.name
      TUNNELS_TABLE = aws_dynamodb_table.tunnels.name
      DOMAINS_TABLE = aws_dynamodb_table.domains.name
      ENVIRONMENT   = var.environment
    }
  }
}

resource "aws_lambda_function" "list_tunnels" {
  function_name = "${var.project_name}-list-tunnels-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = var.lambda_timeout
  memory_size   = var.lambda_memory_size

  filename         = data.archive_file.list_tunnels_placeholder.output_path
  source_code_hash = data.archive_file.list_tunnels_placeholder.output_base64sha256

  environment {
    variables = {
      CLIENTS_TABLE = aws_dynamodb_table.clients.name
      TUNNELS_TABLE = aws_dynamodb_table.tunnels.name
      ENVIRONMENT   = var.environment
    }
  }
}

resource "aws_lambda_function" "authorize_connection" {
  function_name = "${var.project_name}-authorize-connection-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = var.lambda_timeout
  memory_size   = var.lambda_memory_size

  filename         = data.archive_file.authorize_connection_placeholder.output_path
  source_code_hash = data.archive_file.authorize_connection_placeholder.output_base64sha256

  environment {
    variables = {
      CLIENTS_TABLE = aws_dynamodb_table.clients.name
      ENVIRONMENT   = var.environment
    }
  }
}

# HTTP Proxy Lambda
resource "aws_lambda_function" "http_proxy" {
  function_name = "${var.project_name}-http-proxy-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = 180
  memory_size   = var.lambda_memory_size

  filename         = data.archive_file.http_proxy_placeholder.output_path
  source_code_hash = data.archive_file.http_proxy_placeholder.output_base64sha256

  environment {
    variables = {
      DOMAINS_TABLE          = aws_dynamodb_table.domains.name
      TUNNELS_TABLE          = aws_dynamodb_table.tunnels.name
      PENDING_REQUESTS_TABLE = aws_dynamodb_table.pending_requests.name
      WEBSOCKET_ENDPOINT     = "${replace(aws_apigatewayv2_api.websocket_api.api_endpoint, "wss://", "https://")}/${aws_apigatewayv2_stage.websocket_api.name}"
      DOMAIN_NAME            = var.domain_name
      UPLOADS_BUCKET         = aws_s3_bucket.uploads.bucket
      ENVIRONMENT            = var.environment
    }
  }
}

resource "aws_cloudwatch_log_group" "http_proxy" {
  name              = "/aws/lambda/${aws_lambda_function.http_proxy.function_name}"
  retention_in_days = 7
}

# Lambda functions - Data Plane
resource "aws_lambda_function" "tunnel_connect" {
  function_name = "${var.project_name}-tunnel-connect-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = var.lambda_timeout
  memory_size   = var.lambda_memory_size

  filename         = data.archive_file.tunnel_connect_placeholder.output_path
  source_code_hash = data.archive_file.tunnel_connect_placeholder.output_base64sha256

  environment {
    variables = {
      TUNNELS_TABLE = aws_dynamodb_table.tunnels.name
      ENVIRONMENT   = var.environment
    }
  }
}

resource "aws_lambda_function" "tunnel_disconnect" {
  function_name = "${var.project_name}-tunnel-disconnect-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = var.lambda_timeout
  memory_size   = var.lambda_memory_size

  filename         = data.archive_file.tunnel_disconnect_placeholder.output_path
  source_code_hash = data.archive_file.tunnel_disconnect_placeholder.output_base64sha256

  environment {
    variables = {
      TUNNELS_TABLE = aws_dynamodb_table.tunnels.name
      DOMAINS_TABLE = aws_dynamodb_table.domains.name
      ENVIRONMENT   = var.environment
    }
  }
}

resource "aws_lambda_function" "tunnel_proxy" {
  function_name = "${var.project_name}-tunnel-proxy-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = 300 # Longer timeout for proxying
  memory_size   = 512 # More memory for handling requests

  filename         = data.archive_file.tunnel_proxy_placeholder.output_path
  source_code_hash = data.archive_file.tunnel_proxy_placeholder.output_base64sha256

  environment {
    variables = {
      TUNNELS_TABLE          = aws_dynamodb_table.tunnels.name
      DOMAINS_TABLE          = aws_dynamodb_table.domains.name
      PENDING_REQUESTS_TABLE = aws_dynamodb_table.pending_requests.name
      WEBSOCKET_ENDPOINT     = "${replace(aws_apigatewayv2_api.websocket_api.api_endpoint, "wss://", "https://")}/${aws_apigatewayv2_stage.websocket_api.name}"
      ENVIRONMENT            = var.environment
    }
  }
}

# Placeholder Lambda deployment packages (will be replaced with actual code)
data "archive_file" "register_client_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/register-client.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}

data "archive_file" "create_tunnel_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/create-tunnel.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}

data "archive_file" "delete_tunnel_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/delete-tunnel.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}

data "archive_file" "list_tunnels_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/list-tunnels.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}

data "archive_file" "authorize_connection_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/authorize-connection.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}

data "archive_file" "tunnel_connect_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/tunnel-connect.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}

data "archive_file" "tunnel_disconnect_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/tunnel-disconnect.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}

data "archive_file" "tunnel_proxy_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/tunnel-proxy.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}

data "archive_file" "http_proxy_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/http-proxy.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}

# Lambda Function URL for http-proxy with response streaming enabled.
# authorization_type = NONE lets CloudFront forward all viewer headers (including Authorization)
# without stripping them for OAC signing — required for transparent proxy behaviour.
resource "aws_lambda_function_url" "http_proxy" {
  function_name      = aws_lambda_function.http_proxy.function_name
  authorization_type = "NONE"
  invoke_mode        = "RESPONSE_STREAM"
}

# Required since Oct 2025: new function URLs need both InvokeFunctionUrl AND InvokeFunction
resource "aws_lambda_permission" "function_url_public_invoke" {
  statement_id  = "FunctionURLAllowPublicInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.http_proxy.function_name
  principal     = "*"
}

output "http_proxy_function_url" {
  value       = aws_lambda_function_url.http_proxy.function_url
  description = "Lambda Function URL for http-proxy (streaming)"
}

# ── s3-upload-notify Lambda ──────────────────────────────────────────────────
# Triggered by S3 ObjectCreated events on the uploads bucket.
# Reads the request metadata stored as S3 object tags, looks up the active
# tunnel connection in DynamoDB, and forwards the proxy request to the CLI via
# WebSocket so the CLI can download the body from S3 and forward it to localhost.

resource "aws_lambda_function" "s3_upload_notify" {
  function_name = "${var.project_name}-s3-upload-notify-${var.environment}"
  role          = aws_iam_role.lambda_execution.arn
  handler       = "bootstrap"
  runtime       = "provided.al2023"
  timeout       = 180 # must wait for CLI response
  memory_size   = var.lambda_memory_size

  filename         = data.archive_file.s3_upload_notify_placeholder.output_path
  source_code_hash = data.archive_file.s3_upload_notify_placeholder.output_base64sha256

  environment {
    variables = {
      DOMAINS_TABLE          = aws_dynamodb_table.domains.name
      TUNNELS_TABLE          = aws_dynamodb_table.tunnels.name
      PENDING_REQUESTS_TABLE = aws_dynamodb_table.pending_requests.name
      WEBSOCKET_ENDPOINT     = "${replace(aws_apigatewayv2_api.websocket_api.api_endpoint, "wss://", "https://")}/${aws_apigatewayv2_stage.websocket_api.name}"
      UPLOADS_BUCKET         = aws_s3_bucket.uploads.bucket
      ENVIRONMENT            = var.environment
    }
  }
}

resource "aws_cloudwatch_log_group" "s3_upload_notify" {
  name              = "/aws/lambda/${aws_lambda_function.s3_upload_notify.function_name}"
  retention_in_days = 7
}

# Allow S3 to invoke the s3-upload-notify Lambda
resource "aws_lambda_permission" "s3_upload_notify" {
  statement_id  = "AllowS3Invoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.s3_upload_notify.function_name
  principal     = "s3.amazonaws.com"
  source_arn    = aws_s3_bucket.uploads.arn
}

# S3 → Lambda event notification: fire on every new object under requests/
resource "aws_s3_bucket_notification" "upload_notify" {
  bucket = aws_s3_bucket.uploads.id

  lambda_function {
    lambda_function_arn = aws_lambda_function.s3_upload_notify.arn
    events              = ["s3:ObjectCreated:*"]
    filter_prefix       = "requests/"
  }

  depends_on = [aws_lambda_permission.s3_upload_notify]
}

data "archive_file" "s3_upload_notify_placeholder" {
  type        = "zip"
  output_path = "${path.module}/.terraform/lambda-placeholders/s3-upload-notify.zip"

  source {
    content  = "placeholder"
    filename = "bootstrap"
  }
}
