package scanner

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"testing"

	"github.com/dablon/kubesentinel/internal/config"
	"github.com/dablon/kubesentinel/pkg/models"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// --- Helpers ---

func newTestScanner(t *testing.T, pods ...corev1.Pod) *Scanner {
	t.Helper()
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	for i := range pods {
		_, err := client.CoreV1().Pods(pods[i].Namespace).Create(ctx, &pods[i], metav1.CreateOptions{})
		if err != nil {
			t.Fatalf("failed to create test pod: %v", err)
		}
	}
	return NewWithClient(config.Default(), client, "test-cluster")
}

func newTestScannerWithWorkloads(t *testing.T, pods []corev1.Pod, deployments []appsv1.Deployment,
	statefulsets []appsv1.StatefulSet, daemonsets []appsv1.DaemonSet,
	jobs []batchv1.Job, cronjobs []batchv1.CronJob) *Scanner {
	t.Helper()
	client := fake.NewSimpleClientset()
	ctx := context.Background()
	for i := range pods {
		client.CoreV1().Pods(pods[i].Namespace).Create(ctx, &pods[i], metav1.CreateOptions{})
	}
	for i := range deployments {
		client.AppsV1().Deployments(deployments[i].Namespace).Create(ctx, &deployments[i], metav1.CreateOptions{})
	}
	for i := range statefulsets {
		client.AppsV1().StatefulSets(statefulsets[i].Namespace).Create(ctx, &statefulsets[i], metav1.CreateOptions{})
	}
	for i := range daemonsets {
		client.AppsV1().DaemonSets(daemonsets[i].Namespace).Create(ctx, &daemonsets[i], metav1.CreateOptions{})
	}
	for i := range jobs {
		client.BatchV1().Jobs(jobs[i].Namespace).Create(ctx, &jobs[i], metav1.CreateOptions{})
	}
	for i := range cronjobs {
		client.BatchV1().CronJobs(cronjobs[i].Namespace).Create(ctx, &cronjobs[i], metav1.CreateOptions{})
	}
	return NewWithClient(config.Default(), client, "test-cluster")
}

func boolPtr(b bool) *bool    { return &b }
func int64Ptr(i int64) *int64 { return &i }
func int32Ptr(i int32) *int32 { return &i }

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// --- Test Pods ---

func privilegedPod() corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "nginx-prod", Namespace: "production"},
		Spec: corev1.PodSpec{
			HostPID:     true,
			HostNetwork: true,
			HostIPC:     true,
			Containers: []corev1.Container{
				{
					Name:  "nginx",
					Image: "nginx:latest",
					SecurityContext: &corev1.SecurityContext{
						Privileged: boolPtr(true),
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{"SYS_ADMIN", "NET_ADMIN"},
						},
					},
				},
			},
		},
	}
}

func securePod() corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-server", Namespace: "default"},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: boolPtr(false),
			Containers: []corev1.Container{
				{
					Name:            "api",
					Image:           "api:v1.2.3",
					ImagePullPolicy: corev1.PullAlways,
					SecurityContext: &corev1.SecurityContext{
						Privileged:             boolPtr(false),
						RunAsNonRoot:           boolPtr(true),
						RunAsUser:              int64Ptr(1000),
						ReadOnlyRootFilesystem: boolPtr(true),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("256Mi"),
						},
					},
				},
			},
		},
	}
}

func noLimitsPod() corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "mysql-db", Namespace: "database"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "mysql",
					Image: "mysql:8.0",
					SecurityContext: &corev1.SecurityContext{
						Privileged: boolPtr(false),
					},
				},
			},
		},
	}
}

func podWithInitContainer() corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "app-with-init", Namespace: "default"},
		Spec: corev1.PodSpec{
			AutomountServiceAccountToken: boolPtr(false),
			InitContainers: []corev1.Container{
				{
					Name:  "init-db",
					Image: "busybox:latest",
					SecurityContext: &corev1.SecurityContext{
						Privileged: boolPtr(true),
					},
				},
			},
			Containers: []corev1.Container{
				{
					Name:            "app",
					Image:           "app:v1.0.0",
					ImagePullPolicy: corev1.PullAlways,
					SecurityContext: &corev1.SecurityContext{
						Privileged:             boolPtr(false),
						RunAsNonRoot:           boolPtr(true),
						RunAsUser:              int64Ptr(1000),
						ReadOnlyRootFilesystem: boolPtr(true),
						Capabilities: &corev1.Capabilities{
							Drop: []corev1.Capability{"ALL"},
						},
						SeccompProfile: &corev1.SeccompProfile{
							Type: corev1.SeccompProfileTypeRuntimeDefault,
						},
					},
					Resources: corev1.ResourceRequirements{
						Limits: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("200m"),
							corev1.ResourceMemory: resource.MustParse("128Mi"),
						},
					},
				},
			},
		},
	}
}

// --- Workload Fixtures ---

func insecureDeployment() appsv1.Deployment {
	return appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web-app", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(3),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "web",
							Image: "nginx",
						},
					},
				},
			},
		},
	}
}

func insecureStatefulSet() appsv1.StatefulSet {
	return appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "redis", Namespace: "data"},
		Spec: appsv1.StatefulSetSpec{
			Replicas:    int32Ptr(3),
			ServiceName: "redis",
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": "redis"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "redis"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "redis",
							Image: "redis:latest",
						},
					},
				},
			},
		},
	}
}

func insecureDaemonSet() appsv1.DaemonSet {
	return appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{Name: "log-agent", Namespace: "monitoring"},
		Spec: appsv1.DaemonSetSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "log-agent"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "log-agent"}},
				Spec: corev1.PodSpec{
					HostNetwork: true,
					Containers: []corev1.Container{
						{
							Name:  "agent",
							Image: "fluentd:v1.16",
							SecurityContext: &corev1.SecurityContext{
								Privileged: boolPtr(true),
							},
						},
					},
				},
			},
		},
	}
}

func insecureJob() batchv1.Job {
	return batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{Name: "migration", Namespace: "default"},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:  "migrate",
							Image: "migrate:latest",
						},
					},
				},
			},
		},
	}
}

func insecureCronJob() batchv1.CronJob {
	return batchv1.CronJob{
		ObjectMeta: metav1.ObjectMeta{Name: "backup", Namespace: "default"},
		Spec: batchv1.CronJobSpec{
			Schedule: "0 2 * * *",
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						Spec: corev1.PodSpec{
							RestartPolicy: corev1.RestartPolicyNever,
							Containers: []corev1.Container{
								{
									Name:  "backup",
									Image: "backup-tool",
								},
							},
						},
					},
				},
			},
		},
	}
}

// --- Core Tests ---

func TestNew(t *testing.T) {
	s := newTestScanner(t)
	if s == nil {
		t.Fatal("NewWithClient() returned nil")
	}
}

func TestScanAll(t *testing.T) {
	s := newTestScanner(t, privilegedPod(), securePod(), noLimitsPod())

	result, err := s.Scan("all", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.Summary.ScannedPods < 3 {
		t.Errorf("Expected at least 3 scanned workloads, got %d", result.Summary.ScannedPods)
	}
	if result.Summary.TotalIssues == 0 {
		t.Error("Expected issues from privileged and no-limits pods")
	}
}

func TestScanNamespace(t *testing.T) {
	s := newTestScanner(t, privilegedPod(), securePod(), noLimitsPod())

	result, err := s.Scan("production", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if len(result.Issues) == 0 {
		t.Error("Expected issues in production namespace")
	}
	for _, issue := range result.Issues {
		if issue.Namespace != "production" {
			t.Errorf("Expected namespace production, got %s", issue.Namespace)
		}
	}
}

func TestScanSeverityFilter(t *testing.T) {
	s := newTestScanner(t, privilegedPod(), securePod(), noLimitsPod())

	result, _ := s.Scan("all", "critical")
	if len(result.Issues) == 0 {
		t.Error("Expected critical issues")
	}
	for _, issue := range result.Issues {
		if issue.Severity != "critical" {
			t.Errorf("Expected critical, got %s", issue.Severity)
		}
	}
}

func TestHasCritical(t *testing.T) {
	tests := []struct {
		name     string
		critical int
		expected bool
	}{
		{"no critical", 0, false},
		{"has critical", 1, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := &models.ScanResult{
				Summary: models.Summary{Critical: tt.critical},
			}
			if got := result.HasCritical(); got != tt.expected {
				t.Errorf("HasCritical() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func BenchmarkScan(b *testing.B) {
	client := fake.NewSimpleClientset()
	pod := privilegedPod()
	client.CoreV1().Pods(pod.Namespace).Create(
		context.Background(), &pod, metav1.CreateOptions{})
	s := NewWithClient(config.Default(), client, "bench-cluster")

	for i := 0; i < b.N; i++ {
		s.Scan("all", "all")
	}
}

func TestScanInvalidNamespace(t *testing.T) {
	s := newTestScanner(t, privilegedPod(), securePod())

	result, err := s.Scan("nonexistent-namespace", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if result.Summary.ScannedPods != 0 {
		t.Errorf("Expected 0 workloads in nonexistent namespace, got %d", result.Summary.ScannedPods)
	}
}

func TestScanWithHighFilter(t *testing.T) {
	s := newTestScanner(t, privilegedPod(), securePod(), noLimitsPod())

	result, _ := s.Scan("all", "high")
	for _, issue := range result.Issues {
		if issue.Severity != "high" {
			t.Errorf("Expected high severity only, got %s", issue.Severity)
		}
	}
}

func TestScanWithMediumFilter(t *testing.T) {
	s := newTestScanner(t, privilegedPod())

	result, _ := s.Scan("all", "medium")
	for _, issue := range result.Issues {
		if issue.Severity != "medium" {
			t.Errorf("Expected medium severity only, got %s", issue.Severity)
		}
	}
}

func TestCountSeverity(t *testing.T) {
	issues := []models.Issue{
		{Severity: "critical"},
		{Severity: "critical"},
		{Severity: "high"},
		{Severity: "medium"},
		{Severity: "low"},
	}

	tests := []struct {
		severity string
		expected int
	}{
		{"critical", 2},
		{"high", 1},
		{"medium", 1},
		{"low", 1},
		{"nonexistent", 0},
	}

	for _, tt := range tests {
		t.Run(tt.severity, func(t *testing.T) {
			count := countSeverity(issues, tt.severity)
			if count != tt.expected {
				t.Errorf("countSeverity(%s) = %d, want %d", tt.severity, count, tt.expected)
			}
		})
	}
}

func TestScanResultSummary(t *testing.T) {
	s := newTestScanner(t, privilegedPod(), securePod(), noLimitsPod())

	result, _ := s.Scan("all", "all")

	if result.Summary.TotalIssues != len(result.Issues) {
		t.Errorf("Summary.TotalIssues = %d, Issues = %d",
			result.Summary.TotalIssues, len(result.Issues))
	}

	total := result.Summary.Critical + result.Summary.High +
		result.Summary.Medium + result.Summary.Low
	if total != result.Summary.TotalIssues {
		t.Errorf("Sum = %d, Total = %d", total, result.Summary.TotalIssues)
	}
}

func TestClusterName(t *testing.T) {
	s := newTestScanner(t)
	result, _ := s.Scan("all", "all")
	if result.ClusterName != "test-cluster" {
		t.Errorf("Expected cluster name test-cluster, got %s", result.ClusterName)
	}
}

func TestConvertPod(t *testing.T) {
	k8sPod := privilegedPod()
	pod := convertPod(k8sPod)

	if pod.Name != "nginx-prod" {
		t.Errorf("Expected name nginx-prod, got %s", pod.Name)
	}
	if pod.ResourceKind != "Pod" {
		t.Errorf("Expected ResourceKind Pod, got %s", pod.ResourceKind)
	}
	if !pod.SecurityContext.HostPID {
		t.Error("Expected HostPID to be true")
	}
	if !pod.SecurityContext.HostNetwork {
		t.Error("Expected HostNetwork to be true")
	}
	if !pod.SecurityContext.HostIPC {
		t.Error("Expected HostIPC to be true")
	}
	if len(pod.Containers) != 1 {
		t.Fatalf("Expected 1 container, got %d", len(pod.Containers))
	}
	c := pod.Containers[0]
	if c.SecurityContext.Privileged == nil || !*c.SecurityContext.Privileged {
		t.Error("Expected privileged to be true")
	}
	if len(c.SecurityContext.Capabilities.Add) != 2 {
		t.Errorf("Expected 2 capabilities, got %d", len(c.SecurityContext.Capabilities.Add))
	}
}

func TestSecurePodNoIssues(t *testing.T) {
	s := newTestScanner(t, securePod())
	result, _ := s.Scan("all", "all")

	if result.Summary.Critical > 0 {
		t.Errorf("Secure pod should have no critical issues, got %d", result.Summary.Critical)
	}
	if result.Summary.High > 0 {
		t.Errorf("Secure pod should have no high issues, got %d", result.Summary.High)
	}
	if result.Summary.Medium > 0 {
		t.Errorf("Secure pod should have no medium issues, got %d", result.Summary.Medium)
	}
	if result.Summary.Low > 0 {
		t.Errorf("Secure pod should have no low issues, got %d", result.Summary.Low)
	}
}

// --- Workload Controller Tests ---

func TestScanDeployments(t *testing.T) {
	s := newTestScannerWithWorkloads(t, nil,
		[]appsv1.Deployment{insecureDeployment()}, nil, nil, nil, nil)

	result, err := s.Scan("all", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}
	if result.Summary.ScannedWorkloads == 0 {
		t.Error("Expected at least 1 scanned workload (deployment)")
	}

	foundDeploymentResource := false
	for _, issue := range result.Issues {
		if issue.Resource == "deployment/web-app" {
			foundDeploymentResource = true
			break
		}
	}
	if !foundDeploymentResource {
		t.Error("Expected issues with resource 'deployment/web-app'")
	}
}

func TestScanStatefulSets(t *testing.T) {
	s := newTestScannerWithWorkloads(t, nil, nil,
		[]appsv1.StatefulSet{insecureStatefulSet()}, nil, nil, nil)

	result, err := s.Scan("all", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	foundResource := false
	for _, issue := range result.Issues {
		if issue.Resource == "statefulset/redis" {
			foundResource = true
			break
		}
	}
	if !foundResource {
		t.Error("Expected issues with resource 'statefulset/redis'")
	}
}

func TestScanDaemonSets(t *testing.T) {
	s := newTestScannerWithWorkloads(t, nil, nil, nil,
		[]appsv1.DaemonSet{insecureDaemonSet()}, nil, nil)

	result, err := s.Scan("all", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	if result.Summary.Critical == 0 {
		t.Error("Expected critical issues from privileged daemonset")
	}

	foundResource := false
	for _, issue := range result.Issues {
		if issue.Resource == "daemonset/log-agent" {
			foundResource = true
			break
		}
	}
	if !foundResource {
		t.Error("Expected issues with resource 'daemonset/log-agent'")
	}
}

func TestScanJobs(t *testing.T) {
	s := newTestScannerWithWorkloads(t, nil, nil, nil, nil,
		[]batchv1.Job{insecureJob()}, nil)

	result, err := s.Scan("all", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	foundResource := false
	for _, issue := range result.Issues {
		if issue.Resource == "job/migration" {
			foundResource = true
			break
		}
	}
	if !foundResource {
		t.Error("Expected issues with resource 'job/migration'")
	}
}

func TestScanCronJobs(t *testing.T) {
	s := newTestScannerWithWorkloads(t, nil, nil, nil, nil, nil,
		[]batchv1.CronJob{insecureCronJob()})

	result, err := s.Scan("all", "all")
	if err != nil {
		t.Fatalf("Scan() error = %v", err)
	}

	foundResource := false
	for _, issue := range result.Issues {
		if issue.Resource == "cronjob/backup" {
			foundResource = true
			break
		}
	}
	if !foundResource {
		t.Error("Expected issues with resource 'cronjob/backup'")
	}
}

// --- Init Container Tests ---

func TestScanInitContainers(t *testing.T) {
	s := newTestScanner(t, podWithInitContainer())

	result, _ := s.Scan("all", "all")

	foundInitIssue := false
	for _, issue := range result.Issues {
		if issue.Severity == "critical" && issue.Message == "Container init-db runs in privileged mode" {
			foundInitIssue = true
			break
		}
	}
	if !foundInitIssue {
		t.Error("Expected critical issue from privileged init container")
	}
}

// --- New Security Check Tests ---

func TestImageTagCheck(t *testing.T) {
	s := newTestScanner(t, privilegedPod()) // uses nginx:latest

	result, _ := s.Scan("all", "high")

	foundImageIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "image" && issue.Severity == "high" {
			foundImageIssue = true
			break
		}
	}
	if !foundImageIssue {
		t.Error("Expected high severity image tag issue for :latest")
	}
}

func TestReadOnlyRootFS(t *testing.T) {
	s := newTestScanner(t, noLimitsPod()) // no readOnlyRootFilesystem

	result, _ := s.Scan("all", "medium")

	foundFSIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "filesystem" {
			foundFSIssue = true
			break
		}
	}
	if !foundFSIssue {
		t.Error("Expected medium severity filesystem issue")
	}
}

func TestDropAllCapabilities(t *testing.T) {
	s := newTestScanner(t, noLimitsPod()) // no capabilities.drop

	result, _ := s.Scan("all", "medium")

	foundCapIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "capabilities" && issue.Message == "Container mysql does not drop all capabilities" {
			foundCapIssue = true
			break
		}
	}
	if !foundCapIssue {
		t.Error("Expected medium severity drop-all-capabilities issue")
	}
}

func TestSeccompProfile(t *testing.T) {
	s := newTestScanner(t, noLimitsPod()) // no seccomp profile

	result, _ := s.Scan("all", "medium")

	foundSeccompIssue := false
	for _, issue := range result.Issues {
		if issue.Type == "seccomp" {
			foundSeccompIssue = true
			break
		}
	}
	if !foundSeccompIssue {
		t.Error("Expected medium severity seccomp issue")
	}
}

func TestServiceAccountToken(t *testing.T) {
	s := newTestScanner(t, noLimitsPod()) // automountServiceAccountToken not set

	result, _ := s.Scan("all", "medium")

	foundSAIssue := false
	for _, issue := range result.Issues {
		if issue.Message == "pod/mysql-db auto-mounts service account token" {
			foundSAIssue = true
			break
		}
	}
	if !foundSAIssue {
		t.Error("Expected service account token auto-mount issue")
	}
}

// --- Severity Filter Tests ---

func TestSeverityFilterPlus(t *testing.T) {
	s := newTestScanner(t, privilegedPod(), securePod(), noLimitsPod())

	result, _ := s.Scan("all", "high+")

	for _, issue := range result.Issues {
		if issue.Severity != "high" && issue.Severity != "critical" {
			t.Errorf("Expected high+ severity, got %s", issue.Severity)
		}
	}
	if len(result.Issues) == 0 {
		t.Error("Expected at least one high+ issue")
	}
}

func TestMatchesSeverityFilter(t *testing.T) {
	tests := []struct {
		severity string
		filter   string
		expected bool
	}{
		{"critical", "all", true},
		{"low", "all", true},
		{"critical", "critical", true},
		{"high", "critical", false},
		{"critical", "high+", true},
		{"high", "high+", true},
		{"medium", "high+", false},
		{"low", "high+", false},
		{"medium", "medium+", true},
		{"high", "medium+", true},
		{"critical", "medium+", true},
		{"low", "medium+", false},
	}

	for _, tt := range tests {
		name := tt.severity + "_" + tt.filter
		t.Run(name, func(t *testing.T) {
			got := matchesSeverityFilter(tt.severity, tt.filter)
			if got != tt.expected {
				t.Errorf("matchesSeverityFilter(%q, %q) = %v, want %v",
					tt.severity, tt.filter, got, tt.expected)
			}
		})
	}
}

// --- Resource Kind in Output ---

func TestResourceKindInOutput(t *testing.T) {
	s := newTestScannerWithWorkloads(t, nil,
		[]appsv1.Deployment{insecureDeployment()}, nil, nil, nil, nil)

	result, _ := s.Scan("all", "all")

	for _, issue := range result.Issues {
		if issue.Resource == "deployment/web-app" {
			return // Found it
		}
	}
	t.Error("Expected resource to show 'deployment/web-app'")
}

// --- Output Format Tests ---

func TestPrintResults(t *testing.T) {
	s := newTestScanner(t, privilegedPod())
	result, _ := s.Scan("all", "all")
	s.PrintResults(result)
}

func TestPrintJSON(t *testing.T) {
	s := newTestScanner(t, privilegedPod())
	result, _ := s.Scan("all", "all")

	output := captureStdout(t, func() {
		err := s.PrintJSON(result)
		if err != nil {
			t.Fatalf("PrintJSON() error = %v", err)
		}
	})

	var parsed models.ScanResult
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("PrintJSON output is not valid JSON: %v\nOutput: %s", err, output)
	}
	if len(parsed.Issues) == 0 {
		t.Error("Expected issues in JSON output")
	}
	if parsed.ClusterName != "test-cluster" {
		t.Errorf("Expected cluster name test-cluster in JSON, got %s", parsed.ClusterName)
	}
}

func TestPrintTable(t *testing.T) {
	s := newTestScanner(t, privilegedPod())
	result, _ := s.Scan("all", "all")

	output := captureStdout(t, func() {
		s.PrintTable(result)
	})

	if output == "" {
		t.Error("Expected table output")
	}
	if !bytes.Contains([]byte(output), []byte("SEVERITY")) {
		t.Error("Expected table header with SEVERITY")
	}
}

// --- Severity Helper Tests ---

func TestSeverityAtLeast(t *testing.T) {
	tests := []struct {
		issue     string
		threshold string
		expected  bool
	}{
		{"critical", "critical", true},
		{"critical", "high", true},
		{"high", "high", true},
		{"medium", "high", false},
		{"low", "critical", false},
	}

	for _, tt := range tests {
		t.Run(tt.issue+"_vs_"+tt.threshold, func(t *testing.T) {
			got := models.SeverityAtLeast(tt.issue, tt.threshold)
			if got != tt.expected {
				t.Errorf("SeverityAtLeast(%q, %q) = %v, want %v",
					tt.issue, tt.threshold, got, tt.expected)
			}
		})
	}
}
