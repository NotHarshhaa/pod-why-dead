package k8s

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

// Client wraps the Kubernetes clientset.
type Client struct {
	clientset *kubernetes.Clientset
}

// NewClient creates a new Kubernetes client from kubeconfig.
func NewClient(kubeContext string) (*Client, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	if home := homedir.HomeDir(); home != "" {
		loadingRules.Precedence = append(loadingRules.Precedence, filepath.Join(home, ".kube", "config"))
	}

	overrides := &clientcmd.ConfigOverrides{}
	if kubeContext != "" {
		overrides.CurrentContext = kubeContext
	}

	config, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		loadingRules, overrides).ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create clientset: %w", err)
	}

	return &Client{clientset: clientset}, nil
}

// PodInfo holds structured information about a dead pod.
type PodInfo struct {
	Name            string
	Namespace       string
	NodeName        string
	Phase           string
	Reason          string
	Message         string
	DeadContainer   string
	Containers      []ContainerInfo
	HasPreviousLogs bool
	RestartCount    int32
	StartTime       *time.Time
	DeletionTime    *time.Time
	Conditions      []PodCondition
	NodeSelector    map[string]string
	Tolerations     []string
	AffinityRules   []string
	PVCNames        []string
	ImageDigest     string
}

// ContainerInfo holds information about a single container.
type ContainerInfo struct {
	Name           string
	Image          string
	Ready          bool
	RestartCount   int32
	ExitCode       int32
	Reason         string
	Message        string
	Signal         int32
	StartedAt      *time.Time
	FinishedAt     *time.Time
	State          string
	MemoryLimit    string
	MemoryRequest  string
	CPULimit       string
	CPURequest     string
	LivenessProbe  *ProbeInfo
	ReadinessProbe *ProbeInfo
	Command        []string
}

// ProbeInfo holds probe configuration.
type ProbeInfo struct {
	Type             string
	Path             string
	Port             string
	PeriodSeconds    int32
	FailureThreshold int32
}

// PodCondition holds a simplified pod condition.
type PodCondition struct {
	Type    string
	Status  string
	Reason  string
	Message string
	Time    time.Time
}

// NodeConditions holds the conditions of a node.
type NodeConditions struct {
	NodeName          string
	MemoryPressure    bool
	DiskPressure      bool
	PIDPressure       bool
	Ready             bool
	MemoryCapacity    string
	MemoryAllocatable string
	CPUCapacity       string
	CPUAllocatable    string
}

// DeadPodSummary is a brief summary for list mode.
type DeadPodSummary struct {
	Name      string
	Cause     string
	DeathTime time.Time
	Namespace string
	ExitCode  int32
}

// EventInfo is a simplified Kubernetes event.
type EventInfo struct {
	Time    time.Time
	Reason  string
	Message string
	Type    string
	Count   int32
}

// GetPodInfo retrieves detailed information about a pod.
func (c *Client) GetPodInfo(namespace, name string) (*PodInfo, error) {
	ctx := context.Background()
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, name, err)
	}

	info := &PodInfo{
		Name:         pod.Name,
		Namespace:    pod.Namespace,
		NodeName:     pod.Spec.NodeName,
		Phase:        string(pod.Status.Phase),
		Reason:       pod.Status.Reason,
		Message:      pod.Status.Message,
		NodeSelector: pod.Spec.NodeSelector,
	}

	// Extract tolerations
	for _, t := range pod.Spec.Tolerations {
		tolStr := t.Key
		if t.Operator == "Equal" {
			tolStr += "=" + t.Value
		}
		if t.Effect != "" {
			tolStr += ":" + string(t.Effect)
		}
		info.Tolerations = append(info.Tolerations, tolStr)
	}

	// Extract affinity rules
	if pod.Spec.Affinity != nil {
		if pod.Spec.Affinity.NodeAffinity != nil {
			for _, term := range pod.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms {
				for _, expr := range term.MatchExpressions {
					info.AffinityRules = append(info.AffinityRules,
						fmt.Sprintf("%s %s %s", expr.Key, expr.Operator, strings.Join(expr.Values, ",")))
				}
			}
		}
	}

	// Extract PVC names
	for _, vol := range pod.Spec.Volumes {
		if vol.PersistentVolumeClaim != nil {
			info.PVCNames = append(info.PVCNames, vol.PersistentVolumeClaim.ClaimName)
		}
	}

	// Extract image digest from container status
	for _, cs := range pod.Status.ContainerStatuses {
		if cs.ImageID != "" && strings.Contains(cs.ImageID, "@sha256:") {
			info.ImageDigest = cs.ImageID
			break
		}
	}

	if pod.Status.StartTime != nil {
		t := pod.Status.StartTime.Time
		info.StartTime = &t
	}
	if pod.DeletionTimestamp != nil {
		t := pod.DeletionTimestamp.Time
		info.DeletionTime = &t
	}

	for _, cond := range pod.Status.Conditions {
		info.Conditions = append(info.Conditions, PodCondition{
			Type:    string(cond.Type),
			Status:  string(cond.Status),
			Reason:  cond.Reason,
			Message: cond.Message,
			Time:    cond.LastTransitionTime.Time,
		})
	}

	for i, cs := range pod.Status.ContainerStatuses {
		ci := buildContainerInfo(cs, pod.Spec.Containers, i)
		info.Containers = append(info.Containers, ci)
		info.RestartCount += cs.RestartCount

		if cs.LastTerminationState.Terminated != nil || cs.State.Terminated != nil ||
			cs.State.Waiting != nil {
			if info.DeadContainer == "" {
				info.DeadContainer = cs.Name
				info.HasPreviousLogs = cs.RestartCount > 0 || cs.LastTerminationState.Terminated != nil
			}
		}
	}

	for i, cs := range pod.Status.InitContainerStatuses {
		ci := buildContainerInfo(cs, pod.Spec.InitContainers, i)
		ci.Name = "(init) " + ci.Name
		info.Containers = append(info.Containers, ci)
		if cs.State.Terminated != nil && cs.State.Terminated.ExitCode != 0 {
			if info.DeadContainer == "" {
				info.DeadContainer = cs.Name
				info.HasPreviousLogs = true
			}
		}
	}

	return info, nil
}

func buildContainerInfo(cs corev1.ContainerStatus, specs []corev1.Container, idx int) ContainerInfo {
	ci := ContainerInfo{
		Name:         cs.Name,
		Image:        cs.Image,
		Ready:        cs.Ready,
		RestartCount: cs.RestartCount,
	}

	if cs.State.Terminated != nil {
		ci.ExitCode = cs.State.Terminated.ExitCode
		ci.Reason = cs.State.Terminated.Reason
		ci.Message = cs.State.Terminated.Message
		ci.Signal = cs.State.Terminated.Signal
		ci.State = "terminated"
		t := cs.State.Terminated.StartedAt.Time
		ci.StartedAt = &t
		f := cs.State.Terminated.FinishedAt.Time
		ci.FinishedAt = &f
	} else if cs.LastTerminationState.Terminated != nil {
		ci.ExitCode = cs.LastTerminationState.Terminated.ExitCode
		ci.Reason = cs.LastTerminationState.Terminated.Reason
		ci.Message = cs.LastTerminationState.Terminated.Message
		ci.Signal = cs.LastTerminationState.Terminated.Signal
		ci.State = "previously-terminated"
		t := cs.LastTerminationState.Terminated.StartedAt.Time
		ci.StartedAt = &t
		f := cs.LastTerminationState.Terminated.FinishedAt.Time
		ci.FinishedAt = &f
	} else if cs.State.Waiting != nil {
		ci.Reason = cs.State.Waiting.Reason
		ci.Message = cs.State.Waiting.Message
		ci.State = "waiting"
	} else if cs.State.Running != nil {
		ci.State = "running"
		t := cs.State.Running.StartedAt.Time
		ci.StartedAt = &t
	}

	if idx < len(specs) {
		spec := specs[idx]
		ci.Command = spec.Command

		if spec.Resources.Limits.Memory() != nil {
			ci.MemoryLimit = spec.Resources.Limits.Memory().String()
		}
		if spec.Resources.Requests.Memory() != nil {
			ci.MemoryRequest = spec.Resources.Requests.Memory().String()
		}
		if spec.Resources.Limits.Cpu() != nil {
			ci.CPULimit = spec.Resources.Limits.Cpu().String()
		}
		if spec.Resources.Requests.Cpu() != nil {
			ci.CPURequest = spec.Resources.Requests.Cpu().String()
		}

		if spec.LivenessProbe != nil {
			ci.LivenessProbe = extractProbe(spec.LivenessProbe)
		}
		if spec.ReadinessProbe != nil {
			ci.ReadinessProbe = extractProbe(spec.ReadinessProbe)
		}
	}

	return ci
}

func extractProbe(p *corev1.Probe) *ProbeInfo {
	pi := &ProbeInfo{
		PeriodSeconds:    p.PeriodSeconds,
		FailureThreshold: p.FailureThreshold,
	}
	if p.HTTPGet != nil {
		pi.Type = "httpGet"
		pi.Path = p.HTTPGet.Path
		pi.Port = p.HTTPGet.Port.String()
	} else if p.TCPSocket != nil {
		pi.Type = "tcpSocket"
		pi.Port = p.TCPSocket.Port.String()
	} else if p.Exec != nil {
		pi.Type = "exec"
		pi.Path = strings.Join(p.Exec.Command, " ")
	}
	return pi
}

// GetPodEvents retrieves events for a specific pod.
func (c *Client) GetPodEvents(namespace, podName string) ([]EventInfo, error) {
	ctx := context.Background()
	fieldSelector := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName)
	eventList, err := c.clientset.CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fieldSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	var events []EventInfo
	for _, e := range eventList.Items {
		t := e.LastTimestamp.Time
		if t.IsZero() {
			t = e.EventTime.Time
		}
		if t.IsZero() {
			t = e.CreationTimestamp.Time
		}
		events = append(events, EventInfo{
			Time:    t,
			Reason:  e.Reason,
			Message: e.Message,
			Type:    e.Type,
			Count:   e.Count,
		})
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].Time.Before(events[j].Time)
	})

	return events, nil
}

// GetPreviousLogs retrieves previous container logs.
func (c *Client) GetPreviousLogs(namespace, podName, containerName string, tailLines int64) (string, error) {
	ctx := context.Background()
	opts := &corev1.PodLogOptions{
		Container: containerName,
		Previous:  true,
		TailLines: &tailLines,
	}

	req := c.clientset.CoreV1().Pods(namespace).GetLogs(podName, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to stream logs: %w", err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("failed to read log stream: %w", err)
	}

	return string(data), nil
}

// GetNodeConditions retrieves conditions and resource info for a node.
func (c *Client) GetNodeConditions(nodeName string) (*NodeConditions, error) {
	ctx := context.Background()
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	nc := &NodeConditions{
		NodeName: nodeName,
	}

	for _, cond := range node.Status.Conditions {
		switch cond.Type {
		case corev1.NodeMemoryPressure:
			nc.MemoryPressure = cond.Status == corev1.ConditionTrue
		case corev1.NodeDiskPressure:
			nc.DiskPressure = cond.Status == corev1.ConditionTrue
		case corev1.NodePIDPressure:
			nc.PIDPressure = cond.Status == corev1.ConditionTrue
		case corev1.NodeReady:
			nc.Ready = cond.Status == corev1.ConditionTrue
		}
	}

	if mem := node.Status.Capacity.Memory(); mem != nil {
		nc.MemoryCapacity = mem.String()
	}
	if mem := node.Status.Allocatable.Memory(); mem != nil {
		nc.MemoryAllocatable = mem.String()
	}
	if cpu := node.Status.Capacity.Cpu(); cpu != nil {
		nc.CPUCapacity = cpu.String()
	}
	if cpu := node.Status.Allocatable.Cpu(); cpu != nil {
		nc.CPUAllocatable = cpu.String()
	}

	return nc, nil
}

// ListDeadPods lists all dead/failed/crashlooping pods in a namespace within a duration.
func (c *Client) ListDeadPods(namespace string, since time.Duration) ([]DeadPodSummary, error) {
	ctx := context.Background()
	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	cutoff := time.Now().Add(-since)
	var results []DeadPodSummary

	for _, pod := range podList.Items {
		for _, cs := range pod.Status.ContainerStatuses {
			summary := checkContainerForDeath(pod.Name, pod.Namespace, cs, cutoff)
			if summary != nil {
				results = append(results, *summary)
				break
			}
		}

		if pod.Status.Phase == corev1.PodFailed {
			deathTime := pod.CreationTimestamp.Time
			if pod.Status.StartTime != nil {
				deathTime = pod.Status.StartTime.Time
			}
			if deathTime.After(cutoff) {
				alreadyAdded := false
				for _, r := range results {
					if r.Name == pod.Name {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					results = append(results, DeadPodSummary{
						Name:      pod.Name,
						Namespace: pod.Namespace,
						Cause:     pod.Status.Reason,
						DeathTime: deathTime,
					})
				}
			}
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].DeathTime.Before(results[j].DeathTime)
	})

	return results, nil
}

func checkContainerForDeath(podName, ns string, cs corev1.ContainerStatus, cutoff time.Time) *DeadPodSummary {
	// Check currently terminated
	if cs.State.Terminated != nil && cs.State.Terminated.FinishedAt.Time.After(cutoff) {
		return &DeadPodSummary{
			Name:      podName,
			Namespace: ns,
			Cause:     cs.State.Terminated.Reason,
			DeathTime: cs.State.Terminated.FinishedAt.Time,
			ExitCode:  cs.State.Terminated.ExitCode,
		}
	}

	// Check waiting (CrashLoopBackOff, ImagePullBackOff, etc.)
	if cs.State.Waiting != nil && isDeadWaitingReason(cs.State.Waiting.Reason) {
		deathTime := time.Now()
		if cs.LastTerminationState.Terminated != nil {
			deathTime = cs.LastTerminationState.Terminated.FinishedAt.Time
		}
		if deathTime.After(cutoff) {
			return &DeadPodSummary{
				Name:      podName,
				Namespace: ns,
				Cause:     cs.State.Waiting.Reason,
				DeathTime: deathTime,
			}
		}
	}

	// Check last termination
	if cs.LastTerminationState.Terminated != nil && cs.LastTerminationState.Terminated.FinishedAt.Time.After(cutoff) {
		return &DeadPodSummary{
			Name:      podName,
			Namespace: ns,
			Cause:     cs.LastTerminationState.Terminated.Reason,
			DeathTime: cs.LastTerminationState.Terminated.FinishedAt.Time,
			ExitCode:  cs.LastTerminationState.Terminated.ExitCode,
		}
	}

	return nil
}

func isDeadWaitingReason(reason string) bool {
	switch reason {
	case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "CreateContainerError",
		"InvalidImageName", "CreateContainerConfigError", "RunContainerError":
		return true
	}
	return false
}

// PVCInfo holds information about a Persistent Volume Claim.
type PVCInfo struct {
	Name         string
	Status       string
	VolumeName   string
	Capacity     string
	StorageClass string
	AccessModes  string
	Bound        bool
}

// GetPVCInfo retrieves PVC information for a pod.
func (c *Client) GetPVCInfo(namespace string, pvcNames []string) ([]PVCInfo, error) {
	if len(pvcNames) == 0 {
		return nil, nil
	}

	ctx := context.Background()
	var pvcs []PVCInfo

	for _, pvcName := range pvcNames {
		pvc, err := c.clientset.CoreV1().PersistentVolumeClaims(namespace).Get(ctx, pvcName, metav1.GetOptions{})
		if err != nil {
			// PVC might not exist, skip
			continue
		}

		capacity := ""
		if pvc.Status.Capacity != nil {
			if storage := pvc.Status.Capacity.Storage(); storage != nil {
				capacity = storage.String()
			}
		}

		accessModes := ""
		if len(pvc.Spec.AccessModes) > 0 {
			accessModes = string(pvc.Spec.AccessModes[0])
		}

		pvcs = append(pvcs, PVCInfo{
			Name:         pvc.Name,
			Status:       string(pvc.Status.Phase),
			VolumeName:   pvc.Spec.VolumeName,
			Capacity:     capacity,
			StorageClass: *pvc.Spec.StorageClassName,
			AccessModes:  accessModes,
			Bound:        pvc.Status.Phase == corev1.ClaimBound,
		})
	}

	return pvcs, nil
}

// NodeInfo holds detailed node information.
type NodeInfo struct {
	Name             string
	Labels           map[string]string
	Taints           []TaintInfo
	KernelVersion    string
	OSImage          string
	ContainerRuntime string
	KubeletVersion   string
}

// TaintInfo holds node taint information.
type TaintInfo struct {
	Key    string
	Value  string
	Effect string
}

// GetNodeInfo retrieves detailed node information.
func (c *Client) GetNodeInfo(nodeName string) (*NodeInfo, error) {
	ctx := context.Background()
	node, err := c.clientset.CoreV1().Nodes().Get(ctx, nodeName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get node %s: %w", nodeName, err)
	}

	var taints []TaintInfo
	for _, t := range node.Spec.Taints {
		taints = append(taints, TaintInfo{
			Key:    t.Key,
			Value:  t.Value,
			Effect: string(t.Effect),
		})
	}

	return &NodeInfo{
		Name:             node.Name,
		Labels:           node.Labels,
		Taints:           taints,
		KernelVersion:    node.Status.NodeInfo.KernelVersion,
		OSImage:          node.Status.NodeInfo.OSImage,
		ContainerRuntime: node.Status.NodeInfo.ContainerRuntimeVersion,
		KubeletVersion:   node.Status.NodeInfo.KubeletVersion,
	}, nil
}

// QuotaInfo holds resource quota information.
type QuotaInfo struct {
	Name       string
	HardCPU    string
	HardMemory string
	HardPods   string
	UsedCPU    string
	UsedMemory string
	UsedPods   string
	AtCPU      string
	AtMemory   string
	AtPods     string
}

// ReferencedResource holds information about a referenced ConfigMap or Secret.
type ReferencedResource struct {
	Name      string
	Kind      string
	Namespace string
	Exists    bool
}

// NetworkPolicyInfo holds network policy information.
type NetworkPolicyInfo struct {
	Name        string
	Namespace   string
	PodSelector []string
	Ingress     bool
	Egress      bool
}

// GetResourceQuota retrieves namespace resource quota information.
func (c *Client) GetResourceQuota(namespace string) (*QuotaInfo, error) {
	ctx := context.Background()
	quotaList, err := c.clientset.CoreV1().ResourceQuotas(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list resource quotas: %w", err)
	}

	if len(quotaList.Items) == 0 {
		return nil, nil
	}

	// Aggregate all quotas
	var q QuotaInfo
	for _, quota := range quotaList.Items {
		q.Name = quota.Name

		if hardCPU := quota.Status.Hard.Cpu(); hardCPU != nil {
			q.HardCPU = hardCPU.String()
		}
		if hardMem := quota.Status.Hard.Memory(); hardMem != nil {
			q.HardMemory = hardMem.String()
		}
		if hardPods := quota.Status.Hard.Pods(); hardPods != nil {
			q.HardPods = hardPods.String()
		}

		if usedCPU := quota.Status.Used.Cpu(); usedCPU != nil {
			q.UsedCPU = usedCPU.String()
		}
		if usedMem := quota.Status.Used.Memory(); usedMem != nil {
			q.UsedMemory = usedMem.String()
		}
		if usedPods := quota.Status.Used.Pods(); usedPods != nil {
			q.UsedPods = usedPods.String()
		}
	}

	return &q, nil
}

// ValidateReferencedResources checks if ConfigMaps and Secrets referenced by the pod exist.
func (c *Client) ValidateReferencedResources(namespace string, pod *corev1.Pod) ([]ReferencedResource, error) {
	var resources []ReferencedResource

	// Check ConfigMaps from env variables
	for _, container := range pod.Spec.Containers {
		for _, env := range container.EnvFrom {
			if env.ConfigMapRef != nil {
				exists := c.checkConfigMapExists(namespace, env.ConfigMapRef.Name)
				resources = append(resources, ReferencedResource{
					Name:      env.ConfigMapRef.Name,
					Kind:      "ConfigMap",
					Namespace: namespace,
					Exists:    exists,
				})
			}
			if env.SecretRef != nil {
				exists := c.checkSecretExists(namespace, env.SecretRef.Name)
				resources = append(resources, ReferencedResource{
					Name:      env.SecretRef.Name,
					Kind:      "Secret",
					Namespace: namespace,
					Exists:    exists,
				})
			}
		}
		for _, env := range container.Env {
			if env.ValueFrom != nil && env.ValueFrom.ConfigMapKeyRef != nil {
				exists := c.checkConfigMapExists(namespace, env.ValueFrom.ConfigMapKeyRef.Name)
				resources = append(resources, ReferencedResource{
					Name:      env.ValueFrom.ConfigMapKeyRef.Name,
					Kind:      "ConfigMap",
					Namespace: namespace,
					Exists:    exists,
				})
			}
			if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
				exists := c.checkSecretExists(namespace, env.ValueFrom.SecretKeyRef.Name)
				resources = append(resources, ReferencedResource{
					Name:      env.ValueFrom.SecretKeyRef.Name,
					Kind:      "Secret",
					Namespace: namespace,
					Exists:    exists,
				})
			}
		}
	}

	// Check ConfigMaps/Secrets from volumes
	for _, volume := range pod.Spec.Volumes {
		if volume.ConfigMap != nil {
			exists := c.checkConfigMapExists(namespace, volume.ConfigMap.Name)
			resources = append(resources, ReferencedResource{
				Name:      volume.ConfigMap.Name,
				Kind:      "ConfigMap",
				Namespace: namespace,
				Exists:    exists,
			})
		}
		if volume.Secret != nil {
			exists := c.checkSecretExists(namespace, volume.Secret.SecretName)
			resources = append(resources, ReferencedResource{
				Name:      volume.Secret.SecretName,
				Kind:      "Secret",
				Namespace: namespace,
				Exists:    exists,
			})
		}
	}

	return resources, nil
}

func (c *Client) checkConfigMapExists(namespace, name string) bool {
	ctx := context.Background()
	_, err := c.clientset.CoreV1().ConfigMaps(namespace).Get(ctx, name, metav1.GetOptions{})
	return err == nil
}

func (c *Client) checkSecretExists(namespace, name string) bool {
	ctx := context.Background()
	_, err := c.clientset.CoreV1().Secrets(namespace).Get(ctx, name, metav1.GetOptions{})
	return err == nil
}

// CheckNetworkPolicies checks if network policies might affect the pod.
func (c *Client) CheckNetworkPolicies(namespace string, podLabels map[string]string) ([]NetworkPolicyInfo, error) {
	ctx := context.Background()

	// Get network policies in the namespace
	policyList, err := c.clientset.NetworkingV1().NetworkPolicies(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list network policies: %w", err)
	}

	var policies []NetworkPolicyInfo
	for _, policy := range policyList.Items {
		// Check if this policy applies to the pod
		if policyMatchesPod(&policy, podLabels) {
			var selectors []string
			for k, v := range policy.Spec.PodSelector.MatchLabels {
				selectors = append(selectors, fmt.Sprintf("%s=%s", k, v))
			}

			policies = append(policies, NetworkPolicyInfo{
				Name:        policy.Name,
				Namespace:   namespace,
				PodSelector: selectors,
				Ingress:     policy.Spec.Ingress != nil,
				Egress:      policy.Spec.Egress != nil,
			})
		}
	}

	return policies, nil
}

func policyMatchesPod(policy *networkingv1.NetworkPolicy, podLabels map[string]string) bool {
	// If no selector, applies to all pods in namespace
	if len(policy.Spec.PodSelector.MatchLabels) == 0 && len(policy.Spec.PodSelector.MatchExpressions) == 0 {
		return true
	}

	// Check label match
	for k, v := range policy.Spec.PodSelector.MatchLabels {
		if podValue, exists := podLabels[k]; !exists || podValue != v {
			return false
		}
	}

	// For simplicity, skip complex match expressions for now
	return true
}

// GetPod retrieves the raw Pod object.
func (c *Client) GetPod(namespace, name string) (*corev1.Pod, error) {
	ctx := context.Background()
	pod, err := c.clientset.CoreV1().Pods(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get pod %s/%s: %w", namespace, name, err)
	}
	return pod, nil
}

// GetNamespacePodStats returns statistics about pods in a namespace.
func (c *Client) GetNamespacePodStats(namespace string) (map[string]int32, error) {
	ctx := context.Background()
	podList, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	stats := map[string]int32{
		"total":     0,
		"running":   0,
		"pending":   0,
		"failed":    0,
		"succeeded": 0,
		"unknown":   0,
	}

	for _, pod := range podList.Items {
		stats["total"]++
		switch pod.Status.Phase {
		case corev1.PodRunning:
			stats["running"]++
		case corev1.PodPending:
			stats["pending"]++
		case corev1.PodFailed:
			stats["failed"]++
		case corev1.PodSucceeded:
			stats["succeeded"]++
		default:
			stats["unknown"]++
		}
	}

	return stats, nil
}
