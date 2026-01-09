package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Scenario represents a complete drill scenario configuration
type Scenario struct {
	Name              string        `yaml:"name"`
	Description       string        `yaml:"description,omitempty"`
	RTOTarget         string        `yaml:"rto_target"`
	RPOTarget         string        `yaml:"rpo_target,omitempty"`
	DisruptCommand    string        `yaml:"disrupt_command"`
	RecoverCommand    string        `yaml:"recover_command,omitempty"`
	HealthCheckCommand string        `yaml:"health_check_command"`
	PostDisruptDelay  string        `yaml:"post_disrupt_delay,omitempty"`
	RPOCheck          *RPOCheck     `yaml:"rpo_check,omitempty"`
	Factors           *Factors      `yaml:"factors,omitempty"`
}

// RPOCheck contains commands for RPO measurement
type RPOCheck struct {
	PreSnapshot  string `yaml:"pre_snapshot,omitempty"`
	PostSnapshot string `yaml:"post_snapshot,omitempty"`
	VerifyCommand string `yaml:"verify_command,omitempty"`
}

// Factors contains commands to collect influencing factors/logs
type Factors struct {
	LogCommands []string `yaml:"log_commands,omitempty"`
}

// ParseScenario reads and parses a YAML scenario file
func ParseScenario(filePath string) (*Scenario, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read scenario file: %w", err)
	}

	var scenario Scenario
	if err := yaml.Unmarshal(data, &scenario); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &scenario, nil
}

// Validate checks that all required fields are present and valid
func (s *Scenario) Validate() error {
	if s.Name == "" {
		return fmt.Errorf("required field 'name' is missing")
	}

	if s.RTOTarget == "" {
		return fmt.Errorf("required field 'rto_target' is missing")
	}

	if _, err := time.ParseDuration(s.RTOTarget); err != nil {
		return fmt.Errorf("invalid 'rto_target' duration: %w", err)
	}

	if s.RPOTarget != "" {
		if _, err := time.ParseDuration(s.RPOTarget); err != nil {
			return fmt.Errorf("invalid 'rpo_target' duration: %w", err)
		}
	}

	if s.DisruptCommand == "" {
		return fmt.Errorf("required field 'disrupt_command' is missing")
	}

	if s.HealthCheckCommand == "" {
		return fmt.Errorf("required field 'health_check_command' is missing")
	}

	if s.PostDisruptDelay != "" {
		if _, err := time.ParseDuration(s.PostDisruptDelay); err != nil {
			return fmt.Errorf("invalid 'post_disrupt_delay' duration: %w", err)
		}
	}

	return nil
}

// GetRTOTargetDuration returns the parsed RTO target duration
func (s *Scenario) GetRTOTargetDuration() (time.Duration, error) {
	return time.ParseDuration(s.RTOTarget)
}

// GetRPOTargetDuration returns the parsed RPO target duration, or zero if not set
func (s *Scenario) GetRPOTargetDuration() (time.Duration, error) {
	if s.RPOTarget == "" {
		return 0, nil
	}
	return time.ParseDuration(s.RPOTarget)
}

// GetPostDisruptDelay returns the parsed post-disrupt delay, or zero if not set
func (s *Scenario) GetPostDisruptDelay() (time.Duration, error) {
	if s.PostDisruptDelay == "" {
		return 0, nil
	}
	return time.ParseDuration(s.PostDisruptDelay)
}

