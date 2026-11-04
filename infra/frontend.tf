# Frontend hosting: an S3 bucket serves the files as a static website (plain HTTP).
#
# CloudFront (HTTPS) was tried first, but AWS refuses to create CloudFront resources
# on new accounts until they are verified. With a verified account, put CloudFront in
# front of a private bucket instead: it is a small change and gives HTTPS.
#
# The bucket is public-read on purpose, so it must only ever hold the page's files.
# Those contain no secrets: config.js holds only the API address, which is useless
# without the API key.

data "aws_caller_identity" "current" {}

locals {
  frontend_files = ["index.html", "style.css", "app.js"]

  content_types = {
    html = "text/html; charset=utf-8"
    css  = "text/css; charset=utf-8"
    js   = "application/javascript; charset=utf-8"
  }
}

resource "aws_s3_bucket" "frontend" {
  # Bucket names are global, so the account number makes ours unique.
  bucket = "go-agent-frontend-${data.aws_caller_identity.current.account_id}"
}

# Allow a public bucket policy (S3 blocks this by default).
resource "aws_s3_bucket_public_access_block" "frontend" {
  bucket                  = aws_s3_bucket.frontend.id
  block_public_acls       = true
  ignore_public_acls      = true
  block_public_policy     = false
  restrict_public_buckets = false
}

resource "aws_s3_bucket_website_configuration" "frontend" {
  bucket = aws_s3_bucket.frontend.id

  index_document {
    suffix = "index.html"
  }
}

data "aws_iam_policy_document" "frontend_public_read" {
  statement {
    sid       = "PublicReadOfPageFiles"
    actions   = ["s3:GetObject"]
    resources = ["${aws_s3_bucket.frontend.arn}/*"]

    principals {
      type        = "*"
      identifiers = ["*"]
    }
  }
}

resource "aws_s3_bucket_policy" "frontend" {
  bucket = aws_s3_bucket.frontend.id
  policy = data.aws_iam_policy_document.frontend_public_read.json

  depends_on = [aws_s3_bucket_public_access_block.frontend]
}

# The page's files, uploaded from the frontend/ folder (mockup.html is not uploaded).
resource "aws_s3_object" "files" {
  for_each      = toset(local.frontend_files)
  bucket        = aws_s3_bucket.frontend.id
  key           = each.value
  source        = "${path.module}/../frontend/${each.value}"
  etag          = filemd5("${path.module}/../frontend/${each.value}")
  content_type  = local.content_types[regex("[^.]+$", each.value)]
  cache_control = "no-cache" # browsers re-check for edits instead of reusing old copies
}

# config.js is generated here so the page learns the API address without it being typed in.
resource "aws_s3_object" "config" {
  bucket        = aws_s3_bucket.frontend.id
  key           = "config.js"
  content       = "window.APP_CONFIG = ${jsonencode({ apiUrl = aws_api_gateway_stage.prod.invoke_url })};\n"
  content_type  = local.content_types["js"]
  cache_control = "no-cache"
}
