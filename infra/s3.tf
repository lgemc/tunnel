# S3 bucket used as a staging area for large request/response bodies that exceed
# the Lambda invocation payload limit (6 MB) or the DynamoDB item limit (400 KB).
# Objects are automatically deleted after 2 hours via lifecycle rule.

resource "aws_s3_bucket" "uploads" {
  bucket = "${var.project_name}-uploads-${var.environment}"
}

resource "aws_s3_bucket_lifecycle_configuration" "uploads" {
  bucket = aws_s3_bucket.uploads.id

  rule {
    id     = "expire-staged-objects"
    status = "Enabled"

    expiration {
      days = 1 # Minimum allowed by S3; objects are practically short-lived (TTL enforced by DynamoDB)
    }
  }
}

resource "aws_s3_bucket_public_access_block" "uploads" {
  bucket                  = aws_s3_bucket.uploads.id
  block_public_acls       = true
  block_public_policy     = true
  ignore_public_acls      = true
  restrict_public_buckets = true
}

output "uploads_bucket" {
  value       = aws_s3_bucket.uploads.bucket
  description = "S3 bucket name for staging large request/response bodies"
}
