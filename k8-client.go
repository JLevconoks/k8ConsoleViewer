package main

import (
	"flag"
	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"sort"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

type Namespace struct {
	Name  string
	Pods  []Pod
	Error error
}

type Pod struct {
	Name      string
	Total     int
	Ready     int
	Status    string
	Restarts  int
	Age       string
	Namespace *Namespace
}

var k8client *kubernetes.Clientset

func NewK8ClientForContext(context string) error {
	var kubeconfig *string
	configPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return errors.New("No config found in ~/.kube")
	}
	kubeconfig = flag.String("kubeconfig", configPath, "(optional) absolute path to the kubeconfig file")

	flag.Parse()

	config, err := buildConfigFromFlags(context, *kubeconfig)
	if err != nil {
		return err
	}

	k8client, err = kubernetes.NewForConfig(config)
	if err != nil {
		return err
	}

	return nil
}

func getPods(namespace string) Namespace {
	ns := Namespace{Name: namespace}

	pods, err := k8client.CoreV1().Pods(namespace).List(metav1.ListOptions{})
	if err != nil {
		ns.Error = err
		return ns
	}

	ns.Name = namespace
	ns.Pods = make([]Pod, len(pods.Items))

	for index, podItem := range pods.Items {
		pod := Pod{
			Name:      podItem.Name,
			Status:    string(podItem.Status.Phase),
			Total:     len(podItem.Status.ContainerStatuses),
			Ready:     countReady(podItem),
			Restarts:  countRestarts(podItem),
			Age:       "_",
			Namespace: &ns,
		}

		ns.Pods[index] = pod
	}
	return ns
}

func countReady(pod v1.Pod) int {
	count := 0
	for _, s := range pod.Status.ContainerStatuses {
		if s.Ready {
			count++
		}
	}
	return count
}

func countRestarts(pod v1.Pod) int {
	count := 0
	for _, s := range pod.Status.ContainerStatuses {
		if s.Ready {
			count += int(s.RestartCount)
		}
	}
	return count
}

func updateNamespaces(g *Group) ([]Namespace, error) {
	var wg sync.WaitGroup

	nsInfoCh := make(chan Namespace, len(g.Namespaces))
	wg.Add(len(g.Namespaces))

	// Get pod info in parallel
	for i := range g.Namespaces {
		go func(nsInfoCh chan<- Namespace, ctxName, nsName string) {
			nsInfoCh <- getPods(nsName)
		}(nsInfoCh, g.Context, g.Namespaces[i])
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
