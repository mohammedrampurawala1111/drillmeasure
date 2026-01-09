# Terraform AWS EC2 Infrastructure

This directory contains Terraform configuration for a simple EC2 instance that runs an HTTP server on port 3000.

## Prerequisites

1. AWS CLI configured with credentials
2. Terraform installed (>= 1.0)
3. Appropriate AWS permissions to create VPC, EC2 instances, etc.

## Setup

1. **Initialize Terraform:**
   ```bash
   cd examples/terraform-aws/ec2
   terraform init
   ```

2. **Review and customize variables (optional):**
   - Edit `variables.tf` or use `-var` flags
   - Default region: `us-east-1`
   - Default instance type: `t3.micro`
   - Default CIDR: `0.0.0.0/0` (open to all - change for production!)

3. **Apply the infrastructure:**
   ```bash
   terraform apply
   ```

4. **Verify it's working:**
   ```bash
   terraform output url
   curl $(terraform output -raw url)
   ```

## Running the Drill

From the project root:

```bash
./drillmeasure run examples/terraform-infra-recovery.yaml
```

## What Happens

1. The drill destroys the EC2 instance (`aws_instance.web`)
2. Terraform recreates the instance
3. The health check waits 30 seconds for the instance to boot and the service to start
4. The health check verifies the HTTP server responds on port 3000
5. RTO is measured from disruption until health check passes

## Cleanup

```bash
cd examples/terraform-aws/ec2
terraform destroy
```

## Notes

- The instance uses Amazon Linux 2023
- The HTTP server starts automatically via systemd
- Port 3000 is exposed via security group
- The instance gets a public IP automatically
- User data script installs and starts the HTTP server

