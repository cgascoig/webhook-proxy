#!/bin/bash
set -e

STACK_NAME="webhook-proxy-prod"
API_NAME="websocketapi-${STACK_NAME}"
S3_BUCKET_NAME="webhook-proxy"
REGION="ap-southeast-2"

echo "SAM: Build ..."
sam build --use-container 
echo "SAM: Package ..."
sam package --s3-bucket "${S3_BUCKET_NAME}" --output-template-file packaged.yaml 
echo "SAM: Deploy ..."
sam deploy --template-file packaged.yaml --stack-name "${STACK_NAME}" --capabilities CAPABILITY_IAM


################################################################################################################
# All of the below should be unnecessary but CloudFormation doesn't support websocket API gateways yet ...
################################################################################################################

echo "Getting SubscribeFunction and OnDisconnectFunction ARNs..."

SUBSCRIBE_FUNC_ARN=$(aws cloudformation describe-stacks --stack-name "${STACK_NAME}" --output text --query "Stacks[?StackName == '${STACK_NAME}'].Outputs[] | [?OutputKey=='SubscribeFunction'].OutputValue")
ON_DISCONNECT_FUNC_ARN=$(aws cloudformation describe-stacks --stack-name "${STACK_NAME}" --output text --query "Stacks[?StackName == '${STACK_NAME}'].Outputs[] | [?OutputKey=='OnDisconnectFunction'].OutputValue")

if [[ -z "${SUBSCRIBE_FUNC_ARN}" || -z "${ON_DISCONNECT_FUNC_ARN}" ]]
then
    echo "SubscribeFunction or OnDisconnectFunction are not available"
    exit 1
fi

echo "SubscribeFunction ARN: ${SUBSCRIBE_FUNC_ARN}"
echo "OnDisconnectFunction ARN: ${ON_DISCONNECT_FUNC_ARN}"

echo "Getting API ID..."
API_ID=$(aws apigatewayv2 get-apis --output text --query "Items[?Name == '${API_NAME}'].ApiId")

if [[ -z "${API_ID}" ]]
then
    echo "WebSocket API doesn't exist, creating it ..."
    aws apigatewayv2 create-api --name "${API_NAME}" --protocol-type WEBSOCKET --route-selection-expression '$request.body.action'
    API_ID=$(aws apigatewayv2 get-apis --output text --query "Items[?Name == '${API_NAME}'].ApiId")
fi

echo "API_ID: '${API_ID}'"

echo "Creating integrations ..."
SUBSCRIBE_INTEGRATION=$(aws apigatewayv2 create-integration --api-id "${API_ID}" --integration-type "AWS_PROXY" --integration-uri "arn:aws:apigateway:${REGION}:lambda:path/2015-03-31/functions/${SUBSCRIBE_FUNC_ARN}/invocations" --output text --query 'IntegrationId' )
ON_DISCONNECT_INTEGRATION=$(aws apigatewayv2 create-integration --api-id "${API_ID}" --integration-type "AWS_PROXY" --integration-uri "arn:aws:apigateway:${REGION}:lambda:path/2015-03-31/functions/${ON_DISCONNECT_FUNC_ARN}/invocations" --output text --query 'IntegrationId' )

if [[ -z "${SUBSCRIBE_INTEGRATION}" || -z "${ON_DISCONNECT_INTEGRATION}" ]]
then
    echo "Subscribe or OnDisconnect integrations didn't create successfully"
    exit 1
fi

echo "Creating routes ..."
aws apigatewayv2 create-route --api-id "${API_ID}" --route-key 'subscribe' --target "integrations/${SUBSCRIBE_INTEGRATION}"
aws apigatewayv2 create-route --api-id "${API_ID}" --route-key '$disconnect' --target "integrations/${ON_DISCONNECT_INTEGRATION}"

echo "Creating deployment ..."
DEPLOYMENT_ID=$(aws apigatewayv2 create-deployment --api-id "${API_ID}" --output text --query "DeploymentId")

echo "Creating stage ..."
aws apigatewayv2 create-stage --api-id "${API_ID}" --stage-name active --deployment-id "${DEPLOYMENT_ID}"