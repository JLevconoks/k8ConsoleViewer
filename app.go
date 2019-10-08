package main

import (
	"fmt"
	"github.com/gdamore/tcell"
	"log"
	"os"
	"time"
)

type Group struct {
	Id       int       `json:"id"`
	Name     string    `json:"name"`
	NsGroups []NsGroup `json:"nsGroups"`
}

type NsGroup struct {
	Context    string   `json:"context"`
	Namespaces []string `json:"namespaces"`
}

type App struct {
	k8Client K8Client
}

func NewApp(client K8Client) App {
	return App{
		k8Client: client,
	}
}

func (app *App) Run(group Group) {

	s, e := tcell.NewScreen()

	if e != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", e)
		os.Exit(1)
	}
	if e = s.Init(); e != nil {
		_, _ = fmt.Fprintf(os.Stderr, "%v\n", e)
		os.Exit(1)
	}

	s.Clear()
	gui := NewGui(s, group.Name)
	gui.show(s)

	quit := make(chan []string)
	// Get namespace info loop.
	go func() {
		for {
			startTime := time.Now()
			podListResults := app.k8Client.podLists(group)
			endTime := time.Now()

			errorMessages := make([]string, 0)
			for index, _ := range podListResults {
				if podListResults[index].error != nil {
					errorMessages = append(errorMessages, fmt.Sprintf("Context: %v Namespace: %v, Error: %v", podListResults[index].context, podListResults[index].namespace, podListResults[index].Error()))
				}
			}
			if len(errorMessages) == len(podListResults) {
				quit <- errorMessages
				close(quit)
			}

			gui.updateNamespaces(s, podListResults, endTime.Sub(startTime))

			time.Sleep(5 * time.Second)
		}
	}()

	go func() {
		for {
			ev := s.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				switch ev.Key() {
				case tcell.KeyEscape, tcell.KeyCtrlC:
					close(quit)
					return
				case tcell.KeyDown:
					gui.handleKeyDown()
				case tcell.KeyUp:
					gui.handleKeyUp()
				case tcell.KeyLeft:
					gui.handleKeyLeft()
				case tcell.KeyRight:
					gui.handleKeyRight()
				case tcell.KeyPgUp:
					gui.handlePageUp()
				case tcell.KeyPgDn:
					gui.handlePageDown()
				case tcell.KeyHome:
					gui.handleHomeKey()
				case tcell.KeyEnd:
					gui.handleEndKey()
				}
				switch ev.Rune() {
				case 'c':
					gui.collapseAllItems()
				case 'e':
					gui.expandAllNs()
				}

			case *tcell.EventResize:
				gui.handleResize()
			}
		}
	}()

	exitMessages := make([]string, 0)
	for s := range quit {
		exitMessages = s
	}

	s.Fini()

	log.SetOutput(os.Stdout)
	for _, s := range exitMessages {
		log.Println(s)
	}
}
