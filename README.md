# drillmeasure

[![License](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/go-1.21+-00ADD8.svg)](https://golang.org)

**drillmeasure** is a platform-agnostic CLI tool for measuring Recovery Time Objective (RTO) and Recovery Point Objective (RPO) in any infrastructure environment. It runs arbitrary user-provided commands for disruption, health checks, snapshots, and log collection, making it compatible with Kubernetes, VMs, cloud services, on-premises infrastructure, and hybrid setups.

## Why drillmeasure?

### For Platform Engineering

- **Infrastructure-Agnostic**: Works with any environment—Kubernetes, VMs, cloud services, on-prem, or hybrid—by executing shell commands you provide
- **No Dependencies**: Single portable binary with no external dependencies beyond Go stdlib, Cobra, and YAML parsing
- **Flexible**: Define your own disruption scenarios, health checks, and recovery verification logic
- **Evidence-Based**: Generates detailed reports with timestamps, command outputs, and cryptographic hashes for audit trails

### For Compliance (SOC 2, ISO 27001)

- **RTO Measurement**: Quantify how quickly your services recover from failures
- **RPO Measurement**: Verify acceptable data loss windows during disasters
- **Documentation**: Generate human-readable (Markdown) and machine-readable (JSON) reports suitable for audit evidence
- **Repeatable**: Run the same scenarios consistently to track improvements over time

## Installation

### From Source

```bash
go install github.com/drillmeasure/drillmeasure@latest
```

### Build from Source

```bash
git clone https://github.com/drillmeasure/drillmeasure.git
cd drillmeasure
go build -o drillmeasure .
```

## Quickstart

1. **Create a scenario file** (see examples below):

```yaml
name: my-service-recovery
rto_target: 5m
disrupt_command: kubectl delete pod -l app=myapp
health_check_command: curl -f http://myapp/health
```

2. **Validate the scenario**:

```bash
drillmeasure validate my-scenario.yaml
```

3. **Run the drill**:

```bash
drillmeasure run my-scenario.yaml
```

4. **Review the reports**:

Reports are generated in `reports/<timestamp>-<scenario-name>/`:
- `report.md` - Human-readable Markdown report
- `report.json` - Machine-readable JSON report

## Example Scenarios

The `examples/` directory contains realistic scenarios:

1. **Kubernetes Pod Failure** (`examples/kubernetes-pod-failure.yaml`)
   - Deletes a pod and measures recovery time
   - Uses kubectl and curl for health checks
   - **Deployment manifest**: `examples/kubernetes-deployment.yaml` (deploy with `kubectl apply`)

2. **VM Service Restart** (`examples/vm-service-restart.yaml`)
   - Stops a systemd service and monitors recovery
   - Uses SSH and systemctl commands

3. **Terraform Infrastructure Recovery** (`examples/terraform-infra-recovery.yaml`)
   - Destroys infrastructure and measures recreation time
   - Uses Terraform and AWS CLI

4. **Database RPO Check** (`examples/database-rpo-check.yaml`)
   - Measures both RTO and RPO for a database
   - Uses mysqldump and data verification commands

See `examples/README.md` for detailed setup instructions for each scenario.

## Scenario YAML Format

```yaml
name: string                    # Required: Scenario name
description: string            # Optional: Description
rto_target: duration           # Required: Target RTO (e.g., "5m", "1h30m")
rpo_target: duration           # Optional: Target RPO
disrupt_command: string        # Required: Command to simulate failure
health_check_command: string   # Required: Command that returns 0 when healthy
post_disrupt_delay: duration   # Optional: Wait after disruption before checking

rpo_check:                     # Optional: RPO measurement
  pre_snapshot: string         # Command to run before disruption
  post_snapshot: string        # Command to run after recovery
  verify_command: string      # Command to verify data loss (exit 0 = pass)

factors:                       # Optional: Influencing factors
  log_commands:                # Commands to collect logs/evidence
    - string
```

### Duration Format

Durations use Go's time.Duration format:
- `5m` - 5 minutes
- `1h30m` - 1 hour 30 minutes
- `30s` - 30 seconds
- `500ms` - 500 milliseconds

## How It Works

1. **Pre-snapshot** (if configured): Executes `rpo_check.pre_snapshot` command
2. **Disruption**: Executes `disrupt_command` to simulate failure
3. **Post-disrupt delay** (if configured): Waits for the specified duration (captures propagation delay)
4. **Recovery** (if configured): Executes `recover_command` to restore infrastructure
5. **RTA Measurement** (Recovery Time Actual):
   - **RTA Start**: First failed health check after disruption (when service actually goes down)
   - **RTA End**: First successful health check (when service is fully recovered)
   - Repeatedly runs `health_check_command` every 5 seconds (configurable)
   - Each health check has a 5-minute timeout (configurable)
   - Compares RTA vs RTO target → PASS/FAIL
6. **Post-snapshot** (if configured): Executes `rpo_check.post_snapshot` command
7. **RPO Verification** (if configured): Executes `rpo_check.verify_command`
8. **Factor Collection**: Executes all `factors.log_commands` to capture influencing factors
9. **Report Generation**: Creates Markdown and JSON reports with full evidence

### RTO vs RTA Terminology

- **RTO (Recovery Time Objective)**: The TARGET - maximum acceptable downtime (set in YAML as `rto_target`)
- **RTA (Recovery Time Actual)**: The MEASURED downtime - actual time from when service went down until it recovered
- **PASS/FAIL**: Determined by comparing RTA ≤ RTO target

## Report Output

### Markdown Report

Includes:
- Executive summary with PASS/FAIL status
- Detailed timeline of all events
- Health check attempt history
- Full command outputs with timestamps
- SHA256 hashes of all outputs (for tamper detection)
- Compliance notes for audit purposes

### JSON Report

Machine-readable format with:
- All timing data
- Command results with full outputs
- Hashes for verification
- Structured data for integration with monitoring/alerting systems

## Integration with Other Tools

**drillmeasure** complements existing chaos engineering and disaster recovery tools:

- **LitmusChaos**: Use drillmeasure to measure RTO/RPO after LitmusChaos experiments
- **Velero**: Measure recovery time after Velero backup/restore operations
- **Terraform**: Measure infrastructure recreation time after destroy/apply cycles
- **Custom Scripts**: Wrap any existing automation in drillmeasure scenarios

## Commands

### `drillmeasure run <scenario.yaml>`

Execute a complete drill scenario and generate reports.

### `drillmeasure validate <scenario.yaml>`

Validate a scenario YAML file for syntax and required fields.

### `drillmeasure version`

Print version information.

## Requirements

- Go 1.21 or later (for building from source)
- Bash shell (commands are executed via `bash -c`)
- Network access to your infrastructure (for SSH, kubectl, API calls, etc.)

## License

Apache 2.0 - see [LICENSE](LICENSE) file for details.

## Contributing

Contributions are welcome! This project is designed to be a CNCF Sandbox candidate, focusing on:

- Platform-agnostic disaster recovery measurement
- Compliance and audit trail generation
- Integration with existing infrastructure tooling

## Roadmap

- [ ] Support for parallel scenario execution
- [ ] Webhook notifications on drill completion
- [ ] Historical trend analysis
- [ ] Integration with Prometheus/Grafana
- [ ] Scenario templates for common patterns

## Example Output

```
Running scenario: kubernetes-pod-failure
Description: Simulates a pod failure in Kubernetes and measures recovery time.

Output directory: reports/2026-01-08-143022-kubernetes-pod-failure

Drill completed!
RTO: 2m15s (target: 5m) - ✅ PASS
RPO: ✅ PASS

Reports generated in: reports/2026-01-08-143022-kubernetes-pod-failure
```

## Support

For issues, questions, or contributions, please open an issue on GitHub.

