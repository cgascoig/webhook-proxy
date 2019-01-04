# webhook-proxy

This folder contains the "serverless" components of the webhook-proxy. This includes Lambda functions and a SAM template for deploying them along with the DynamoDB and the REST API gateway for incoming webhooks. CloudFormation does not yet support managing the API Gateway's WebSocket APIs so these need to be configured manually and connected to the Lambda functions.

The SAM template can be built, packaged and deployed using these commands:

```bash

#the following is only needed the first time to create the S3 bucket
aws s3 mb s3://webhook-proxy

sam build --use-container && sam package --s3-bucket webhook-proxy --output-template-file packaged.yaml && sam deploy --template-file packaged.yaml --stack-name webhook-proxy-dev --capabilities CAPABILITY_IAM
```

