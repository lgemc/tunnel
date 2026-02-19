# Clients table
resource "aws_dynamodb_table" "clients" {
  name         = "${var.project_name}-clients-${var.environment}"
  billing_mode = var.dynamodb_billing_mode
  hash_key     = "client_id"

  attribute {
    name = "client_id"
    type = "S"
  }

  ttl {
    attribute_name = "ttl"
    enabled        = false
  }

  point_in_time_recovery {
    enabled = true
  }

  tags = {
    Name = "${var.project_name}-clients-${var.environment}"
  }
}

# Tunnels table
resource "aws_dynamodb_table" "tunnels" {
  name         = "${var.project_name}-tunnels-${var.environment}"
  billing_mode = var.dynamodb_billing_mode
  hash_key     = "tunnel_id"

  attribute {
    name = "tunnel_id"
    type = "S"
  }

  attribute {
    name = "client_id"
    type = "S"
  }

  global_secondary_index {
    name            = "client_id-index"
    hash_key        = "client_id"
    projection_type = "ALL"
  }

  ttl {
    attribute_name = "ttl"
    enabled        = false
  }

  point_in_time_recovery {
    enabled = true
  }

  tags = {
    Name = "${var.project_name}-tunnels-${var.environment}"
  }
}

# Domains table
resource "aws_dynamodb_table" "domains" {
  name         = "${var.project_name}-domains-${var.environment}"
  billing_mode = var.dynamodb_billing_mode
  hash_key     = "domain"

  attribute {
    name = "domain"
    type = "S"
  }

  ttl {
    attribute_name = "ttl"
    enabled        = false
  }

  point_in_time_recovery {
    enabled = true
  }

  tags = {
    Name = "${var.project_name}-domains-${var.environment}"
  }
}

# Pending HTTP requests table (for request/response cycle)
resource "aws_dynamodb_table" "pending_requests" {
  name         = "${var.project_name}-pending-requests-${var.environment}"
  billing_mode = var.dynamodb_billing_mode
  hash_key     = "request_id"

  attribute {
    name = "request_id"
    type = "S"
  }

  ttl {
    attribute_name = "ttl"
    enabled        = true
  }

  point_in_time_recovery {
    enabled = true
  }

  tags = {
    Name = "${var.project_name}-pending-requests-${var.environment}"
  }
}
