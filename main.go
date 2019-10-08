package main

import (
	"encoding/json"
	"fmt"
	"github.com/JLevconoks/k8ConsoleViewer/version"
	"github.com/pkg/errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
)

func main() {
	groups, err := readConfig()
	if err != nil {
		log.Fatal(err)
	}

	args := os.Args
	for _, s := range args {
		if s == "-v" || s == "-version" || s == "--version" {
			version.PrintVersion()
			os.Exit(0)
		}
	}

	if len(args) < 2 {
		fmt.Println("Group name is not provided")
		printGroups(groups)
		os.Exit(1)
	}

	group, err := getGroup(args[1], groups)
	if err != nil {
		log.Fatal(err)
	}

	contextNameSet := make(map[string]struct{})
	for i := range group.NsGroups {
		contextNameSet[group.NsGroups[i].Context] = struct{}{}
	}

	k8Client, err := NewK8ClientSets(contextNameSet)
	if err != nil {
		log.Fatal(err)
	}
	//logToFile()
	app := NewApp(k8Client)
	app.Run(group)
}

func logToFile() {
	appDir, err := getAppDir()
	if err != nil {
		log.Fatal(err)
	}
	file, err := os.OpenFile(appDir+"/log.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(file)
}

func readConfig() ([]Group, error) {
	appDir, err := getAppDir()
	if err != nil {
		return nil, errors.Wrap(err, "Error reading config")
	}

	configFilePath := appDir + "/groups.json"
	_, err = os.Stat(configFilePath)
	if err != nil {
		return nil, errors.Wrap(err, "File does not exist")
	}

	file, err := os.Open(configFilePath)
	defer func() {
		err := file.Close()
		if err != nil {
			panic(fmt.Sprint("Error closing groups file: ", err))

		}
	}()
	if err != nil {
		return nil, errors.Wrap(err, "Error opening file")
	}

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, errors.Wrap(err, "Error reading file")
	}

	groups := make([]Group, 0)
	err = json.Unmarshal(bytes, &groups)

	if err != nil {
		return nil, errors.Wrap(err, "Error unmarshalling file")
	}

	sort.Slice(groups, func(i, j int) bool { return groups[i].Id < groups[j].Id })

	return groups, nil
}

func printGroups(groups []Group) {
	fmt.Println("Available groups: ")
	for k := range groups {
		fmt.Printf("   %v - %v\n", groups[k].Id, groups[k].Name)
	}
}

func getGroup(param string, groups []Group) (Group, error) {
	id, err := strconv.Atoi(param)

	if err != nil {
		for k := range groups {
			if groups[k].Name == param {
				return groups[k], nil
			}
		}

		return Group{}, errors.Errorf("group '%v' not found", param)
	}

	for k := range groups {
		if groups[k].Id == id {
			return groups[k], nil
		}
	}

	return Group{}, errors.Errorf("ID '%v' not found", id)
}

func getAppDir() (string, error) {
	s, err := os.Executable()
	if err != nil {
		return "", errors.Wrapf(err, "Error in os.Executable() %v", s)
	}
	symlink, err := filepath.EvalSymlinks(s)
	if err != nil {
		return "", errors.Wrapf(err, "Error in filepath.EvalSymlinks() %v", s)
	}

	return filepath.Dir(symlink), nil
}
