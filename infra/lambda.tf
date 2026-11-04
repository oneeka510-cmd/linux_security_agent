# Package the Python files, give each Lambda only the permissions it needs, and deploy them.

data "archive_file" "ingest" {
  type        = "zip"
  source_file = "${path.module}/../backend/ingest.py"
  output_path = "${path.module}/build/ingest.zip"
}

data "archive_file" "read" {
  type        = "zip"
  source_file = "${path.module}/../backend/read.py"
  output_path = "${path.module}/build/read.zip"
}

# A role is "who the function runs as". This policy lets AWS Lambda use the role.
data "aws_iam_policy_document" "lambda_assume" {
  statement {
    actions = ["sts:AssumeRole"]
    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

locals {
  table_arns = [
    aws_dynamodb_table.hosts.arn,
    aws_dynamodb_table.packages.arn,
    aws_dynamodb_table.cis_results.arn,
  ]

  lambda_environment = {
    HOSTS_TABLE    = aws_dynamodb_table.hosts.name
    PACKAGES_TABLE = aws_dynamodb_table.packages.name
    CIS_TABLE      = aws_dynamodb_table.cis_results.name
  }
}

# ---------- ingest: may only WRITE to the three tables ----------

resource "aws_iam_role" "ingest" {
  name               = "endpoint-agent-ingest-role"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

resource "aws_iam_role_policy_attachment" "ingest_logs" {
  role       = aws_iam_role.ingest.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "ingest_dynamodb" {
  name = "write-tables"
  role = aws_iam_role.ingest.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["dynamodb:PutItem"]
      Resource = local.table_arns
    }]
  })
}

resource "aws_lambda_function" "ingest" {
  function_name    = "endpoint-agent-ingest"
  role             = aws_iam_role.ingest.arn
  runtime          = "python3.12"
  handler          = "ingest.handler"
  filename         = data.archive_file.ingest.output_path
  source_code_hash = data.archive_file.ingest.output_base64sha256
  timeout          = 30
  memory_size      = 256

  environment {
    variables = local.lambda_environment
  }
}

# ---------- read: may only READ from the three tables ----------

resource "aws_iam_role" "read" {
  name               = "endpoint-agent-read-role"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume.json
}

resource "aws_iam_role_policy_attachment" "read_logs" {
  role       = aws_iam_role.read.name
  policy_arn = "arn:aws:iam::aws:policy/service-role/AWSLambdaBasicExecutionRole"
}

resource "aws_iam_role_policy" "read_dynamodb" {
  name = "read-tables"
  role = aws_iam_role.read.id
  policy = jsonencode({
    Version = "2012-10-17"
    Statement = [{
      Effect   = "Allow"
      Action   = ["dynamodb:GetItem", "dynamodb:Scan"]
      Resource = local.table_arns
    }]
  })
}

resource "aws_lambda_function" "read" {
  function_name    = "endpoint-agent-read"
  role             = aws_iam_role.read.arn
  runtime          = "python3.12"
  handler          = "read.handler"
  filename         = data.archive_file.read.output_path
  source_code_hash = data.archive_file.read.output_base64sha256
  timeout          = 30
  memory_size      = 256

  environment {
    variables = local.lambda_environment
  }
}
