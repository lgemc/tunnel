# REST API for control plane
resource "aws_apigatewayv2_api" "rest_api" {
  name          = "${var.project_name}-rest-api-${var.environment}"
  protocol_type = "HTTP"
  description   = "Tunnel service REST API for control plane operations"

  cors_configuration {
    allow_origins = ["*"]
    allow_methods = ["GET", "POST", "PUT", "DELETE", "OPTIONS"]
    allow_headers = ["*"]
    max_age       = 300
  }
}

resource "aws_apigatewayv2_stage" "rest_api" {
  api_id      = aws_apigatewayv2_api.rest_api.id
  name        = "$default"
  auto_deploy = true

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.rest_api.arn
    format = jsonencode({
      requestId      = "$context.requestId"
      ip             = "$context.identity.sourceIp"
      requestTime    = "$context.requestTime"
      httpMethod     = "$context.httpMethod"
      routeKey       = "$context.routeKey"
      status         = "$context.status"
      protocol       = "$context.protocol"
      responseLength = "$context.responseLength"
    })
  }

  depends_on = [aws_api_gateway_account.main]
}

resource "aws_cloudwatch_log_group" "rest_api" {
  name              = "/aws/apigateway/${var.project_name}-rest-${var.environment}"
  retention_in_days = 7
}

# WebSocket API for data plane
resource "aws_apigatewayv2_api" "websocket_api" {
  name                       = "${var.project_name}-websocket-api-${var.environment}"
  protocol_type              = "WEBSOCKET"
  route_selection_expression = "$request.body.action"
  description                = "Tunnel service WebSocket API for tunnel connections"
}

resource "aws_apigatewayv2_stage" "websocket_api" {
  api_id      = aws_apigatewayv2_api.websocket_api.id
  name        = var.environment
  auto_deploy = true

  access_log_settings {
    destination_arn = aws_cloudwatch_log_group.websocket_api.arn
    format = jsonencode({
      requestId      = "$context.requestId"
      ip             = "$context.identity.sourceIp"
      requestTime    = "$context.requestTime"
      routeKey       = "$context.routeKey"
      status         = "$context.status"
      protocol       = "$context.protocol"
      responseLength = "$context.responseLength"
      connectionId   = "$context.connectionId"
    })
  }

  default_route_settings {
    throttling_burst_limit = 5000
    throttling_rate_limit  = 10000
  }

  depends_on = [aws_api_gateway_account.main]
}

resource "aws_cloudwatch_log_group" "websocket_api" {
  name              = "/aws/apigateway/${var.project_name}-websocket-${var.environment}"
  retention_in_days = 7
}

# WebSocket routes
resource "aws_apigatewayv2_route" "connect" {
  api_id    = aws_apigatewayv2_api.websocket_api.id
  route_key = "$connect"
  target    = "integrations/${aws_apigatewayv2_integration.connect.id}"

  authorization_type = "CUSTOM"
  authorizer_id      = aws_apigatewayv2_authorizer.websocket.id
}

resource "aws_apigatewayv2_route" "disconnect" {
  api_id    = aws_apigatewayv2_api.websocket_api.id
  route_key = "$disconnect"
  target    = "integrations/${aws_apigatewayv2_integration.disconnect.id}"
}

resource "aws_apigatewayv2_route" "default" {
  api_id    = aws_apigatewayv2_api.websocket_api.id
  route_key = "$default"
  target    = "integrations/${aws_apigatewayv2_integration.default.id}"
}

# WebSocket integrations
resource "aws_apigatewayv2_integration" "connect" {
  api_id           = aws_apigatewayv2_api.websocket_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.tunnel_connect.invoke_arn
}

resource "aws_apigatewayv2_integration" "disconnect" {
  api_id           = aws_apigatewayv2_api.websocket_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.tunnel_disconnect.invoke_arn
}

resource "aws_apigatewayv2_integration" "default" {
  api_id           = aws_apigatewayv2_api.websocket_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.tunnel_proxy.invoke_arn
}

# WebSocket authorizer
resource "aws_apigatewayv2_authorizer" "websocket" {
  api_id           = aws_apigatewayv2_api.websocket_api.id
  authorizer_type  = "REQUEST"
  authorizer_uri   = aws_lambda_function.authorize_connection.invoke_arn
  identity_sources = ["route.request.header.Authorization"]
  name             = "websocket-authorizer"
}

# Lambda permissions for API Gateway
resource "aws_lambda_permission" "apigw_connect" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.tunnel_connect.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.websocket_api.execution_arn}/*/*"
}

resource "aws_lambda_permission" "apigw_disconnect" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.tunnel_disconnect.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.websocket_api.execution_arn}/*/*"
}

resource "aws_lambda_permission" "apigw_default" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.tunnel_proxy.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.websocket_api.execution_arn}/*/*"
}

resource "aws_lambda_permission" "apigw_authorizer" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.authorize_connection.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.websocket_api.execution_arn}/*/*"
}

# REST API routes and integrations
resource "aws_apigatewayv2_integration" "register_client" {
  api_id           = aws_apigatewayv2_api.rest_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.register_client.invoke_arn
}

resource "aws_apigatewayv2_route" "register_client" {
  api_id    = aws_apigatewayv2_api.rest_api.id
  route_key = "POST /clients"
  target    = "integrations/${aws_apigatewayv2_integration.register_client.id}"
}

resource "aws_lambda_permission" "rest_register_client" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.register_client.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.rest_api.execution_arn}/*/*"
}

resource "aws_apigatewayv2_integration" "create_tunnel" {
  api_id           = aws_apigatewayv2_api.rest_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.create_tunnel.invoke_arn
}

resource "aws_apigatewayv2_route" "create_tunnel" {
  api_id    = aws_apigatewayv2_api.rest_api.id
  route_key = "POST /tunnels"
  target    = "integrations/${aws_apigatewayv2_integration.create_tunnel.id}"
}

resource "aws_lambda_permission" "rest_create_tunnel" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.create_tunnel.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.rest_api.execution_arn}/*/*"
}

resource "aws_apigatewayv2_integration" "list_tunnels" {
  api_id           = aws_apigatewayv2_api.rest_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.list_tunnels.invoke_arn
}

resource "aws_apigatewayv2_route" "list_tunnels" {
  api_id    = aws_apigatewayv2_api.rest_api.id
  route_key = "GET /tunnels"
  target    = "integrations/${aws_apigatewayv2_integration.list_tunnels.id}"
}

resource "aws_lambda_permission" "rest_list_tunnels" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.list_tunnels.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.rest_api.execution_arn}/*/*"
}

resource "aws_apigatewayv2_integration" "delete_tunnel" {
  api_id           = aws_apigatewayv2_api.rest_api.id
  integration_type = "AWS_PROXY"
  integration_uri  = aws_lambda_function.delete_tunnel.invoke_arn
}

resource "aws_apigatewayv2_route" "delete_tunnel" {
  api_id    = aws_apigatewayv2_api.rest_api.id
  route_key = "DELETE /tunnels/{tunnel_id}"
  target    = "integrations/${aws_apigatewayv2_integration.delete_tunnel.id}"
}

resource "aws_lambda_permission" "rest_delete_tunnel" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.delete_tunnel.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.rest_api.execution_arn}/*/*"
}

# HTTP Proxy route for accessing tunnels
resource "aws_apigatewayv2_integration" "http_proxy" {
  api_id                 = aws_apigatewayv2_api.rest_api.id
  integration_type       = "AWS_PROXY"
  integration_uri        = aws_lambda_function.http_proxy.invoke_arn
  payload_format_version = "2.0"
  invoke_mode            = "RESPONSE_STREAM"
}

resource "aws_apigatewayv2_route" "http_proxy" {
  api_id    = aws_apigatewayv2_api.rest_api.id
  route_key = "ANY /t/{subdomain}/{proxy+}"
  target    = "integrations/${aws_apigatewayv2_integration.http_proxy.id}"
}

resource "aws_apigatewayv2_route" "http_proxy_root" {
  api_id    = aws_apigatewayv2_api.rest_api.id
  route_key = "ANY /t/{subdomain}"
  target    = "integrations/${aws_apigatewayv2_integration.http_proxy.id}"
}

resource "aws_lambda_permission" "rest_http_proxy" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.http_proxy.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_apigatewayv2_api.rest_api.execution_arn}/*/*"
}
