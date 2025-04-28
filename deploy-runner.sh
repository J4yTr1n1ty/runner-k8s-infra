#!/bin/bash
set -e

# Usage information
if [ "$#" -lt 2 ]; then
  echo "Usage: $0 <org-name> <github-token>"
  echo "Example: $0 myorg ghp_1234567890abcdef"
  exit 1
fi

ORG_NAME=$1
GITHUB_TOKEN=$2

# Create a temporary file
TEMP_FILE=$(mktemp)

# Read the template
cat github-runner-deployment.yaml >$TEMP_FILE

# Replace placeholders
sed -i "s/org-name/$ORG_NAME/g" $TEMP_FILE

# Base64 encode the token (handling platform-specific base64 options)
if [[ "$OSTYPE" == "darwin"* ]]; then
  # macOS
  TOKEN_B64=$(echo -n "$GITHUB_TOKEN" | base64)
else
  # Linux and others
  TOKEN_B64=$(echo -n "$GITHUB_TOKEN" | base64 -w 0)
fi

# Replace the token placeholder
sed -i "s/YOUR_BASE64_ENCODED_TOKEN/$TOKEN_B64/g" $TEMP_FILE

# Apply the deployment
echo "Deploying GitHub Runner for organization: $ORG_NAME"
# Create namespace if it doesn't exist
kubectl apply -f github-runners-namespace.yaml

# Apply the deployment to the github-runners namespace
kubectl apply -f $TEMP_FILE -n github-runners

# Clean up
rm $TEMP_FILE

echo "Deployment complete!"
echo "Check status with: kubectl get pods -l org=$ORG_NAME -n github-runners"
