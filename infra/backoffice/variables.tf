variable "aws_region" {
  description = "AWS region"
  type        = string
  default     = "us-east-1"
}

variable "environment" {
  description = "Environment name"
  type        = string
  default     = "dev"
}

variable "project_name" {
  description = "Project name"
  type        = string
  default     = "tunnel"
}

variable "admin_api_key" {
  description = "Admin API key for backoffice authentication (stored as Lambda env var)"
  type        = string
  sensitive   = true
}

variable "cloudfront_distribution_id" {
  description = "CloudFront distribution ID of the main tunnel service (for monitoring)"
  type        = string
  default     = ""
}

variable "certificate_arn" {
  description = "ACM certificate ARN for the backoffice domain (must be in us-east-1 for CloudFront)"
  type        = string
  default     = ""
}

variable "domain_name" {
  description = "Base domain (e.g. tunnel.atelier.run) — backoffice served at admin.<domain>"
  type        = string
  default     = ""
}

variable "route53_zone_id" {
  description = "Route53 hosted zone ID"
  type        = string
  default     = ""
}
