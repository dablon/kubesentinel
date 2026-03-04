package scanner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dablon/kubesentinel/internal/config"
	"github.com/dablon/kubesentinel/pkg/models"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var Verbose bool

type Scanner struct {
	config      *config.Config
	clientset   kubernetes.Interface
	clusterName string
}

// New creates a Scanner connected to a real Kubernetes cluster.
func New(cfg *config.Config) (*Scanner, error) {
	var restConfig *rest.Config
	var err error
	clusterName := "unknown"

	kubeconfigPath := cfg.Kubeconfig

	if kubeconfigPath != "" {
		restConfig, err = clientcmd.BuildConfigFromFlags("", kubeconfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to build config from kubeconfig %q: %w", kubeconfigPath, err)
		}
		clusterName = getClusterName(kubeconfigPath)
	} else {
		restConfig, err = rest.InClusterConfig()
		if err != nil {
			loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
			configOverrides := &clientcmd.ConfigOverrides{}
			kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

			restConfig, err = kubeConfig.ClientConfig()
			if err != nil {
				return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
			}

			rawConfig, rawErr := kubeConfig.RawConfig()
			if rawErr == nil && rawConfig.CurrentContext != "" {
				clusterName = rawConfig.CurrentContext
			}
		} else {
			clusterName = "in-cluster"
		}
	}

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &Scanner{
		config:      cfg,
		clientset:   clientset,
		clusterName: clusterName,
	}, nil
}

// NewWithClient creates a Scanner with a provided clientset (for testing).
func NewWithClient(cfg *config.Config, client kubernetes.Interface, clusterName string) *Scanner {
	return &Scanner{
		config:      cfg,
		clientset:   client,
		clusterName: clusterName,
	}
}

func getClusterName(kubeconfigPath string) string {
	config, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return "unknown"
	}
	if config.CurrentContext != "" {
		return config.CurrentContext
	}
	return "unknown"
}

func verboseLog(format string, args ...interface{}) {
	if Verbose {
		fmt.Fprintf(os.Stderr, "[verbose] "+format+"\n", args...)
	}
}

// --- Scan ---

func (s *Scanner) Scan(namespace, severityFilter string) (*models.ScanResult, error) {
	ctx := context.Background()

	result := &models.ScanResult{
		Timestamp:   time.Now().UTC().Format(time.RFC3339),
		ClusterName: s.clusterName,
		Issues:      []models.Issue{},
		Summary:     models.Summary{},
	}

	workloads, err := s.getWorkloads(ctx, namespace)
	if err != nil {
		return nil, fmt.Errorf("failed to list workloads: %w", err)
	}

	verboseLog("Found %d workloads to scan", len(workloads))

	for i := range workloads {
		issues := s.scanPod(&workloads[i])
		for _, issue := range issues {
			if matchesSeverityFilter(issue.Severity, severityFilter) {
				result.Issues = append(result.Issues, issue)
			}
		}
	}

	result.Summary = models.Summary{
		TotalIssues:      len(result.Issues),
		Critical:         countSeverity(result.Issues, "critical"),
		High:             countSeverity(result.Issues, "high"),
		Medium:           countSeverity(result.Issues, "medium"),
		Low:              countSeverity(result.Issues, "low"),
		ScannedPods:      len(workloads),
		ScannedWorkloads: len(workloads),
	}

	return result, nil
}

// --- Workload Fetching ---

func (s *Scanner) getWorkloads(ctx context.Context, namespace string) ([]models.Pod, error) {
	var allPods []models.Pod
	ns := namespace
	if ns == "all" || ns == "" {
		ns = ""
	}

	// Standalone Pods (skip pods owned by a controller to avoid duplicates)
	podList, err := s.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	standalone := 0
	for _, p := range podList.Items {
		if isOwnedByController(p) {
			continue
		}
		standalone++
		allPods = append(allPods, convertPod(p))
	}
	verboseLog("Found %d pods (%d standalone, %d controller-managed)", len(podList.Items), standalone, len(podList.Items)-standalone)

	// Deployments
	deployments, err := s.clientset.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
	if err == nil {
		verboseLog("Found %d deployments", len(deployments.Items))
		for _, d := range deployments.Items {
			allPods = append(allPods, convertPodTemplateSpec(
				d.Spec.Template, d.Namespace, "Deployment", d.Name))
		}
	}

	// StatefulSets
	statefulsets, err := s.clientset.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
	if err == nil {
		verboseLog("Found %d statefulsets", len(statefulsets.Items))
		for _, ss := range statefulsets.Items {
			allPods = append(allPods, convertPodTemplateSpec(
				ss.Spec.Template, ss.Namespace, "StatefulSet", ss.Name))
		}
	}

	// DaemonSets
	daemonsets, err := s.clientset.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
	if err == nil {
		verboseLog("Found %d daemonsets", len(daemonsets.Items))
		for _, ds := range daemonsets.Items {
			allPods = append(allPods, convertPodTemplateSpec(
				ds.Spec.Template, ds.Namespace, "DaemonSet", ds.Name))
		}
	}

	// Jobs
	jobs, err := s.clientset.BatchV1().Jobs(ns).List(ctx, metav1.ListOptions{})
	if err == nil {
		verboseLog("Found %d jobs", len(jobs.Items))
		for _, j := range jobs.Items {
			allPods = append(allPods, convertPodTemplateSpec(
				j.Spec.Template, j.Namespace, "Job", j.Name))
		}
	}

	// CronJobs
	cronjobs, err := s.clientset.BatchV1().CronJobs(ns).List(ctx, metav1.ListOptions{})
	if err == nil {
		verboseLog("Found %d cronjobs", len(cronjobs.Items))
		for _, cj := range cronjobs.Items {
			allPods = append(allPods, convertPodTemplateSpec(
				cj.Spec.JobTemplate.Spec.Template, cj.Namespace, "CronJob", cj.Name))
		}
	}

	return allPods, nil
}

// --- Conversion ---

func convertPod(p corev1.Pod) models.Pod {
	pod := models.Pod{
		Name:         p.Name,
		Namespace:    p.Namespace,
		ResourceKind: "Pod",
		ResourceName: p.Name,
	}

	pod.SecurityContext.HostPID = p.Spec.HostPID
	pod.SecurityContext.HostNetwork = p.Spec.HostNetwork
	pod.SecurityContext.HostIPC = p.Spec.HostIPC
	pod.SecurityContext.AutomountServiceAccountToken = p.Spec.AutomountServiceAccountToken

	for _, c := range p.Spec.Containers {
		pod.Containers = append(pod.Containers, convertContainer(c))
	}
	for _, c := range p.Spec.InitContainers {
		pod.InitContainers = append(pod.InitContainers, convertContainer(c))
	}

	return pod
}

func convertPodTemplateSpec(template corev1.PodTemplateSpec, namespace, kind, name string) models.Pod {
	pod := models.Pod{
		Name:         name,
		Namespace:    namespace,
		ResourceKind: kind,
		ResourceName: name,
	}

	pod.SecurityContext.HostPID = template.Spec.HostPID
	pod.SecurityContext.HostNetwork = template.Spec.HostNetwork
	pod.SecurityContext.HostIPC = template.Spec.HostIPC
	pod.SecurityContext.AutomountServiceAccountToken = template.Spec.AutomountServiceAccountToken

	for _, c := range template.Spec.Containers {
		pod.Containers = append(pod.Containers, convertContainer(c))
	}
	for _, c := range template.Spec.InitContainers {
		pod.InitContainers = append(pod.InitContainers, convertContainer(c))
	}

	return pod
}

func convertContainer(c corev1.Container) models.Container {
	container := models.Container{
		Name:            c.Name,
		Image:           c.Image,
		ImagePullPolicy: string(c.ImagePullPolicy),
	}

	if sc := c.SecurityContext; sc != nil {
		container.SecurityContext.Privileged = sc.Privileged
		container.SecurityContext.RunAsNonRoot = sc.RunAsNonRoot
		container.SecurityContext.ReadOnlyRootFilesystem = sc.ReadOnlyRootFilesystem

		if sc.RunAsUser != nil {
			uid := int(*sc.RunAsUser)
			container.SecurityContext.RunAsUser = &uid
		}

		if sc.Capabilities != nil {
			for _, cap := range sc.Capabilities.Add {
				container.SecurityContext.Capabilities.Add = append(
					container.SecurityContext.Capabilities.Add, string(cap))
			}
			for _, cap := range sc.Capabilities.Drop {
				container.SecurityContext.Capabilities.Drop = append(
					container.SecurityContext.Capabilities.Drop, string(cap))
			}
		}

		if sc.SeccompProfile != nil {
			container.SecurityContext.SeccompProfile = string(sc.SeccompProfile.Type)
		}
	}

	if c.Resources.Limits != nil {
		container.Resources.Limits = make(map[string]string)
		for k, v := range c.Resources.Limits {
			container.Resources.Limits[string(k)] = v.String()
		}
	}

	return container
}

// --- Security Checks ---

func (s *Scanner) scanPod(pod *models.Pod) []models.Issue {
	var issues []models.Issue
	resourceRef := fmt.Sprintf("%s/%s", strings.ToLower(pod.ResourceKind), pod.Name)

	verboseLog("Scanning %s in namespace %s", resourceRef, pod.Namespace)

	// Pod-level checks
	if pod.SecurityContext.HostPID {
		issues = append(issues, models.Issue{
			Type: "privilege", Severity: "high",
			Message:     "Pod has hostPID enabled",
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Disable hostPID",
		})
	}

	if pod.SecurityContext.HostNetwork {
		issues = append(issues, models.Issue{
			Type: "network", Severity: "medium",
			Message:     "Pod has hostNetwork enabled",
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Disable hostNetwork unless required",
		})
	}

	if pod.SecurityContext.HostIPC {
		issues = append(issues, models.Issue{
			Type: "privilege", Severity: "high",
			Message:     "Pod has hostIPC enabled",
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Disable hostIPC",
		})
	}

	if pod.SecurityContext.AutomountServiceAccountToken == nil || *pod.SecurityContext.AutomountServiceAccountToken {
		issues = append(issues, models.Issue{
			Type: "privilege", Severity: "medium",
			Message:     fmt.Sprintf("%s auto-mounts service account token", resourceRef),
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Set automountServiceAccountToken: false unless needed",
		})
	}

	// Container-level checks
	for _, container := range pod.Containers {
		issues = append(issues, s.checkContainer(pod, container, resourceRef)...)
	}
	for _, container := range pod.InitContainers {
		issues = append(issues, s.checkContainer(pod, container, resourceRef)...)
	}

	return issues
}

func (s *Scanner) checkContainer(pod *models.Pod, container models.Container, resourceRef string) []models.Issue {
	var issues []models.Issue

	verboseLog("  Checking container %s", container.Name)

	// 1. Running as root
	if container.SecurityContext.RunAsNonRoot == nil || !*container.SecurityContext.RunAsNonRoot {
		issues = append(issues, models.Issue{
			Type: "privilege", Severity: "high",
			Message:     fmt.Sprintf("Container %s runs as root", container.Name),
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Set runAsNonRoot: true and runAsUser > 0",
		})
	}

	// 2. Privileged mode
	if container.SecurityContext.Privileged != nil && *container.SecurityContext.Privileged {
		issues = append(issues, models.Issue{
			Type: "privilege", Severity: "critical",
			Message:     fmt.Sprintf("Container %s runs in privileged mode", container.Name),
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Set privileged: false",
		})
	}

	// 3. Dangerous capabilities
	if len(container.SecurityContext.Capabilities.Add) > 0 {
		dangerousCaps := []string{"SYS_ADMIN", "NET_ADMIN", "DAC_READ_SEARCH", "SYS_RAWIO"}
		var found []string
		for _, cap := range container.SecurityContext.Capabilities.Add {
			for _, dangerous := range dangerousCaps {
				if cap == dangerous {
					found = append(found, cap)
					break
				}
			}
		}
		if len(found) > 0 {
			issues = append(issues, models.Issue{
				Type: "capabilities", Severity: "medium",
				Message:     fmt.Sprintf("Container %s has dangerous capabilities: %v", container.Name, found),
				Namespace:   pod.Namespace, Resource: resourceRef,
				Remediation: "Drop all capabilities and add only what's needed",
			})
		}
	}

	// 4. Not dropping ALL capabilities
	hasDropAll := false
	for _, cap := range container.SecurityContext.Capabilities.Drop {
		if cap == "ALL" {
			hasDropAll = true
			break
		}
	}
	if !hasDropAll {
		issues = append(issues, models.Issue{
			Type: "capabilities", Severity: "medium",
			Message:     fmt.Sprintf("Container %s does not drop all capabilities", container.Name),
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Add capabilities.drop: [\"ALL\"] and only add back needed capabilities",
		})
	}

	// 5. Read-only root filesystem
	if container.SecurityContext.ReadOnlyRootFilesystem == nil || !*container.SecurityContext.ReadOnlyRootFilesystem {
		issues = append(issues, models.Issue{
			Type: "filesystem", Severity: "medium",
			Message:     fmt.Sprintf("Container %s does not have read-only root filesystem", container.Name),
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Set readOnlyRootFilesystem: true",
		})
	}

	// 6. Image tag check
	if strings.HasSuffix(container.Image, ":latest") || !strings.Contains(container.Image, ":") {
		issues = append(issues, models.Issue{
			Type: "image", Severity: "high",
			Message:     fmt.Sprintf("Container %s uses latest or untagged image: %s", container.Name, container.Image),
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Pin image to a specific version tag or digest",
		})
	}

	// 7. Seccomp profile
	if container.SecurityContext.SeccompProfile == "" || container.SecurityContext.SeccompProfile == "Unconfined" {
		issues = append(issues, models.Issue{
			Type: "seccomp", Severity: "medium",
			Message:     fmt.Sprintf("Container %s has no seccomp profile or uses Unconfined", container.Name),
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Set seccompProfile.type to RuntimeDefault or Localhost",
		})
	}

	// 8. Image pull policy
	if container.ImagePullPolicy != "" && container.ImagePullPolicy != string(corev1.PullAlways) {
		issues = append(issues, models.Issue{
			Type: "image", Severity: "low",
			Message:     fmt.Sprintf("Container %s has imagePullPolicy: %s", container.Name, container.ImagePullPolicy),
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Set imagePullPolicy: Always to prevent using stale cached images",
		})
	}

	// 9. Missing resource limits
	if container.Resources.Limits == nil {
		issues = append(issues, models.Issue{
			Type: "resources", Severity: "low",
			Message:     fmt.Sprintf("Container %s has no resource limits", container.Name),
			Namespace:   pod.Namespace, Resource: resourceRef,
			Remediation: "Set resource limits",
		})
	}

	return issues
}

// --- Severity Filtering ---

func matchesSeverityFilter(issueSeverity, filter string) bool {
	if filter == "all" {
		return true
	}
	if strings.HasSuffix(filter, "+") {
		threshold := strings.TrimSuffix(filter, "+")
		return models.SeverityAtLeast(issueSeverity, threshold)
	}
	return issueSeverity == filter
}

// --- Output ---

func (s *Scanner) PrintResults(result *models.ScanResult) {
	sevIcon := map[string]string{"critical": "🔴", "high": "🟠", "medium": "🟡", "low": "🟢"}
	sevBar := func(c, h, m, l int) string {
		parts := []string{}
		if c > 0 {
			parts = append(parts, fmt.Sprintf("🔴 %d critical", c))
		}
		if h > 0 {
			parts = append(parts, fmt.Sprintf("🟠 %d high", h))
		}
		if m > 0 {
			parts = append(parts, fmt.Sprintf("🟡 %d medium", m))
		}
		if l > 0 {
			parts = append(parts, fmt.Sprintf("🟢 %d low", l))
		}
		return strings.Join(parts, "  ")
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════╗")
	fmt.Println("║          KubeSentinel Security Scan              ║")
	fmt.Println("╚══════════════════════════════════════════════════╝")
	fmt.Printf("  Cluster:   %s\n", result.ClusterName)
	fmt.Printf("  Time:      %s\n", result.Timestamp)
	fmt.Printf("  Workloads: %d scanned\n", result.Summary.ScannedWorkloads)
	fmt.Println()
	fmt.Printf("  %s\n", sevBar(result.Summary.Critical, result.Summary.High, result.Summary.Medium, result.Summary.Low))
	fmt.Printf("  Total: %d issues\n", result.Summary.TotalIssues)

	if len(result.Issues) == 0 {
		fmt.Println("\n  ✅ No issues found!")
		return
	}

	// Group issues by namespace, then by resource
	type resourceKey struct {
		Namespace string
		Resource  string
	}
	order := []resourceKey{}
	grouped := map[resourceKey][]models.Issue{}
	for _, issue := range result.Issues {
		key := resourceKey{issue.Namespace, issue.Resource}
		if _, exists := grouped[key]; !exists {
			order = append(order, key)
		}
		grouped[key] = append(grouped[key], issue)
	}

	// Sort order: by namespace
	currentNS := ""
	for _, key := range order {
		if key.Namespace != currentNS {
			currentNS = key.Namespace
			fmt.Printf("\n┌─ namespace: %s\n", currentNS)
		}

		issues := grouped[key]
		c := countSeverity(issues, "critical")
		h := countSeverity(issues, "high")
		m := countSeverity(issues, "medium")
		l := countSeverity(issues, "low")

		fmt.Printf("│\n│  📦 %s  [%s]\n", key.Resource, sevBar(c, h, m, l))

		for _, issue := range issues {
			icon := sevIcon[issue.Severity]
			fmt.Printf("│     %s %s\n", icon, issue.Message)
			if Verbose && issue.Remediation != "" {
				fmt.Printf("│        ↳ Fix: %s\n", issue.Remediation)
			}
		}
	}
	fmt.Println("└")

	if !Verbose {
		fmt.Println("\n  💡 Use -v to see remediation details")
	}
}

func (s *Scanner) PrintJSON(result *models.ScanResult) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func (s *Scanner) PrintTable(result *models.ScanResult) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "SEVERITY\tRESOURCE\tNAMESPACE\tTYPE\tMESSAGE")
	for _, issue := range result.Issues {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			strings.ToUpper(issue.Severity), issue.Resource, issue.Namespace, issue.Type, issue.Message)
	}
	w.Flush()
}

// --- Helpers ---

// isOwnedByController returns true if the pod is managed by a higher-level controller
// (ReplicaSet, DaemonSet, StatefulSet, Job). We skip these pods because we scan the
// controller's PodTemplateSpec directly, avoiding duplicate findings.
func isOwnedByController(p corev1.Pod) bool {
	controllerKinds := map[string]bool{
		"ReplicaSet":  true,
		"DaemonSet":   true,
		"StatefulSet": true,
		"Job":         true,
	}
	for _, ref := range p.OwnerReferences {
		if controllerKinds[ref.Kind] {
			return true
		}
	}
	return false
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
