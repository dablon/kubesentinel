package models

type ScanResult struct {
	Timestamp   string  `json:"timestamp"`
	ClusterName string  `json:"cluster_name"`
	Issues      []Issue `json:"issues"`
	Summary     Summary `json:"summary"`
}

type Summary struct {
	TotalIssues      int `json:"total_issues"`
	Critical         int `json:"critical"`
	High             int `json:"high"`
	Medium           int `json:"medium"`
	Low              int `json:"low"`
	ScannedPods      int `json:"scanned_pods"`
	ScannedWorkloads int `json:"scanned_workloads"`
}

type Issue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Message     string `json:"message"`
	Namespace   string `json:"namespace"`
	Resource    string `json:"resource"`
	Remediation string `json:"remediation,omitempty"`
}

type Pod struct {
	Name            string
	Namespace       string
	ResourceKind    string
	ResourceName    string
	Containers      []Container
	InitContainers  []Container
	SecurityContext PodSecurityContext
}

type PodSecurityContext struct {
	HostPID                      bool
	HostNetwork                  bool
	HostIPC                      bool
	AutomountServiceAccountToken *bool
}

type Container struct {
	Name            string
	Image           string
	ImagePullPolicy string
	SecurityContext SecurityContext
	Resources       ResourceRequirements
}

type SecurityContext struct {
	Privileged             *bool
	RunAsNonRoot           *bool
	RunAsUser              *int
	ReadOnlyRootFilesystem *bool
	Capabilities           Capabilities
	SeccompProfile         string
}

type Capabilities struct {
	Add  []string
	Drop []string
}

type ResourceRequirements struct {
	Limits map[string]string
}

func (r *ScanResult) HasCritical() bool {
	return r.Summary.Critical > 0
}

// SeverityOrder defines the ranking of severities.
var SeverityOrder = map[string]int{
	"critical": 4,
	"high":     3,
	"medium":   2,
	"low":      1,
}

// SeverityAtLeast returns true if issueSeverity is at or above the threshold.
func SeverityAtLeast(issueSeverity, threshold string) bool {
	return SeverityOrder[issueSeverity] >= SeverityOrder[threshold]
}
