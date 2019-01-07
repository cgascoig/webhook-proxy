from util import *
import os
import boto3
import json


def lambda_handler(event, context):
    """Sample pure Lambda function

    Parameters
    ----------
    event: dict, required
        API Gateway Lambda Proxy Input Format

        {
            "resource": "Resource path",
            "path": "Path parameter",
            "httpMethod": "Incoming request's method name"
            "headers": {Incoming request headers}
            "queryStringParameters": {query string parameters }
            "pathParameters":  {path parameters}
            "stageVariables": {Applicable stage variables}
            "requestContext": {Request context, including authorizer-returned key-value pairs}
            "body": "A JSON string of the request payload."
            "isBase64Encoded": "A boolean flag to indicate if the applicable request payload is Base64-encode"
        }

        https://docs.aws.amazon.com/apigateway/latest/developerguide/set-up-lambda-proxy-integrations.html#api-gateway-simple-proxy-for-lambda-input-format

    context: object, required
        Lambda Context runtime methods and attributes

    Attributes
    ----------

    context.aws_request_id: str
         Lambda request ID
    context.client_context: object
         Additional context when invoked through AWS Mobile SDK
    context.function_name: str
         Lambda function name
    context.function_version: str
         Function version identifier
    context.get_remaining_time_in_millis: function
         Time in milliseconds before function times out
    context.identity:
         Cognito identity provider context when invoked through AWS Mobile SDK
    context.invoked_function_arn: str
         Function ARN
    context.log_group_name: str
         Cloudwatch Log group name
    context.log_stream_name: str
         Cloudwatch Log stream name
    context.memory_limit_in_mb: int
        Function memory

        https://docs.aws.amazon.com/lambda/latest/dg/python-context-object.html

    Returns
    ------
    API Gateway Lambda Proxy Output Format: dict
        'statusCode' and 'body' are required

        {
            "isBase64Encoded": true | false,
            "statusCode": httpStatusCode,
            "headers": {"headerName": "headerValue", ...},
            "body": "..."
        }

        # api-gateway-simple-proxy-for-lambda-output-format
        https: // docs.aws.amazon.com/apigateway/latest/developerguide/set-up-lambda-proxy-integrations.html
    """

    log(f"Received webhook. Event {event}", INFO)

    subscriptionId = event["pathParameters"].get("webhookId")
    log(f"subscriptionId: {subscriptionId}", DEBUG)

    if subscriptionId is None:
        return {"statusCode": 404}

    table = getDDBTable(os.environ['TABLE_NAME'])
    response = table.query(
        IndexName='subscriptionId-index',
        KeyConditionExpression=boto3.dynamodb.conditions.Key('subscriptionId').eq(subscriptionId),
    )

    log(f"Queried DDB for connections with matching subscriptionId: {response['Items']}", DEBUG)

    apigw = boto3.client('apigatewaymanagementapi')

    for con in response['Items']:
        try:
            # I'm sure there's a better way to do this but the documentation is ... lacking at the moment
            apigw._endpoint.host = con['callbackEndpoint']
            
            apigw.post_to_connection(
                Data=json.dumps({
                    "action": "received_webhook",
                    "subscriptionId": subscriptionId,
                    "body": event['body'],
                    "contentType": event["headers"].get("content-type", "application/json"),
                }),
                ConnectionId=con['connectionId']
            )
        except:
            log(f"Exception occurred sending message to web socket with connectionId {con['connectionId']}, deleting connection", INFO)
            table.delete_item(Key={
                'connectionId': con['connectionId']
            })


    return {
        "statusCode": 200,
        "body": "connected",
    }
