# HTTP API for the backoffice
resource "aws_apigatewayv2_api" "backoffice" {
  name          = "${local.name_prefix}-api"
  protocol_type = "HTTP"
  description   = "Backoffice admin API for tunnel service"
  tags          = local.common_tags

  cors_configuration {
    allow_origins = ["*"]
    allow_methods = ["GET", "OPTIONS"]
    allow_headers = ["Authorization", "Content-Type"]
    max_age       = 300
  }
}

resource "aws_cloudwatch_log_group" "backoffice_apigw" {
  name              = "/aws/apigateway/${local.name_prefix}"
  retention_in_days = 7
  tags              = local.common_tags
}

resource "aws_apigatewayv2_stage" "backoffice" {
  api_id      = aws_apigatewayv2_api.backoffice.id
  name        = "$default"
  auto_deploy = true
  tags        = local.common_tags

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.backoffice_apigw.arn
    format = jsonencode({
      requestId  = "$context.requestId"
      ip         = "$context.identity.sourceIp"
      method     = "$context.httpMethod"
      path       = "$context.path"
      status     = "$context.status"
      latency    = "$context.responseLatency"
    })
  }
}

# Lambda integration
resource "aws_apigatewayv2_integration" "backoffice_lambda" {
  api_id                 = aws_apigatewayv2_api.backoffice.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.backoffice_api.invoke_arn
  payload_format_version = "2.0"
}

# Catch-all route → Lambda
resource "aws_apigatewayv2_route" "backoffice_catchall" {
  api_id    = aws_apigatewayv2_api.backoffice.id
  route_key = "$default"
  target    = "integrations/${aws_apigatewayv2_integration.backoffice_lambda.id}"
}
