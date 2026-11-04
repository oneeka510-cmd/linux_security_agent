"""Lambda: GET /hosts, /apps, /cis-results. Reads from DynamoDB and returns JSON."""

import json
import os
from decimal import Decimal

import boto3

dynamodb = boto3.resource("dynamodb")
hosts_table = dynamodb.Table(os.environ["HOSTS_TABLE"])
packages_table = dynamodb.Table(os.environ["PACKAGES_TABLE"])
cis_table = dynamodb.Table(os.environ["CIS_TABLE"])

HEADERS = {
    "Content-Type": "application/json",
    "Access-Control-Allow-Origin": "*",
}


def to_json(value):
    # DynamoDB returns numbers as Decimal, which json cannot encode by default.
    if isinstance(value, Decimal):
        return int(value) if value == value.to_integral_value() else float(value)
    raise TypeError(f"cannot encode {type(value)}")


def respond(status, body):
    return {"statusCode": status, "headers": HEADERS, "body": json.dumps(body, default=to_json)}


def scan_all(table):
    """Return every item in a table, following DynamoDB's pagination."""
    response = table.scan()
    items = response["Items"]
    while "LastEvaluatedKey" in response:
        response = table.scan(ExclusiveStartKey=response["LastEvaluatedKey"])
        items.extend(response["Items"])
    return items


def get_one(table, machine_id):
    item = table.get_item(Key={"machine_id": machine_id}).get("Item")
    if item is None:
        return respond(404, {"error": f"no data for machine_id {machine_id}"})
    return respond(200, item)


def handler(event, context):
    path = event.get("path", "")
    params = event.get("queryStringParameters") or {}
    machine_id = params.get("machine_id")

    if path.endswith("/hosts"):
        return respond(200, scan_all(hosts_table))

    if path.endswith("/apps"):
        if not machine_id:
            return respond(400, {"error": "machine_id query parameter is required"})
        return get_one(packages_table, machine_id)

    if path.endswith("/cis-results"):
        if machine_id:
            return get_one(cis_table, machine_id)
        return respond(200, scan_all(cis_table))

    return respond(404, {"error": "unknown path"})
