package detect

import (
	"fmt"
	"os"
	"strings"
)

// Environment represents the detected environment type
type Environment string

const (
	EnvLocal      Environment = "local"
	EnvDocker     Environment = "docker"
	EnvWSL2       Environment = "wsl2"
	EnvCloudVM    Environment = "cloud"
	EnvVirtualBox Environment = "virtualbox"
	EnvVMware     Environment = "vmware"
)

// GetEnvironment detects the current environment
func GetEnvironment() (Environment, string) {
	// Check for Docker
	if isDocker() {
		return EnvDocker, "Docker container detected"
	}

	// Check for WSL2
	if isWSL2() {
		return EnvWSL2, "WSL2 environment detected"
	}

	// Check for Cloud VMs
	if cloudProv := isCloudVM(); cloudProv != "" {
		return EnvCloudVM, fmt.Sprintf("Cloud VM detected: %s", cloudProv)
	}

	// Check for VirtualBox
	if isVirtualBox() {
		return EnvVirtualBox, "VirtualBox VM detected"
	}

	// Check for VMware
	if isVMware() {
		return EnvVMware, "VMware VM detected"
	}

	return EnvLocal, "Local environment detected"
}

// ShouldBindPublic returns true if we should bind to 0.0.0.0
// based on the detected environment
func ShouldBindPublic() bool {
	detectedEnv, _ := GetEnvironment()

	// Bind to 0.0.0.0 for容器化环境和云环境
	switch detectedEnv {
	case EnvDocker, EnvWSL2, EnvCloudVM, EnvVirtualBox, EnvVMware:
		return true
	default:
		return false
	}
}

// GetBindingMessage returns a user-friendly message about the binding
func GetBindingMessage() string {
	_, desc := GetEnvironment()

	if ShouldBindPublic() {
		return fmt.Sprintf("Running in %s: binding to 0.0.0.0 for external network access", desc)
	}

	return fmt.Sprintf("Running in %s: binding to 127.0.0.1 (local only)", desc)
}

// isDocker returns true if running in a Docker container
func isDocker() bool {
	// Method 1: Check for /.dockerenv file
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Method 2: Check cgroup for docker string
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		return strings.Contains(string(data), "docker") || strings.Contains(string(data), "lxc")
	}

	return false
}

// isWSL2 returns true if running in WSL2
func isWSL2() bool {
	// Check if we're on Linux first
	if os.Getenv("OS") == "Windows_NT" {
		return false
	}

	// Check /proc/version for Microsoft strings
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}

	version := string(data)
	return strings.Contains(version, "Microsoft") || strings.Contains(version, "WSL")
}

// isCloudVM detects cloud VMs
// Returns the cloud provider name if detected, empty string otherwise
func isCloudVM() string {
	// AWS: /sys/hypervisor/uuid
	if _, err := os.Stat("/sys/hypervisor/uuid"); err == nil {
		// Check content for Amazon UUID pattern
		if data, err := os.ReadFile("/sys/hypervisor/uuid"); err == nil {
			if strings.HasPrefix(string(data), "ec2") {
				return "AWS EC2"
			}
		}
		return "AWS"
	}

	// GCP: Check for GCP specific files
	if _, err := os.ReadFile("/etc/os-release"); err == nil {
		if data, err := os.ReadFile("/var/lib/cloud/data/instance-id"); err == nil {
			if len(data) > 0 {
				return "GCP"
			}
		}
	}

	// Azure: WAAgent file
	if _, err := os.Stat("/var/lib/waagent/ManagedHostname-0.xml"); err == nil {
		return "Azure"
	}

	// DigitalOcean
	if _, err := os.Stat("/etc/digitalocean"); err == nil {
		return "DigitalOcean"
	}

	return ""
}

// isVirtualBox returns true if running in VirtualBox
func isVirtualBox() bool {
	// Check /sys/class/dmi/id/product_name
	if data, err := os.ReadFile("/sys/class/dmi/id/product_name"); err == nil {
		return strings.Contains(strings.ToLower(string(data)), "virtualbox")
	}

	// Check /sys/class/dmi/id/bios_vendor
	if data, err := os.ReadFile("/sys/class/dmi/id/bios_vendor"); err == nil {
		return strings.EqualFold(strings.TrimSpace(string(data)), "innotek gmbh")
	}

	// Check /proc/cpuinfo for hypervisor flags
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		cpuinfo := string(data)
		return strings.Contains(strings.ToLower(cpuinfo), "virtualbox")
	}

	return false
}

// isVMware returns true if running in VMware
func isVMware() bool {
	// Check /sys/class/dmi/id/product_name
	if data, err := os.ReadFile("/sys/class/dmi/id/product_name"); err == nil {
		product := strings.ToLower(string(data))
		return strings.Contains(product, "vmware")
	}

	// Check /sys/class/dmi/id/sys_vendor
	if data, err := os.ReadFile("/sys/class/dmi/id/sys_vendor"); err == nil {
		vendor := strings.ToLower(string(data))
		return strings.Contains(vendor, "vmware")
	}

	// Check /proc/cpuinfo for hypervisor info
	if data, err := os.ReadFile("/proc/cpuinfo"); err == nil {
		cpuinfo := strings.ToLower(string(data))
		return strings.Contains(cpuinfo, "vmware")
	}

	return false
}
