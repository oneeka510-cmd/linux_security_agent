# Three tables, one per API endpoint. Each is keyed by machine_id (one item per host).
# PAY_PER_REQUEST means no capacity to manage and no cost while idle.

resource "aws_dynamodb_table" "hosts" {
  name         = "endpoint-agent-hosts"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "machine_id"

  attribute {
    name = "machine_id"
    type = "S"
  }
}

resource "aws_dynamodb_table" "packages" {
  name         = "endpoint-agent-packages"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "machine_id"

  attribute {
    name = "machine_id"
    type = "S"
  }
}

resource "aws_dynamodb_table" "cis_results" {
  name         = "endpoint-agent-cis-results"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "machine_id"

  attribute {
    name = "machine_id"
    type = "S"
  }
}
