package app

import (
	"fmt"
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

type getPodJob struct {
	context   string
	namespace string
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
	resultCh := make(chan PodListResult)
	jobCh := make(chan getPodJob)
	workerCount := 3

	wg.Add(workerCount)
	for i := 0; i < workerCount; i++ {
		go getPods(k8Client, jobCh, resultCh, &wg)
	}

	go func() {
		for gIndex := range group.NsGroups {
			for nsIndex := range group.NsGroups[gIndex].Namespaces {
				jobCh <- getPodJob{
					context:   group.NsGroups[gIndex].Context,
					namespace: group.NsGroups[gIndex].Namespaces[nsIndex],
				}
			}
		}
		close(jobCh)
	}()
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	podListResults := make([]PodListResult, 0)
	for podListResult := range resultCh {
		podListResults = append(podListResults, podListResult)
	}
	return podListResults
}

func getPods(k8Client Client, jobCh <-chan getPodJob, resultCh chan<- PodListResult, wg *sync.WaitGroup) {
	for job := range jobCh {
		podList, err := k8Client.k8ClientSets[job.context].CoreV1().Pods(job.namespace).List(metav1.ListOptions{})
		resultCh <- PodListResult{job.context, job.namespace, *podList, err}
	}
	wg.Done()
}

func (k8Client Client) listNamespaces(context string) (*v1.NamespaceList, error) {
	fmt.Printf("Getting namespace list for context: %v \n", context)
	return k8Client.k8ClientSets[context].CoreV1().Namespaces().List(metav1.ListOptions{})
}

func buildConfigFromFlags(context, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}
