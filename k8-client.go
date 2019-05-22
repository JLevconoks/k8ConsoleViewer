package main

import (
	"fmt"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/pkg/util/node"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

var k8Clientsets map[string]*kubernetes.Clientset

func NewK8ClientSets(contexts map[string]struct{}) error {
	configPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return errors.New("No config found in ~/.kube")
	}

	k8Clientsets = make(map[string]*kubernetes.Clientset)
	for context := range contexts {
		config, err := buildConfigFromFlags(context, configPath)
		if err != nil {
			return err
		}

		k8client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return err
		}
		k8Clientsets[context] = k8client
	}

	return nil
}

func getPods(context, namespace string) Namespace {
	ns := Namespace{Name: namespace, Context: context}

	pods, err := k8Clientsets[context].CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		ns.Error = err
		return ns
	}

	ns.Name = namespace
	ns.Pods = make([]Pod, 0)

	for i, p := range pods.Items {
		if p.Status.Phase == "Succeeded" {
			continue
		}
		pod := Pod{
			Name:      p.Name,
			Status:    getStatus(&pods.Items[i]),
			Total:     len(p.Status.ContainerStatuses),
			Ready:     countReady(&pods.Items[i]),
			Restarts:  countRestarts(&pods.Items[i]),
			Age:       translateTimeSince(&p.CreationTimestamp),
			Namespace: &ns,
		}

		containers := make([]Container, len(p.Spec.Containers))
		for i, cs := range p.Status.ContainerStatuses {

			msg := ""
			if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
				msg = cs.State.Waiting.Message
			} else if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
				msg = cs.State.Terminated.Message
			}

			cont := Container{
				Name:    cs.Name,
				Image:   cs.Image,
				Ready:   cs.Ready,
				Message: msg,
				Pod:     &pod,
			}
			containers[i] = cont
		}
		pod.Containers = containers
		ns.Pods = append(ns.Pods, pod)
	}
	return ns
}

func countReady(p *v1.Pod) int {
	count := 0
	for _, s := range p.Status.ContainerStatuses {
		if s.Ready {
			count++
		}
	}
	return count
}

func countRestarts(p *v1.Pod) int {
	count := 0
	for _, s := range p.Status.ContainerStatuses {
		count += int(s.RestartCount)
	}
	return count
}

func updateNamespaces(g *Group) ([]Namespace, error) {
	var wg sync.WaitGroup

	nsTotal := 0

	for i := range g.NsGroups {
		nsTotal += len(g.NsGroups[i].Namespaces)
	}
	nsInfoCh := make(chan Namespace, nsTotal)
	wg.Add(nsTotal)

	// Get pod info in parallel
	for gi := range g.NsGroups {
		for i := range g.NsGroups[gi].Namespaces {
			go func(nsInfoCh chan<- Namespace, ctxName, nsName string) {
				nsInfoCh <- getPods(ctxName, nsName)
			}(nsInfoCh, g.NsGroups[gi].Context, g.NsGroups[gi].Namespaces[i])
		}
	}

	// Wait for everything to finish and collect pod info into one slice
	go func() {
		wg.Wait()
		close(nsInfoCh)
	}()

	nsInfos := make([]Namespace, 0)
	for nsInfo := range nsInfoCh {
		nsInfo.sortPods()
		nsInfos = append(nsInfos, nsInfo)
		wg.Done()
	}
	sort.Slice(nsInfos, func(i, j int) bool { return nsInfos[i].Name < nsInfos[j].Name })
	return nsInfos, nil
}

func (ns *Namespace) sortPods() {
	sort.Slice(ns.Pods, func(i, j int) bool {
		return ns.Pods[i].Name < ns.Pods[j].Name
	})
}

func buildConfigFromFlags(context, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}

// Translate time to human readable duration using k8s package.
func translateTimeSince(t *metav1.Time) string {
	if t == nil || t.IsZero() {
		return "<unknown>"
	}
	return duration.ShortHumanDuration(time.Since(t.Time))
}

// This logic is pretty much a copy of https://github.com/kubernetes/kubernetes/tree/master/pkg/printers/internalversion/printers.go
// printPod() function
func getStatus(pod *v1.Pod) string {
	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
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
		hasRunning := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

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

	return reason
}
