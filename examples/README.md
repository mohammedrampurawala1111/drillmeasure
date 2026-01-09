# Example Scenarios

This directory contains example scenario files and supporting Kubernetes manifests for testing drillmeasure.

## Kubernetes Pod Failure Example

### Prerequisites

1. Deploy the example application:
   ```bash
   kubectl apply -f examples/kubernetes-deployment.yaml
   ```

2. Wait for pods to be ready:
   ```bash
   kubectl wait --for=condition=ready pod -l app=webapp -n production --timeout=60s
   ```

3. Verify the service is accessible:
   ```bash
   kubectl run -it --rm debug --image=curlimages/curl --restart=Never -- \
     curl http://webapp.production.svc.cluster.local/health
   ```

### Running the Drill

**Option 1: Using kubectl run (may be slower)**
```bash
drillmeasure run examples/kubernetes-pod-failure.yaml
```

**Option 2: Using kubectl exec (faster, recommended)**
```bash
drillmeasure run examples/kubernetes-pod-failure-simple.yaml
```

**Note:** If the drill appears to hang, it's likely waiting for the health check to pass. The health check runs every 5 seconds until the service recovers or the RTO target (5 minutes) is exceeded. Check the reports directory for detailed logs.

### What Happens

1. The drill deletes one pod matching `app=webapp` in the `production` namespace
2. Kubernetes automatically recreates the pod (if replicas > 1)
3. The health check verifies:
   - At least one pod is in `Running` state
   - The service endpoint responds with HTTP 200
4. RTO is measured from disruption until health check passes
5. Reports are generated with full evidence

### Cleanup

```bash
kubectl delete -f examples/kubernetes-deployment.yaml
```

## Terraform AWS EC2 Recovery Example

### Prerequisites

1. AWS CLI configured with credentials
2. Terraform installed
3. Initialize and apply the infrastructure:
   ```bash
   cd examples/terraform-aws/ec2
   terraform init
   terraform apply
   ```

### Running the Drill

```bash
drillmeasure run examples/terraform-infra-recovery.yaml
```

### What Happens

1. The drill destroys the EC2 instance
2. Terraform recreates the instance
3. Health check waits for instance boot and service startup
4. Verifies HTTP server responds on port 3000
5. RTO is measured from disruption until health check passes

See `examples/terraform-aws/ec2/README.md` for detailed setup instructions.

## Other Examples

- **VM Service Restart** (`vm-service-restart.yaml`): Requires SSH access to a VM
- **Database RPO Check** (`database-rpo-check.yaml`): Requires MySQL database access

## Customizing Examples

All example scenarios use environment variables and can be customized:

- Replace placeholder values (e.g., `web-server.example.com`, `db-server.example.com`)
- Update commands to match your infrastructure
- Adjust RTO/RPO targets based on your requirements
- Add additional factor log commands for your specific use case

