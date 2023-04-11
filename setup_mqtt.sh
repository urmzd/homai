#!/bin/bash

# Set your AWS CLI profile, region, and the target cluster name
TARGET_CLUSTER_NAME="SunriseLamp-MQTTBrokerCluster7506FC6E-c83XaduNaKX6"

TASK_ARN=$(aws ecs list-tasks --cluster $TARGET_CLUSTER_NAME | jq -r '.taskArns[0]')
TASK_ID="${TASK_ARN##*/}"
RUNTIME_ID=$(aws ecs describe-tasks --cluster $TARGET_CLUSTER_NAME --tasks $TASK_ID --query 'tasks[0].containers[0].runtimeId' --output text)

TARGET="ecs:${TARGET_CLUSTER_NAME}_${TASK_ID}_${RUNTIME_ID}"

echo "$TARGET"

aws ssm start-session --target $TARGET\
  --parameters '{"portNumber":["1883"], "localPortNumber":["1883"]}'\
  --document-name AWS-StartPortForwardingSession
