# IAM role for the backoffice Lambda
resource "aws_iam_role" "backoffice_lambda" {
  name = "${local.name_prefix}-lambda-role"
  tags = local.common_tags

  assume_role_policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Action    = "sts:AssumeRole"
      Effect    = "Allow"
      Principal = { Service = "lambda.amazonaws.com" }
    }]
  })
}

# Policy: CloudWatch Logs (for the Lambda itself)
resource "aws_iam_role_policy_attachment" "basic_execution" {
  role       = aws_iam_role.backoffice_lambda.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

# Policy: read-only access to project resources
resource "aws_iam_role_policy" "backoffice_read" {
  name = "${local.name_prefix}-read-policy"
  role = aws_iam_role.backoffice_lambda.id

  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [
      # Lambda: list + get functions
      {
        Effect = "Allow"
        Action = [
          "lambda:ListFunctions",
          "lambda:GetFunction",
          "lambda:GetFunctionConfiguration",
          "lambda:ListAliases",
          "lambda:ListVersionsByFunction",
        ]
        Resource = "*"
      },
      # CloudWatch Logs: read project log groups
      {
        Effect = "Allow"
        Action = [
          "logs:DescribeLogGroups",
          "logs:DescribeLogStreams",
          "logs:GetLogEvents",
          "logs:FilterLogEvents",
          "logs:StartQuery",
          "logs:GetQueryResults",
        ]
        Resource = [
          "arn:aws:logs:${var.aws_region}:*:log-group:/aws/lambda/${var.project_name}-*",
          "arn:aws:logs:${var.aws_region}:*:log-group:/aws/lambda/${var.project_name}-*:*",
          "arn:aws:logs:${var.aws_region}:*:log-group:/aws/apigateway/${var.project_name}-*",
          "arn:aws:logs:${var.aws_region}:*:log-group:/aws/apigateway/${var.project_name}-*:*",
        ]
      },
      # DynamoDB: read project tables
      {
        Effect = "Allow"
        Action = [
          "dynamodb:DescribeTable",
          "dynamodb:Scan",
          "dynamodb:Query",
          "dynamodb:GetItem",
        ]
        Resource = [
          "arn:aws:dynamodb:${var.aws_region}:*:table/${var.project_name}-*-${var.environment}",
          "arn:aws:dynamodb:${var.aws_region}:*:table/${var.project_name}-*-${var.environment}/index/*",
        ]
      },
      # CloudFront: list distributions
      {
        Effect = "Allow"
        Action = [
          "cloudfront:ListDistributions",
          "cloudfront:GetDistribution",
          "cloudfront:ListTagsForResource",
        ]
        Resource = "*"
      },
    ]
  })
}
