package app

import (
	"fmt"
	"github.com/gdamore/tcell"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/kubernetes/pkg/util/node"
	"sort"
	"strings"
	"time"
)

type Type int

const (
	TypeNamespace Type = iota + 1
	TypePodGroup
	TypePod
	TypeContainer
	TypeNamespaceError
	TypeNamespaceMessage
)

type Item interface {
	Type() Type
	Expanded(b bool)
	IsExpanded() bool
}

type Namespace struct {
	name        string
	context     string
	deployments []*PodGroup
	nsError     NamespaceError
	nsMessage   NamespaceMessage
	isExpanded  bool
}

func (n *Namespace) Type() Type {
	return TypeNamespace
}

func (n *Namespace) Expanded(b bool) {
	n.isExpanded = b
}

func (n *Namespace) IsExpanded() bool {
	return n.isExpanded
}

func (n *Namespace) DisplayName() string {
	return fmt.Sprintf("%v / %v", n.name, n.context)
}

type PodGroup struct {
	name       string
	pods       []Pod
	isExpanded bool
	namespace  *Namespace
}

func (pg *PodGroup) Type() Type {
	return TypePodGroup
}

func (pg *PodGroup) Expanded(b bool) {
	pg.isExpanded = b
}

func (pg *PodGroup) IsExpanded() bool {
	return pg.isExpanded
}

func (pg *PodGroup) countReadyPods() (ready int) {
	ready = 0

	for pIndex := range pg.pods {
		if pg.pods[pIndex].ready == pg.pods[pIndex].total {
			ready++
		}
	}
	return ready
}

func (pg *PodGroup) podNames() []string {
	names := make([]string, 0)
	for index := range pg.pods {
		names = append(names, pg.pods[index].name)
	}
	return names
}

type Pod struct {
	name         string
	ready        int
	total        int
	status       string
	restarts     int
	age          string
	creationTime time.Time
	containers   []Container
	isExpanded   bool
	podGroup     *PodGroup
}

func (p *Pod) Type() Type {
	return TypePod
}

func (p *Pod) Expanded(b bool) {
	p.isExpanded = b
}

func (p *Pod) IsExpanded() bool {
	return p.isExpanded
}

func (p *Pod) ReadyString() string {
	return fmt.Sprintf("%d/%d", p.ready, p.total)
}

func (p *Pod) containerNames() []string {
	names := make([]string, 0)

	for index := range p.containers {
		names = append(names, p.containers[index].name)
	}

	return names
}

type Container struct {
	name       string
	image      string
	version    string
	message    string
	ready      bool
	isExpanded bool
	pod        *Pod
}

func (c Container) Type() Type {
	return TypeContainer
}

func (c Container) Expanded(b bool) {
	c.isExpanded = b
}

func (c Container) IsExpanded() bool {
	return c.isExpanded
}

func (c Container) DisplayName() string {
	return fmt.Sprintf("%v:%v", c.name, c.version)
}

type NamespaceError struct {
	error      error
	isExpanded bool
	namespace  *Namespace
}

func (nse NamespaceError) Type() Type {
	return TypeNamespaceError
}

func (nse NamespaceError) Expanded(b bool) {
	nse.isExpanded = b
}

func (nse NamespaceError) IsExpanded() bool {
	return nse.isExpanded
}

type NamespaceMessage struct {
	message    string
	isExpanded bool
	namespace  *Namespace
}

func (nsm NamespaceMessage) Type() Type {
	return TypeNamespaceMessage
}

func (nsm NamespaceMessage) Expanded(b bool) {
	nsm.isExpanded = b
}

func (nsm NamespaceMessage) IsExpanded() bool {
	return nsm.isExpanded
}

type StringItem struct {
	x, y, length int
	value        string
}

func (i *StringItem) Draw(s tcell.Screen) {
	draw(s, i.value, i.x, i.y, i.Len(s), tcell.StyleDefault)
}

func (i *StringItem) DrawS(s tcell.Screen, style tcell.Style) {
	draw(s, i.value, i.x, i.y, i.Len(s), style)
}

func (i *StringItem) Len(s tcell.Screen) int {
	if i.length == 0 {
		x, _ := s.Size()
		return x - i.x
	}
	return i.length
}

func (i *StringItem) Update(s tcell.Screen, newValue string) {
	i.value = newValue
	i.Draw(s)
}

func (i *StringItem) UpdateS(s tcell.Screen, newValue string, style tcell.Style) {
	i.value = newValue
	i.DrawS(s, style)
}

func toNamespace(plr *PodListResult) Namespace {
	ns := Namespace{
		name:    plr.namespace,
		context: plr.context,
	}
	ns.nsError = NamespaceError{
		error:     plr.error,
		namespace: &ns,
	}

	if len(plr.Items) == 0 {
		ns.nsMessage = NamespaceMessage{
			message:   "No resources found.",
			namespace: &ns,
		}
	}

	ns.deployments = toPodGroup(plr.Items, &ns)
	return ns
}

func toPodGroup(pods []v1.Pod, parent *Namespace) []*PodGroup {
	podGroup := make(map[string]*PodGroup)

	for _, pod := range pods {
		podGroupName := "_"

		switch {
		case pod.Labels["deployment"] != "":
			podGroupName = pod.Labels["deployment"]
		case pod.Labels["statefulSet"] != "":
			podGroupName = pod.Labels["statefulSet"]
		case pod.Labels["job-name"] != "":
			podGroupName = pod.Labels["job-name"]
		}

		d, ok := podGroup[podGroupName]
		if !ok {
			d = &PodGroup{
				name:       podGroupName,
				pods:       make([]Pod, 0),
				isExpanded: false,
				namespace:  parent,
			}
		}

		d.pods = append(d.pods, toPod(pod, d))
		podGroup[podGroupName] = d
	}

	podGroups := make([]*PodGroup, len(podGroup))

	index := 0
	for _, d := range podGroup {
		podGroups[index] = d
		index++
	}

	sort.Slice(podGroups, func(i, j int) bool {
		return podGroups[i].name < podGroups[j].name
	})

	return podGroups
}

func toPod(p v1.Pod, parent *PodGroup) Pod {
	pod := Pod{name: p.Name, podGroup: parent}

	status, ready, total, restarts, creationTime := podStats(&p)

	pod.status = status
	pod.ready = ready
	pod.total = total
	pod.restarts = restarts
	pod.creationTime = creationTime
	pod.age = translateTimestampSince(creationTime)

	containers := make([]Container, 0)
	for _, c := range p.Status.ContainerStatuses {
		containers = append(containers, toContainer(c, &pod))
	}

	pod.containers = containers
	return pod
}

func toContainer(cs v1.ContainerStatus, parent *Pod) Container {
	msg := ""
	if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
		msg = cs.State.Waiting.Message
	} else if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
		msg = cs.State.Terminated.Message
	}

	versionPosition := strings.LastIndex(cs.Image, ":")
	var version string
	if versionPosition > 0 {
		version = cs.Image[versionPosition+1:]
	}

	return Container{
		name:    cs.Name,
		image:   cs.Image,
		version: version,
		message: msg,
		ready:   cs.Ready,
		pod:     parent,
	}
}

// This logic is pretty much a copy of https://github.com/kubernetes/kubernetes/tree/master/pkg/printers/internalversion/printers.go
// printPod() function
func podStats(pod *v1.Pod) (status string, ready int, total int, restarts int, creationTime time.Time) {
	restarts = 0
	totalContainers := len(pod.Spec.Containers)
	readyContainers := 0

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	// TODO: Check Job pod statuses.
	//switch pod.Status.Phase {
	//case v1.PodSucceeded:
	//	row.Conditions = podSuccessConditions
	//case api.PodFailed:
	//	row.Conditions = podFailedConditions
	//}

	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
		restarts += int(container.RestartCount)
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		restarts = 0
		hasRunning := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			restarts += int(container.RestartCount)
			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else if container.Ready && container.State.Running != nil {
				hasRunning = true
				readyContainers++
			}
		}

		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if reason == "Completed" && hasRunning {
			reason = "Running"
		}
	}

	if pod.DeletionTimestamp != nil && pod.Status.Reason == node.NodeUnreachablePodReason {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	return reason, readyContainers, totalContainers, restarts, pod.CreationTimestamp.Time
}

func translateTimestampSince(timestamp time.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}
	return duration.HumanDuration(time.Since(timestamp))
}
