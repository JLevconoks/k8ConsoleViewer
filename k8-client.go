package main

import (
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"os"
	"path/filepath"
	"sync"
)

type clientSetMap map[string]*kubernetes.Clientset

type Client struct {
	k8ClientSets clientSetMap
}

type K8Client interface {
	podLists(group Group) []PodListResult
}

type PodListResult struct {
	context   string
	namespace string
	v1.PodList
	error
}

func NewK8ClientSets(contexts map[string]struct{}) (Client, error) {
	configPath := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return Client{}, errors.New("No config found in ~/.kube")
	}

	k8ClientSets := make(map[string]*kubernetes.Clientset)
	for context := range contexts {
		config, err := buildConfigFromFlags(context, configPath)
		if err != nil {
			return Client{}, errors.Wrapf(err, "Error creating client config for context: %v", context)
		}

		k8client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return Client{}, errors.Wrapf(err, "Error creating clientset for context: %v", context)
		}
		k8ClientSets[context] = k8client
	}

	return Client{k8ClientSets: k8ClientSets}, nil
}

func (k8Client Client) podLists(group Group) []PodListResult {
	var wg sync.WaitGroup

	nsTotal := 0

	for i := range group.NsGroups {
		nsTotal += len(group.NsGroups[i].Namespaces)
	}
	resultCh := make(chan PodListResult, nsTotal)
	wg.Add(nsTotal)

	for gIndex := range group.NsGroups {
		for nsIndex := range group.NsGroups[gIndex].Namespaces {
			go func(ctxName, nsName string) {
				podList, err := k8Client.k8ClientSets[ctxName].CoreV1().Pods(nsName).List(metav1.ListOptions{})
				resultCh <- PodListResult{ctxName, nsName, *podList, err}
			}(group.NsGroups[gIndex].Context, group.NsGroups[gIndex].Namespaces[nsIndex])
		}
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	podListResults := make([]PodListResult, 0)
	for podListResult := range resultCh {
		podListResults = append(podListResults, podListResult)
		wg.Done()
	}
	return podListResults
}

func buildConfigFromFlags(context, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}
