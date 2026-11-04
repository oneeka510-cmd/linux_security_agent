"""Lambda: POST /ingest. Receives the agent's JSON report and stores it in DynamoDB."""

import json
import os
from datetime import datetime, timezone

import boto3

dynamodb = boto3.resource("dynamodb")
hosts_table = dynamodb.Table(os.environ["HOSTS_TABLE"])
packages_table = dynamodb.Table(os.environ["PACKAGES_TABLE"])
cis_table = dynamodb.Table(os.environ["CIS_TABLE"])

HEADERS = {
    "Content-Type": "application/json",
    "Access-Control-Allow-Origin": "*",
}


def respond(status, body):
    return {"statusCode": status, "headers": HEADERS, "body": json.dumps(body)}


def handler(event, context):
    try:
        report = json.loads(event.get("body") or "")
    except json.JSONDecodeError:
        return respond(400, {"error": "body is not valid JSON"})

    host = report.get("host")
    if not isinstance(host, dict) or not host.get("machine_id"):
        return respond(400, {"error": "host.machine_id is required"})

    machine_id = host["machine_id"]
    checks = report.get("checks") or []
    packages = report.get("packages") or []
    received_at = datetime.now(timezone.utc).isoformat()

    # How many checks ended in each status, e.g. {"PASS": 3, "FAIL": 7}
    summary = {}
    for check in checks:
        status = check.get("status", "UNKNOWN")
        summary[status] = summary.get(status, 0) + 1

    # Each run overwrites the host's latest data (no history in the MVP).
    hosts_table.put_item(Item={
        **host,
        "last_seen": received_at,
        "package_count": len(packages),
        "check_summary": summary,
    })
    packages_table.put_item(Item={
        "machine_id": machine_id,
        "collected_at": host.get("collected_at", received_at),
        "packages": packages,
    })
    cis_table.put_item(Item={
        "machine_id": machine_id,
        "collected_at": host.get("collected_at", received_at),
        "summary": summary,
        "checks": checks,
    })

    return respond(200, {"status": "ok", "machine_id": machine_id,
                         "packages": len(packages), "checks": len(checks)})
