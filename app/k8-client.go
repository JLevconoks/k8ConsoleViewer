package app

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"os"
	"path/filepath"
	"sync"
)

type clientSetMap map[string]*kubernetes.Clientset

type Client struct {
	k8ClientSets clientSetMap
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
	configPath, err := configPath()
	if err != nil {
		return Client{}, err
	}

	k8ClientSets := make(map[string]*kubernetes.Clientset)
	for ctx := range contexts {
		config, err := buildConfigFromFlags(ctx, configPath)
		if err != nil {
			return Client{}, errors.Wrapf(err, "Error creating client config for context: %v", ctx)
		}

		k8client, err := kubernetes.NewForConfig(config)
		if err != nil {
			return Client{}, errors.Wrapf(err, "Error creating clientset for context: %v", ctx)
		}
		k8ClientSets[ctx] = k8client
	}

	return Client{k8ClientSets: k8ClientSets}, nil
}

func CurrentContextName() (string, error) {
	configPath, err := configPath()
	if err != nil {
		return "", err
	}

	configRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: configPath}
	config, err := configRules.Load()
	if err != nil {
		return "", errors.Wrapf(err, "error loading default config from %v", configPath)
	}
	return config.CurrentContext, nil
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
		podList, err := k8Client.k8ClientSets[job.context].CoreV1().Pods(job.namespace).List(context.Background(), metav1.ListOptions{})
		resultCh <- PodListResult{job.context, job.namespace, *podList, err}
	}
	wg.Done()
}

func (k8Client Client) listNamespaces(ctxName string) (*v1.NamespaceList, error) {
	fmt.Printf("Getting namespace list for context: %v \n", ctxName)
	return k8Client.k8ClientSets[ctxName].CoreV1().Namespaces().List(context.Background(), metav1.ListOptions{})
}

func buildConfigFromFlags(context, kubeconfigPath string) (*rest.Config, error) {
	return clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfigPath},
		&clientcmd.ConfigOverrides{
			CurrentContext: context,
		}).ClientConfig()
}

func configPath() (string, error) {
	configPath := filepath.Join(homedir.HomeDir(), ".kube", "config")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return "", errors.New(fmt.Sprintf("No config found in '%v'", configPath))
	}

	return configPath, nil
}
