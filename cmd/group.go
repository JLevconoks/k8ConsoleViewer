package cmd

import (
	"encoding/json"
	"fmt"
	"github.com/JLevconoks/k8ConsoleViewer/app"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"io/ioutil"
	"os"
	"sort"
	"strconv"
)

var groupCmd = &cobra.Command{
	Use:   "group",
	Short: "use group definition from groups.json",
	Run:   runGroupCmd,
}

func init() {
	rootCmd.AddCommand(groupCmd)
}

func runGroupCmd(cmd *cobra.Command, args []string) {
	groups, err := readGroups()
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	if len(args) < 1 {
		fmt.Println("Group name is not provided")
		printGroups(groups)
		os.Exit(1)
	}

	group, err := getGroup(args[0], groups)
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}

	k8app, err := app.NewAppFromGroup(group)
	if err != nil {
		fmt.Println(err)
		os.Exit(0)
	}
	//logToFile()
	k8app.Run()
}

func readGroups() ([]app.Group, error) {
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
		return nil, errors.Wrap(err, fmt.Sprint("Error opening file", configFilePath))
	}

	bytes, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprint("Error reading file: ", configFilePath))
	}

	groups := make([]app.Group, 0)
	err = json.Unmarshal(bytes, &groups)

	if err != nil {
		return nil, errors.Wrap(err, fmt.Sprint("Error unmarshalling file: ", configFilePath))
	}

	sort.Slice(groups, func(i, j int) bool { return groups[i].Id < groups[j].Id })

	return groups, nil
}

func printGroups(groups []app.Group) {
	fmt.Println("Available groups: ")
	for k := range groups {
		fmt.Printf("   %v - %v\n", groups[k].Id, groups[k].Name)
	}
}

func getGroup(param string, groups []app.Group) (app.Group, error) {
	id, err := strconv.Atoi(param)

	if err != nil {
		for k := range groups {
			if groups[k].Name == param {
				return groups[k], nil
			}
		}

		return app.Group{}, errors.Errorf("group '%v' not found", param)
	}

	for k := range groups {
		if groups[k].Id == id {
			return groups[k], nil
		}
	}

	return app.Group{}, errors.Errorf("ID '%v' not found", id)
}
