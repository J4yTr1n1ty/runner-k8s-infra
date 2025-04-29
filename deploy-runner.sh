#!/bin/bash
set -e

# Usage information
if [ "$#" -lt 1 ]; then
  echo "Usage: $0 <org-name>"
  echo "Example: $0 myorg"
  echo "Note: GitHub token will be read from GITHUB_TOKEN environment variable or prompted securely"
  exit 1
fi

ORG_NAME=$1
ORG_NAME=$(echo "$ORG_NAME" | tr '[:upper:]' '[:lower:]')

if [ -z "${GITHUB_TOKEN}" ]; then
  echo "GitHub token not found in environment variables."
  echo "Please enter your GitHub token (input will not be displayed):"
  read -s GITHUB_TOKEN

  # Validate that something was entered
  if [ -z "${GITHUB_TOKEN}" ]; then
    echo "Error: GitHub token is required."
    exit 1
  fi
  echo "Token received."
else
  echo "Using GitHub token from environment variable."
fi

# Create a temporary file
TEMP_FILE=$(mktemp)

# Read the template
cat github-runner-deployment.yaml >"$TEMP_FILE"

# Replace organization name placeholders
sed -i "s/org-name/$ORG_NAME/g" "$TEMP_FILE"

# Base64 encode the token (handling platform-specific base64 options)
if [[ "$OSTYPE" == "darwin"* ]]; then
  # macOS
  TOKEN_B64=$(echo -n "$GITHUB_TOKEN" | base64)
else
  # Linux and others
  TOKEN_B64=$(echo -n "$GITHUB_TOKEN" | base64 -w 0)
fi

# Replace the token placeholder
sed -i "s/YOUR_BASE64_ENCODED_TOKEN/$TOKEN_B64/g" "$TEMP_FILE"

# Apply the deployment
echo "Deploying GitHub Runner for organization: $ORG_NAME"

# Create namespace if it doesn't exist
microk8s kubectl apply -f github-runners-namespace.yaml

# Apply the deployment to the github-runners namespace
microk8s kubectl apply -f "$TEMP_FILE" -n github-runners

# Clean up
rm "$TEMP_FILE"

# Unset the token variable for security
unset GITHUB_TOKEN

echo "Deployment complete!"
echo "Check status with: microk8s kubectl get pods -l org=$ORG_NAME -n github-runners"
