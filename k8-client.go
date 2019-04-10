package main

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"log"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type Namespace struct {
	Name  string
	Pods  []Pod
	Error error
}

type Pod struct {
	Name      string
	Total     int // Total and ready are represented as 0/1 or 2/2 etc.
	Ready     int
	Status    string
	Restarts  string
	Age       string
	Namespace *Namespace
}

func getPods(context, namespace string) Namespace {
	//TODO: Figure out how to test exec.Command.
	cmd := exec.Command("kubectl", fmt.Sprintf("--context=%v", context), fmt.Sprintf("-n=%v", namespace), "get", "pods")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		log.Printf("cmd.Run() failed with %v\n", err)
	}

	if len(stderr.Bytes()) != 0 {
		return Namespace{Name: namespace, Error: errors.New(stderr.String())}
	}

	ns, err := processPodResponse(namespace, stdout.Bytes())

	if err != nil {
		return Namespace{Error: err}
	}

	return ns
}

func processPodResponse(namespace string, bytes []byte) (Namespace, error) {
	ns := Namespace{Name: namespace}

	lineSplit := strings.Split(string(bytes), "\n")

	if !strings.HasPrefix(lineSplit[0], "NAME") {
		return ns, errors.New(fmt.Sprintf("Pod Response does not start with header string.\n Actual:\n%v ", string(bytes)))
	}

	pods := make([]Pod, 0)
	for _, line := range lineSplit[1:] {
		if len(strings.TrimSpace(line)) > 0 {
			pod, err := parsePodLine(line)
			if err != nil {
				return ns, err
			}
			pod.Namespace = &ns
			pods = append(pods, pod)
		}
	}

	ns.Pods = pods
	return ns, nil
}

func parsePodLine(s string) (Pod, error) {
	cleanSplit := cleanSplit(s)

	if len(cleanSplit) != 5 {
		return Pod{}, errors.New(fmt.Sprintf("Splitting pod info line `%v` brought %v values\n", s, len(cleanSplit)))
	}

	ready, total, err := splitReadyString(cleanSplit[1])
	if err != nil {
		return Pod{}, err
	}
	pod := Pod{
		Name:     cleanSplit[0],
		Ready:    ready,
		Total:    total,
		Status:   cleanSplit[2],
		Restarts: cleanSplit[3],
		Age:      cleanSplit[4],
	}
	return pod, nil
}

// Splits Ready section of the Pod info output which is in "2/2" or "1/2" format
func splitReadyString(s string) (ready, total int, err error) {
	split := strings.Split(s, "/")
	ready, err = strconv.Atoi(split[0])
	if err != nil {
		return ready, total, errors.New(fmt.Sprintf("error converting Ready from string: '%v' '%v'", s, err))
	}
	total, err = strconv.Atoi(split[1])
	if err != nil {
		return ready, total, errors.New(fmt.Sprintf("error converting Total from string: '%v' '%v'", s, err))
	}

	return ready, total, nil
}

// Splits the output line removing blank spaces.
func cleanSplit(input string) []string {
	var cleanSplit []string

	for _, v := range strings.Split(input, " ") {
		v = strings.TrimSpace(v)
		if len(v) != 0 {
			cleanSplit = append(cleanSplit, v)
		}
	}

	return cleanSplit
}

func updateNamespaces(g *Group) ([]Namespace, error) {
	var wg sync.WaitGroup

	nsInfoCh := make(chan Namespace, len(g.Namespaces))
	wg.Add(len(g.Namespaces))

	// Get pod info in parallel
	for i := range g.Namespaces {
		go func(nsInfoCh chan<- Namespace, ctxName, nsName string) {
			nsInfoCh <- getPods(ctxName, nsName)
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
