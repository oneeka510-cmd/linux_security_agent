output "api_url" {
  description = "Base address of the API (add /ingest, /hosts, /apps, /cis-results)"
  value       = aws_api_gateway_stage.prod.invoke_url
}

output "frontend_url" {
  description = "Address of the dashboard (plain HTTP, served by S3 website hosting)"
  value       = "http://${aws_s3_bucket_website_configuration.frontend.website_endpoint}"
}

output "api_key" {
  description = "API key. Read it with: terraform output -raw api_key"
  value       = aws_api_gateway_api_key.agent.value
  sensitive   = true
}
