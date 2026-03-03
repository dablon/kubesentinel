package models

type ScanResult struct {
	Timestamp   string   `json:"timestamp"`
	ClusterName string   `json:"cluster_name"`
	Issues      []Issue  `json:"issues"`
	Summary     Summary `json:"summary"`
}

type Summary struct {
	TotalIssues int `json:"total_issues"`
	Critical    int `json:"critical"`
	High        int `json:"high"`
	Medium      int `json:"medium"`
	Low         int `json:"low"`
	ScannedPods int `json:"scanned_pods"`
}

type Issue struct {
	Type        string `json:"type"`
	Severity    string `json:"severity"`
	Message    string `json:"message"`
	Namespace  string `json:"namespace"`
	Resource   string `json:"resource"`
	Remediation string `json:"remediation,omitempty"`
}

type Pod struct {
	Name             string
	Namespace        string
	Containers       []Container
	SecurityContext  PodSecurityContext
}

type PodSecurityContext struct {
	HostPID     bool
	HostNetwork bool
	HostIPC    bool
}

type Container struct {
	Name            string
	SecurityContext SecurityContext
	Resources       ResourceRequirements
}

type SecurityContext struct {
	Privileged    *bool
	RunAsNonRoot *bool
	RunAsUser    *int
	Capabilities Capabilities
}

type Capabilities struct {
	Add []string
}

type ResourceRequirements struct {
	Limits map[string]string
}

func (r *ScanResult) HasCritical() bool {
	return r.Summary.Critical > 0
}
