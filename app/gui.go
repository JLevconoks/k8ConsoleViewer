package app

import (
	"fmt"
	"github.com/JLevconoks/k8ConsoleViewer/clipboard"
	"github.com/JLevconoks/k8ConsoleViewer/terminal"
	"github.com/gdamore/tcell"
	"time"
)

const (
	NamespaceXOffset           = 0
	NamespaceErrorXOffset      = 2
	NamespaceMessageXOffset    = 2
	PodGroupXOffset            = 1
	PodXOffset                 = 2
	ContainerXOffset           = 4
	ColumnSpacing              = 2
	NameColumnDefaultWidth     = 25 + ColumnSpacing
	ReadyColumnDefaultWidth    = 5 + ColumnSpacing
	StatusColumnDefaultWidth   = 6 + ColumnSpacing
	RestartsColumnDefaultWidth = 8 + ColumnSpacing
	AgeColumnDefaultWidth      = 3 + ColumnSpacing
	MainFrameStartY            = 5
	FooterFrameHeight          = 4
)

type Gui struct {
	s           tcell.Screen
	currentTime StringItem
	execLabel   StringItem
	execTime    StringItem
	groupName   StringItem
	mainFrame   *InfoFrame
	footerFrame *FooterFrame
	statusBarCh chan string
}

func NewGui(s tcell.Screen, name string) Gui {
	sw, sh := s.Size()

	currentTime := StringItem{0, 0, 30, time.Now().Format(time.RFC1123Z)}
	execLabel := StringItem{currentTime.length + 3, 0, 17, "Time to execute: "}
	execTime := StringItem{execLabel.x + execLabel.length, 0, 0, "0ms"}
	groupName := StringItem{0, 1, 0, fmt.Sprintf("Group: %v", name)}

	footerFrame := NewFooterFrame(s)

	return Gui{
		s:           s,
		currentTime: currentTime,
		execLabel:   execLabel,
		execTime:    execTime,
		groupName:   groupName,
		mainFrame:   NewInfoFrame(sw, sh),
		footerFrame: footerFrame,
		statusBarCh: footerFrame.statusBarCh,
	}
}

func (gui *Gui) show(s tcell.Screen) {
	gui.currentTime.Draw(s)
	gui.execLabel.Draw(s)
	gui.execTime.Draw(s)
	gui.groupName.Draw(s)
	gui.mainFrame.namespaceHeader.Draw(s)
	gui.mainFrame.podHeader.Draw(s)
	s.Show()
}

func (gui *Gui) updateNamespaces(s tcell.Screen, podListResults []PodListResult, timeToExec time.Duration) {
	gui.mainFrame.Mutex.Lock()
	gui.mainFrame.updateNamespaces(podListResults)
	// TODO Might be worth moving timeToExec into separate struct and move this logic into a method.
	timeStyle := tcell.StyleDefault
	if timeToExec > time.Duration(1)*time.Second {
		timeStyle = timeStyle.Foreground(tcell.ColorYellow)
	}
	gui.execTime.UpdateS(s, timeToExec.String(), timeStyle)
	gui.mainFrame.refresh(s)
	gui.mainFrame.Mutex.Unlock()
	gui.updateStatusFrame()
	gui.statusBarCh <- ""
}

func (gui *Gui) handleKeyDown() {
	gui.mainFrame.moveCursor(gui.s, 1)
	gui.updateStatusFrame()
}

func (gui *Gui) handleKeyUp() {
	gui.mainFrame.moveCursor(gui.s, -1)
	gui.updateStatusFrame()
}

func (gui *Gui) handleKeyLeft() {
	gui.mainFrame.collapseCurrentItem(gui.s)
	gui.updateStatusFrame()
}

func (gui *Gui) handleKeyRight() {
	gui.mainFrame.expandCurrentItem(gui.s)
}

func (gui *Gui) handleResize() {
	winWidth, winHeight := gui.s.Size()
	gui.mainFrame.resize(gui.s, winWidth, winHeight)
	gui.footerFrame.resize(gui.s, winWidth, winHeight)
}

func (gui *Gui) handleCollapseAll() {
	gui.mainFrame.collapseAllItems(gui.s)
	gui.updateStatusFrame()
}

func (gui *Gui) handleExpandAll() {
	gui.mainFrame.expandAll(gui.s)
	gui.updateStatusFrame()
}

func (gui *Gui) handlePageUp() {
	gui.mainFrame.pageUp(gui.s)
	gui.updateStatusFrame()
}

func (gui *Gui) handlePageDown() {
	gui.mainFrame.pageDown(gui.s)
	gui.updateStatusFrame()
}

func (gui *Gui) handleHomeKey() {
	gui.mainFrame.moveCursor(gui.s, -len(gui.mainFrame.positions)-1)
}

func (gui *Gui) handleEndKey() {
	gui.mainFrame.moveCursor(gui.s, len(gui.mainFrame.positions)-1)
}

func (gui *Gui) execToPods() {
	position := gui.mainFrame.cursorFullPosition()
	item := gui.mainFrame.positions[position]

	commands := make([]string, 0)
	switch item.Type() {
	case TypeContainer:
		cont := item.(*Container)
		d := cont.pod.deployment
		pods := d.pods
		context := d.namespace.context
		nsName := d.namespace.name
		gui.statusBarCh <- fmt.Sprint("Pod Count", len(pods))
		for pIndex := range pods {
			cmdString := fmt.Sprintf("kubectl --context %v -n %v exec -it %v -c %v /bin/bash", context, nsName, pods[pIndex].name, cont.name)
			commands = append(commands, cmdString)
		}
	}
	if len(commands) > 0 {
		err := terminal.OpenAndExecute(commands)
		if err != nil {
			gui.statusBarCh <- err.Error()
		}
	}
}

func (gui *Gui) handleRune(r rune) {
	if len(gui.mainFrame.positions) == 0 {
		return
	}
	position := gui.mainFrame.cursorFullPosition()
	item := gui.mainFrame.positions[position]
	var value string
	switch item.Type() {
	case TypeNamespace:
		ns := item.(*Namespace)
		switch r {
		case '1':
			value = fmt.Sprintf("kubectl --context %v -n %v get all", ns.context, ns.name)
		case '2':
			value = fmt.Sprintf("kubectl --context %v -n %v get ingress", ns.context, ns.name)
		case '3':
			value = fmt.Sprintf("kubectl --context %v -n %v get ev --sort-by=.lastTimestamp", ns.context, ns.name)
		case '4':
			value = fmt.Sprintf("kubectl --context %v describe ns %v", ns.context, ns.name)
		case '5':
			value = fmt.Sprintf("kubectl --context %v -n %v get secrets", ns.context, ns.name)
		case '6':
			value = fmt.Sprintf("kubectl --context %v -n %v get cm", ns.context, ns.name)
		}
	case TypePodGroup:
		pg := item.(*PodGroup)
		context := pg.namespace.context
		nsName := pg.namespace.name
		switch r {
		case '1':
			value = fmt.Sprintf("kubectl --context %v -n %v describe deployment %v", context, nsName, pg.name)
		case '2':
			value = fmt.Sprintf("kubectl --context %v -n %v delete pod %v", context, nsName, pg.name)
		case '3':
			value = fmt.Sprintf("kubectl --context %v -n %v scale deployment %v --replicas=", context, nsName, pg.name)
		}
	case TypePod:
		pod := item.(*Pod)
		context := pod.deployment.namespace.context
		nsName := pod.deployment.namespace.name
		switch r {
		case '1':
			value = fmt.Sprintf("kubectl --context %v -n %v logs %v", context, nsName, pod.name)
		case '2':
			value = fmt.Sprintf("kubectl --context %v -n %v exec -it %v /bin/bash", context, nsName, pod.name)
		case '3':
			value = fmt.Sprintf("kubectl --context %v -n %v describe pod %v", context, nsName, pod.name)
		case '4':
			value = fmt.Sprintf("kubectl --context %v -n %v delete pod %v", context, nsName, pod.name)
		case '5':
			value = fmt.Sprintf("kubectl --context %v -n %v scale deployment %v --replicas=", context, nsName, pod.deployment.name)
		}
	case TypeContainer:
		cont := item.(*Container)
		context := cont.pod.deployment.namespace.context
		nsName := cont.pod.deployment.namespace.name
		switch r {
		case '1':
			value = fmt.Sprintf("kubectl --context %v -n %v logs %v -c %v", context, nsName, cont.pod.name, cont.name)
		case '2':
			value = fmt.Sprintf("kubectl --context %v -n %v exec -it %v -c %v /bin/bash", context, nsName, cont.pod.name, cont.name)
		}
	}

	if value == "" {
		return
	}
	gui.statusBarCh <- "Clipboard: " + value
	err := clipboard.ToClipboard(value)

	if err != nil {
		gui.statusBarCh <- "Error: " + err.Error()
		return
	}
}

func (gui *Gui) updateStatusFrame() {
	if len(gui.mainFrame.positions) == 0 {
		//Special case triggered by resize event being sent on app load and before positions were calculated for namespaces
		return
	}
	item := gui.mainFrame.positions[gui.mainFrame.cursorFullPosition()]
	gui.footerFrame.updateShortcutInfo(gui.s, item)
}

func drawS(s tcell.Screen, value string, x, y, length int, style tcell.Style) {
	for i := 0; i < length; i++ {
		r := ' '
		if i < len(value) {
			r = rune(value[i])
		}
		s.SetContent(i+x, y, r, nil, style)
	}
}

func draw(s tcell.Screen, value string, x, y, length int, style tcell.Style) {
	drawS(s, value, x, y, length, style)
}
