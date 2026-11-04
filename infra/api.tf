# API Gateway (REST API): the public front door.
#   POST /ingest      -> ingest Lambda   (agent sends its report)
#   GET  /hosts       -> read Lambda
#   GET  /apps        -> read Lambda
#   GET  /cis-results -> read Lambda
# Every route needs an API key. OPTIONS routes (browser CORS preflight) are open,
# because browsers send them without the key.

locals {
  routes = {
    ingest      = { path = "ingest", method = "POST", lambda = aws_lambda_function.ingest }
    hosts       = { path = "hosts", method = "GET", lambda = aws_lambda_function.read }
    apps        = { path = "apps", method = "GET", lambda = aws_lambda_function.read }
    cis_results = { path = "cis-results", method = "GET", lambda = aws_lambda_function.read }
  }
}

resource "aws_api_gateway_rest_api" "api" {
  name = "endpoint-agent-api"

  endpoint_configuration {
    types = ["REGIONAL"]
  }
}

resource "aws_api_gateway_resource" "route" {
  for_each    = local.routes
  rest_api_id = aws_api_gateway_rest_api.api.id
  parent_id   = aws_api_gateway_rest_api.api.root_resource_id
  path_part   = each.value.path
}

# ---------- the real methods, wired to Lambda ----------

resource "aws_api_gateway_method" "route" {
  for_each         = local.routes
  rest_api_id      = aws_api_gateway_rest_api.api.id
  resource_id      = aws_api_gateway_resource.route[each.key].id
  http_method      = each.value.method
  authorization    = "NONE"
  api_key_required = true
}

resource "aws_api_gateway_integration" "route" {
  for_each                = local.routes
  rest_api_id             = aws_api_gateway_rest_api.api.id
  resource_id             = aws_api_gateway_resource.route[each.key].id
  http_method             = aws_api_gateway_method.route[each.key].http_method
  integration_http_method = "POST" # how API Gateway calls Lambda, always POST
  type                    = "AWS_PROXY"
  uri                     = each.value.lambda.invoke_arn
}

resource "aws_lambda_permission" "ingest" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.ingest.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.api.execution_arn}/*/*"
}

resource "aws_lambda_permission" "read" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.read.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.api.execution_arn}/*/*"
}

# ---------- CORS preflight (OPTIONS), answered by API Gateway itself ----------

resource "aws_api_gateway_method" "options" {
  for_each         = local.routes
  rest_api_id      = aws_api_gateway_rest_api.api.id
  resource_id      = aws_api_gateway_resource.route[each.key].id
  http_method      = "OPTIONS"
  authorization    = "NONE"
  api_key_required = false
}

resource "aws_api_gateway_integration" "options" {
  for_each    = local.routes
  rest_api_id = aws_api_gateway_rest_api.api.id
  resource_id = aws_api_gateway_resource.route[each.key].id
  http_method = aws_api_gateway_method.options[each.key].http_method
  type        = "MOCK"

  request_templates = {
    "application/json" = jsonencode({ statusCode = 200 })
  }
}

resource "aws_api_gateway_method_response" "options" {
  for_each    = local.routes
  rest_api_id = aws_api_gateway_rest_api.api.id
  resource_id = aws_api_gateway_resource.route[each.key].id
  http_method = aws_api_gateway_method.options[each.key].http_method
  status_code = "200"

  response_parameters = {
    "method.response.header.Access-Control-Allow-Headers" = true
    "method.response.header.Access-Control-Allow-Methods" = true
    "method.response.header.Access-Control-Allow-Origin"  = true
  }
}

resource "aws_api_gateway_integration_response" "options" {
  for_each    = local.routes
  rest_api_id = aws_api_gateway_rest_api.api.id
  resource_id = aws_api_gateway_resource.route[each.key].id
  http_method = aws_api_gateway_method.options[each.key].http_method
  status_code = aws_api_gateway_method_response.options[each.key].status_code

  response_parameters = {
    "method.response.header.Access-Control-Allow-Headers" = "'Content-Type,x-api-key'"
    "method.response.header.Access-Control-Allow-Methods" = "'GET,POST,OPTIONS'"
    "method.response.header.Access-Control-Allow-Origin"  = "'*'"
  }

  depends_on = [aws_api_gateway_integration.options]
}

# ---------- deployment, stage, API key ----------

resource "aws_api_gateway_deployment" "api" {
  rest_api_id = aws_api_gateway_rest_api.api.id

  # Redeploy whenever any route definition changes.
  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_resource.route,
      aws_api_gateway_method.route,
      aws_api_gateway_integration.route,
      aws_api_gateway_method.options,
      aws_api_gateway_integration.options,
      aws_api_gateway_integration_response.options,
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_api_gateway_stage" "prod" {
  rest_api_id   = aws_api_gateway_rest_api.api.id
  deployment_id = aws_api_gateway_deployment.api.id
  stage_name    = "prod"
}

resource "aws_api_gateway_api_key" "agent" {
  name = "endpoint-agent-key"
}

resource "aws_api_gateway_usage_plan" "standard" {
  name = "endpoint-agent-plan"

  api_stages {
    api_id = aws_api_gateway_rest_api.api.id
    stage  = aws_api_gateway_stage.prod.stage_name
  }

  throttle_settings {
    rate_limit  = 10
    burst_limit = 20
  }
}

resource "aws_api_gateway_usage_plan_key" "agent" {
  key_id        = aws_api_gateway_api_key.agent.id
  key_type      = "API_KEY"
  usage_plan_id = aws_api_gateway_usage_plan.standard.id
}
