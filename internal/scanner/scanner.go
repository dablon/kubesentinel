package scanner

import (
	"fmt"
	"strings"
	"github.com/dablon/kubesentinel/internal/config"
	"github.com/dablon/kubesentinel/pkg/models"
)

var Verbose bool

type Scanner struct {
	config *config.Config
}

func New(cfg *config.Config) *Scanner {
	return &Scanner{config: cfg}
}

func (s *Scanner) Scan(namespace, severityFilter string) (*models.ScanResult, error) {
	result := &models.ScanResult{
		Timestamp:   "2026-03-03T05:00:00Z",
		ClusterName: "demo-cluster",
		Issues:      []models.Issue{},
		Summary:     models.Summary{},
	}

	// Simulated pods for demo - in real implementation would use K8s client
	pods := s.getDemoPods(namespace)
	
	for i := range pods {
		issues := s.scanPod(&pods[i])
		for _, issue := range issues {
			if severityFilter == "all" || strings.Contains(severityFilter, issue.Severity) {
				result.Issues = append(result.Issues, issue)
			}
		}
	}

	result.Summary = models.Summary{
		TotalIssues:  len(result.Issues),
		Critical:    countSeverity(result.Issues, "critical"),
		High:        countSeverity(result.Issues, "high"),
		Medium:      countSeverity(result.Issues, "medium"),
		Low:         countSeverity(result.Issues, "low"),
		ScannedPods: len(pods),
	}

	return result, nil
}

func (s *Scanner) getDemoPods(namespace string) []models.Pod {
	if namespace != "" && namespace != "all" {
		return []models.Pod{
			{
				Name:      "nginx-demo",
				Namespace: namespace,
				Containers: []models.Container{
					{
						Name: "nginx",
						SecurityContext: models.SecurityContext{
							Privileged: boolPtr(false),
							RunAsNonRoot: boolPtr(false),
							Capabilities: models.Capabilities{
								Add: []string{"NET_ADMIN"},
							},
						},
						Resources: models.ResourceRequirements{
							Limits: nil,
						},
					},
				},
				SecurityContext: models.PodSecurityContext{
					HostPID:     false,
					HostNetwork: true,
					HostIPC:    false,
				},
			},
		}
	}
	
	return []models.Pod{
		{
			Name:      "nginx-prod",
			Namespace: "production",
			Containers: []models.Container{
				{
					Name: "nginx",
					SecurityContext: models.SecurityContext{
						Privileged: boolPtr(true),
						RunAsNonRoot: boolPtr(false),
						Capabilities: models.Capabilities{
							Add: []string{"SYS_ADMIN", "NET_ADMIN"},
						},
					},
					Resources: models.ResourceRequirements{},
				},
			},
			SecurityContext: models.PodSecurityContext{
				HostPID:     true,
				HostNetwork: true,
				HostIPC:    true,
			},
		},
		{
			Name:      "api-server",
			Namespace: "default",
			Containers: []models.Container{
				{
					Name: "api",
					SecurityContext: models.SecurityContext{
						Privileged:     boolPtr(false),
						RunAsNonRoot:  boolPtr(true),
						RunAsUser:     intPtr(1000),
					},
					Resources: models.ResourceRequirements{
						Limits: map[string]string{
							"cpu":    "500m",
							"memory": "256Mi",
						},
					},
				},
			},
			SecurityContext: models.PodSecurityContext{
				HostPID:     false,
				HostNetwork: false,
				HostIPC:    false,
			},
		},
		{
			Name:      "mysql-db",
			Namespace: "database",
			Containers: []models.Container{
				{
					Name: "mysql",
					SecurityContext: models.SecurityContext{
						Privileged: boolPtr(false),
					},
					Resources: models.ResourceRequirements{
						Limits: nil,
					},
				},
			},
			SecurityContext: models.PodSecurityContext{
				HostPID:     false,
				HostNetwork: false,
				HostIPC:    false,
			},
		},
	}
}

func (s *Scanner) scanPod(pod *models.Pod) []models.Issue {
	var issues []models.Issue

	for _, container := range pod.Containers {
		// Check running as root
		if container.SecurityContext.RunAsNonRoot != nil && !*container.SecurityContext.RunAsNonRoot {
			issues = append(issues, models.Issue{
				Type:        "privilege",
				Severity:    "high",
				Message:     fmt.Sprintf("Container %s runs as root", container.Name),
				Namespace:  pod.Namespace,
				Resource:   fmt.Sprintf("pod/%s", pod.Name),
				Remediation: "Set runAsNonRoot: true and runAsUser > 0",
			})
		}

		// Check capabilities
		if len(container.SecurityContext.Capabilities.Add) > 0 {
			dangerousCaps := []string{"SYS_ADMIN", "NET_ADMIN", "DAC_READ_SEARCH", "SYS_RAWIO"}
			hasDangerous := false
			for _, cap := range container.SecurityContext.Capabilities.Add {
				for _, dangerous := range dangerousCaps {
					if cap == dangerous {
						hasDangerous = true
						break
					}
				}
			}
			if hasDangerous {
				issues = append(issues, models.Issue{
					Type:        "capabilities",
					Severity:    "medium",
					Message:     fmt.Sprintf("Container %s has dangerous capabilities: %v", container.Name, container.SecurityContext.Capabilities.Add),
					Namespace:  pod.Namespace,
					Resource:   fmt.Sprintf("pod/%s", pod.Name),
					Remediation: "Drop all capabilities and add only what's needed",
				})
			}
		}

		// Check privileged
		if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
			issues = append(issues, models.Issue{
				Type:        "privilege",
				Severity:    "critical",
				Message:     fmt.Sprintf("Container %s runs in privileged mode", container.Name),
				Namespace:  pod.Namespace,
				Resource:   fmt.Sprintf("pod/%s", pod.Name),
				Remediation: "Set privileged: false",
			})
		}

		// Check hostPID
		if pod.SecurityContext.HostPID {
			issues = append(issues, models.Issue{
				Type:        "privilege",
				Severity:    "high",
				Message:     "Pod has hostPID enabled",
				Namespace:  pod.Namespace,
				Resource:   fmt.Sprintf("pod/%s", pod.Name),
				Remediation: "Disable hostPID",
			})
		}

		// Check hostNetwork
		if pod.SecurityContext.HostNetwork {
			issues = append(issues, models.Issue{
				Type:        "network",
				Severity:    "medium",
				Message:     "Pod has hostNetwork enabled",
				Namespace:  pod.Namespace,
				Resource:   fmt.Sprintf("pod/%s", pod.Name),
				Remediation: "Disable hostNetwork unless required",
			})
		}

		// Check hostIPC
		if pod.SecurityContext.HostIPC {
			issues = append(issues, models.Issue{
				Type:        "privilege",
				Severity:    "high",
				Message:     "Pod has hostIPC enabled",
				Namespace:  pod.Namespace,
				Resource:   fmt.Sprintf("pod/%s", pod.Name),
				Remediation: "Disable hostIPC",
			})
		}

		// Check resources
		if container.Resources.Limits == nil {
			issues = append(issues, models.Issue{
				Type:        "resources",
				Severity:    "low",
				Message:     fmt.Sprintf("Container %s has no resource limits", container.Name),
				Namespace:  pod.Namespace,
				Resource:   fmt.Sprintf("pod/%s", pod.Name),
				Remediation: "Set resource limits",
			})
		}
	}

	return issues
}

func (s *Scanner) PrintResults(result *models.ScanResult) {
	fmt.Printf("\n=== KubeSentinel Security Scan ===\n")
	fmt.Printf("Cluster: %s\n", result.ClusterName)
	fmt.Printf("Time: %s\n", result.Timestamp)
	fmt.Printf("\n--- Summary ---\n")
	fmt.Printf("Total Issues: %d\n", result.Summary.TotalIssues)
	fmt.Printf("  Critical: %d\n", result.Summary.Critical)
	fmt.Printf("  High: %d\n", result.Summary.High)
	fmt.Printf("  Medium: %d\n", result.Summary.Medium)
	fmt.Printf("  Low: %d\n", result.Summary.Low)
	fmt.Printf("Scanned Pods: %d\n", result.Summary.ScannedPods)

	if len(result.Issues) > 0 {
		fmt.Printf("\n--- Issues ---\n")
		for _, issue := range result.Issues {
			icon := map[string]string{"critical": "🔴", "high": "🟠", "medium": "🟡", "low": "🟢"}[issue.Severity]
			fmt.Printf("%s [%s] %s\n", icon, issue.Severity, issue.Message)
			fmt.Printf("   Namespace: %s\n", issue.Namespace)
			fmt.Printf("   Resource: %s\n", issue.Resource)
			if issue.Remediation != "" {
				fmt.Printf("   Fix: %s\n", issue.Remediation)
			}
		}
	}
}

func (s *Scanner) PrintJSON(result *models.ScanResult) {
	fmt.Printf(`{"cluster":"%s","timestamp":"%s","issues":%d,"summary":{"critical":%d,"high":%d,"medium":%d,"low":%d}}`,
		result.ClusterName, result.Timestamp, len(result.Issues),
		result.Summary.Critical, result.Summary.High, result.Summary.Medium, result.Summary.Low)
}

func countSeverity(issues []models.Issue, sev string) int {
	count := 0
	for _, i := range issues {
		if i.Severity == sev {
			count++
		}
	}
	return count
}

func boolPtr(b bool) *bool { return &b }
func intPtr(i int) *int { return &i }
