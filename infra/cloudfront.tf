# CloudFront Function to rewrite URLs based on Host header
resource "aws_cloudfront_function" "tunnel_url_rewrite" {
  count   = var.enable_cloudfront ? 1 : 0
  name    = "${var.project_name}-url-rewrite-${var.environment}"
  runtime = "cloudfront-js-2.0"
  comment = "Rewrite subdomain requests to /t/{subdomain}/{path} for the REST API"
  publish = true

  code = <<-EOT
    function handler(event) {
      var request = event.request;
      var host = request.headers.host.value;
      var subdomain = host.split('.')[0];
      var uri = request.uri;
      if (uri === '/' || uri === '') {
        request.uri = '/t/' + subdomain;
      } else {
        request.uri = '/t/' + subdomain + uri;
      }
      return request;
    }
  EOT
}

# CloudFront distribution for tunnel traffic
resource "aws_cloudfront_distribution" "tunnel" {
  count   = var.enable_cloudfront ? 1 : 0
  enabled = true
  comment = "Tunnel service distribution for ${var.environment}"

  aliases = var.enable_cloudfront ? ["*.${var.domain_name}"] : []

  origin {
    domain_name = replace(aws_apigatewayv2_api.rest_api.api_endpoint, "https://", "")
    origin_id   = "rest-api-origin"

    custom_origin_config {
      http_port                = 80
      https_port               = 443
      origin_protocol_policy   = "https-only"
      origin_ssl_protocols     = ["TLSv1.2"]
      origin_read_timeout      = 60
      origin_keepalive_timeout = 60
    }
  }

  default_cache_behavior {
    allowed_methods  = ["DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT"]
    cached_methods   = ["GET", "HEAD"]
    target_origin_id = "rest-api-origin"

    cache_policy_id          = "4135ea2d-6df8-44a3-9df3-4b5a84be39ad" # CachingDisabled
    origin_request_policy_id = "b689b0a8-53d0-40ab-baf2-68738e2966ac" # AllViewerExceptHostHeader

    function_association {
      event_type   = "viewer-request"
      function_arn = aws_cloudfront_function.tunnel_url_rewrite[0].arn
    }

    viewer_protocol_policy = "redirect-to-https"
    min_ttl                = 0
    default_ttl            = 0
    max_ttl                = 0
    compress               = true
  }

  restrictions {
    geo_restriction {
      restriction_type = "none"
    }
  }

  viewer_certificate {
    acm_certificate_arn            = local.certificate_arn
    ssl_support_method             = "sni-only"
    minimum_protocol_version       = "TLSv1.2_2021"
    cloudfront_default_certificate = false
  }

  tags = {
    Name = "${var.project_name}-distribution-${var.environment}"
  }
}

# Route53 record for wildcard subdomain
resource "aws_route53_record" "tunnel_wildcard" {
  count   = var.enable_cloudfront && var.route53_zone_id != "" ? 1 : 0
  zone_id = var.route53_zone_id
  name    = "*.${var.domain_name}"
  type    = "A"

  alias {
    name                   = aws_cloudfront_distribution.tunnel[0].domain_name
    zone_id                = aws_cloudfront_distribution.tunnel[0].hosted_zone_id
    evaluate_target_health = false
  }
}
