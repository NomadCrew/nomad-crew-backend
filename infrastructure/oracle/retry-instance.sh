#!/bin/bash
# Retry loop for OCI ARM instance creation
# Runs until instance is created successfully

ATTEMPT=1
MAX_ATTEMPTS=60  # Stop after 60 attempts (1 hour at 60s intervals)

while [ $ATTEMPT -le $MAX_ATTEMPTS ]; do
    echo ""
    echo "=========================================="
    echo "Attempt $ATTEMPT of $MAX_ATTEMPTS - $(date)"
    echo "=========================================="
    
    if terraform apply -auto-approve -var="availability_domain_number=1" 2>&1; then
        echo ""
        echo "SUCCESS! Instance created!"
        terraform output instance_public_ip
        exit 0
    fi
    
    echo "Failed. Waiting 60 seconds before retry..."
    ATTEMPT=$((ATTEMPT + 1))
    sleep 60
done

echo "Max attempts reached. Try again later or use a different region."
exit 1
