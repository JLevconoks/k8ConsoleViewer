package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
)

type FakeK8Client struct{}

var previous []PodListResult

func (FakeK8Client) podLists(group Group) []PodListResult {

	result := make([]PodListResult, 0)
	//result := previous
	//if len(previous) == 0 {
	//	result = append(result, getPodListResult("sample-data/pods.json", "dev", "first-namespace-dev"))
	//}
	//
	//l := len(result[0].Items)
	//if l < 5 {
	//	newItem := result[0].Items[0]
	//	newItem.Name += strconv.Itoa(l)
	//	result[0].Items = append(result[0].Items, newItem)
	//	//log.Printf("New item: %v Items count %v\n", newItem.Name, len(result[0].Items))
	//} else {
	//	name := result[0].Items[3].Name
	//	if strings.HasSuffix(name, "a") {
	//		name = name[:len(name)-1] + "b"
	//	} else {
	//		name = name[:len(name)-1] + "a"
	//	}
	//	result[0].Items[3].Name = name
	//	//log.Printf("New name: %v\n", name)
	//}
	errorResult := PodListResult{
		context:   "dev",
		namespace: "error-namespace",
		error:     errors.New("Unauthorized"),
	}

	result = append(result, getPodListResult("sample-data/first.json", "dev", "first-namespace-dev"))
	result = append(result, errorResult)
	result = append(result, getPodListResult("sample-data/second.json", "dev", "second-namespace-dev"))
	result = append(result, getPodListResult("sample-data/third.json", "dev", "third-namespace-dev"))

	previous = result
	return result
}

func getPodListResult(filename, context, namespace string) PodListResult {
	bytes, err := ioutil.ReadFile(filename)
	if err != nil {
		panic(err)
	}

	var pods v1.PodList
	err = json.Unmarshal(bytes, &pods)
	if err != nil {
		panic(err)
	}

	return PodListResult{context, namespace, pods, nil}
}
