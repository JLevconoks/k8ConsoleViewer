package main

import (
	"bytes"
	"fmt"
	"github.com/pkg/errors"
	"log"
	"os/exec"
	"strconv"
	"strings"
)

type Namespace struct {
	Name  string
	Pods  []Pod
	Error error
}

type Pod struct {
	Name     string
	Total    int // Total and ready are represented as 0/1 or 2/2 etc.
	Ready    int
	Status   string
	Restarts string
	Age      string
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
		log.Printf("stderr: %v", stderr.String())
		return Namespace{Error: errors.New(stderr.String())}
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
			podInfo, err := parsePodInfoLine(line)
			if err != nil {
				return ns, err
			}
			pods = append(pods, podInfo)
		}
	}

	ns.Pods = pods
	return ns, nil
}

func parsePodInfoLine(s string) (Pod, error) {
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
