package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/nsf/termbox-go"
	"io/ioutil"
	"log"
	"os"
	"sort"
	"strconv"
	"time"
)

type Group struct {
	Id         int      `json:"id"`
	Name       string   `json:"name"`
	Context    string   `json:"context"`
	Namespaces []string `json:"namespaces"`
}

func main() {
	groups, err := readGroups("groups.json")

	if err != nil {
		log.Fatal(err)
	}

	args := os.Args

	if len(args) < 2 {
		fmt.Println("Group name is not provided")
		printGroups(groups)
		os.Exit(1)
	}

	userInput := args[1]

	group, err := getGroup(userInput, groups)
	if err != nil {
		log.Fatal(err)
	}

	err = termbox.Init()
	if err != nil {
		log.Panic("main.termbox.Init(): ", err)
	}
	defer func() {
		clear()
		termbox.Close()
	}()

	logToFile()

	termbox.SetInputMode(termbox.InputEsc | termbox.InputMouse)

	gui := Gui{
		Group:        group.Name,
		Context:      group.Context,
		curTopBorder: TOP_AREA_HEIGHT + 1,
	}
	gui.updateWindowSize()

	updateGuiCh := make(chan struct{})

	go func() {
		for {
			startTime := time.Now()
			namespaceInfos, _ := updateNamespaces(group)
			endTime := time.Now()

			gui.mutex.Lock()
			gui.Namespaces = namespaceInfos
			gui.TimeToExecute = endTime.Sub(startTime)
			gui.mutex.Unlock()

			updateGuiCh <- struct{}{}

			time.Sleep(5 * time.Second)
		}
	}()

	go func() {
		for range updateGuiCh {
			gui.redrawAll()
		}
	}()

	updateGuiCh <- struct{}{}

mainEventLoop:
	for {
		switch ev := termbox.PollEvent(); ev.Type {
		case termbox.EventKey:
			switch ev.Key {
			case termbox.KeyEsc, termbox.KeyCtrlC:
				break mainEventLoop
			case termbox.KeyArrowDown:
				gui.moveCursorDown()
			case termbox.KeyArrowUp:
				gui.moveCursorUp()
			}
		case termbox.EventMouse:
			switch ev.Key {
			case termbox.MouseLeft:
				gui.handleMouseKey(ev.MouseY)
			case termbox.MouseWheelDown:
				gui.moveCursorDown()
			case termbox.MouseWheelUp:
				gui.moveCursorUp()
			}
		case termbox.EventResize:
			gui.updateWindowSize()
			updateGuiCh <- struct{}{}
		case termbox.EventError:
			panic(ev.Err)
		}

	}
}

func readGroups(filepath string) ([]Group, error) {
	if _, err := os.Stat(filepath); os.IsNotExist(err) {
		return nil, fmt.Errorf("'%v' does not exist: %v\n", filepath, err)
	}

	file, err := os.Open(filepath)
	defer func() {
		err := file.Close()
		if err != nil {
			panic(fmt.Sprint("Error closing groups file: ", err))
		}
	}()

	if err != nil {
		return nil, errors.New("Error opening file: " + err.Error())
	}

	bytes, err := ioutil.ReadAll(file)

	if err != nil {
		return nil, errors.New("Error reading file: " + err.Error())
	}

	groups := make([]Group, 0)
	err = json.Unmarshal(bytes, &groups)

	if err != nil {
		return nil, errors.New("Error unmarshalling file " + err.Error())
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

func getGroup(param string, groups []Group) (*Group, error) {

	id, err := strconv.Atoi(param)

	if err != nil {
		for k := range groups {
			if groups[k].Name == param {
				return &groups[k], nil
			}
		}

		return nil, fmt.Errorf("group '%v' not found", param)
	}

	for k := range groups {
		if groups[k].Id == id {
			return &groups[k], nil
		}
	}

	return nil, fmt.Errorf("ID '%v' not found", id)
}

func logToFile() {
	file, err := os.OpenFile("log.log", os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatal("err")
	}

	log.SetOutput(file)
}
