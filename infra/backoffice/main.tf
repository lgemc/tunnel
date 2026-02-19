terraform {
  required_version = ">= 1.0"

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
    archive = {
      source  = "hashicorp/archive"
      version = "~> 2.0"
    }
  }
}

provider "aws" {
  region = var.aws_region

  default_tags {
    tags = {
      Project     = "tunnel-service"
      Component   = "backoffice"
      Environment = var.environment
      ManagedBy   = "opentofu"
    }
  }
}

locals {
  name_prefix = "${var.project_name}-backoffice-${var.environment}"

  common_tags = {
    Project     = "tunnel-service"
    Component   = "backoffice"
    Environment = var.environment
    ManagedBy   = "opentofu"
  }
}
