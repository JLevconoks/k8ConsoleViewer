package app

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/JLevconoks/k8ConsoleViewer/clipboard"
	"github.com/gdamore/tcell"
	"log"
	"os"
	"regexp"
	"strings"
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
	group    Group
	// This is a bit ugly, but will do for now...
	commandShortcuts map[Type]map[rune]ClipboardShortcut
}

func NewApp(context string, namespace string, settings map[string]interface{}) (App, error) {
	contextNameSet := make(map[string]struct{})
	contextNameSet[context] = struct{}{}
	k8Client, err := NewK8ClientSets(contextNameSet)
	if err != nil {
		return App{}, err
	}

	var g Group
	if strings.Contains(namespace, "*") {
		g, err = buildGroupFromWildcard(k8Client, context, namespace)
		if err != nil {
			return App{}, err
		}
	} else {
		g = buildGroup(fmt.Sprintf("%v/%v", context, namespace), context, namespace)
	}

	cs, err := getClipboardShortcuts(settings)
	if err != nil {
		return App{}, err
	}
	return App{
		k8Client:         k8Client,
		group:            g,
		commandShortcuts: cs,
	}, nil
}

func NewAppFromGroup(group Group, settings map[string]interface{}) (App, error) {
	contextNameSet := make(map[string]struct{})
	for i := range group.NsGroups {
		contextNameSet[group.NsGroups[i].Context] = struct{}{}
	}
	k8Client, err := NewK8ClientSets(contextNameSet)
	if err != nil {
		return App{}, err
	}

	cs, err := getClipboardShortcuts(settings)
	if err != nil {
		return App{}, err
	}

	return App{
		k8Client:         k8Client,
		group:            group,
		commandShortcuts: cs,
	}, nil
}

func getClipboardShortcuts(settings map[string]interface{}) (map[Type]map[rune]ClipboardShortcut, error) {
	cs, err := convertFromViperSettings(settings)
	if err != nil {
		return nil, err
	}
	// this is the case when 'clipboardShortcuts' block is not present in config.
	if cs == nil {
		cs, err = defaultClipboardShortcuts()
		if err != nil {
			return nil, err
		}
	}

	return cs, nil
}

func (app *App) Run() {
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
	gui := NewGui(s, app.group.Name)
	gui.footerFrame.clipboardShortcuts = getShortcutDisplayMap(app.commandShortcuts)
	gui.show(s)

	quit := make(chan []string)
	// Get namespace info loop.
	go func() {
		for {
			gui.statusBarCh <- "Updating namespace info..."
			startTime := time.Now()
			podListResults := app.k8Client.podLists(app.group)
			endTime := time.Now()

			errorMessages := make([]string, 0)
			for index := range podListResults {
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
		previousKeyEvent := tcell.EventKey{}
		for {
			ev := s.PollEvent()
			switch ev := ev.(type) {
			case *tcell.EventKey:
				// This is to ignore event spam primarily from mouse scroll
				if previousKeyEvent.Key() == ev.Key() && ev.When().Sub(previousKeyEvent.When()) < 5*time.Millisecond {
					break
				}
				previousKeyEvent = *ev

				switch ev.Key() {
				case tcell.KeyEscape:
					if gui.popupFrame.visible {
						gui.hidePopupFrame()
						continue
					}
					fallthrough
				case tcell.KeyCtrlC:
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
				case tcell.KeyCtrlE:
					gui.execToPods()
				case tcell.KeyCtrlL:
					gui.getLogsFromPods()
				case tcell.KeyCtrlK:
					gui.getLogsAndFollowFromPods()
				case tcell.KeyEnter:
					gui.handleEnterKey()
				}
				switch ev.Rune() {
				case 'c':
					gui.handleCollapseEvent()
				case 'e':
					gui.handleExpandEvent()
				default:
					data := gui.getCurrentGuiItemInfo()
					value, err := app.handleClipboardShortcut(ev.Rune(), data)
					if err != nil {
						gui.statusBarCh <- "Error: " + err.Error()
						continue
					}
					if value != "" {
						gui.statusBarCh <- "Clipboard: " + value
					}
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

func (app *App) handleClipboardShortcut(r rune, data GuiItemInfo) (string, error) {
	scMap, ok := app.commandShortcuts[data.itemType]
	if !ok {
		return "", nil
	}

	shortcut, ok := scMap[r]
	if !ok {
		return "", nil
	}

	var buf bytes.Buffer
	err := shortcut.template.Execute(&buf, data)
	if err != nil {
		return "", err
	}

	value := buf.String()
	if value == "" {
		return "", nil
	}

	err = clipboard.ToClipboard(value)
	if err != nil {
		return "", err
	}

	return value, nil
}

func buildGroup(groupName string, context string, namespace ...string) Group {
	return Group{
		Id:   0,
		Name: groupName,
		NsGroups: []NsGroup{
			{
				Context:    context,
				Namespaces: namespace,
			},
		},
	}
}

func buildGroupFromWildcard(k8Client Client, context string, nsNameWC string) (Group, error) {
	nsRegexString := strings.Replace(nsNameWC, "*", ".*", -1)
	nsRegexString = fmt.Sprintf("^%v$", nsRegexString)
	nsRegex, err := regexp.Compile(nsRegexString)
	if err != nil {
		return Group{}, err
	}
	nsList, err := k8Client.listNamespaces(context)
	if err != nil {
		return Group{}, err
	}

	nsNames := make([]string, 0)
	for _, ns := range nsList.Items {
		if nsRegex.MatchString(ns.Name) {
			nsNames = append(nsNames, ns.Name)
		}
	}
	if len(nsNames) == 0 {
		return Group{}, errors.New(fmt.Sprintf("no namespaces found matching '%v'", nsNameWC))
	}

	return buildGroup(fmt.Sprintf("%v/%v", context, nsNameWC), context, nsNames...), nil
}
