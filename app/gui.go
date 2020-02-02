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
	popupFrame  *PopupFrame
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
		popupFrame:  NewPopupFrame(s, "", nil, nil),
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
	gui.redraw(s)
	gui.mainFrame.Mutex.Unlock()
	gui.statusBarCh <- ""
}

func (gui *Gui) redraw(s tcell.Screen) {
	gui.mainFrame.refresh(s)
	gui.updateStatusFrame()
	if gui.popupFrame != nil && gui.popupFrame.visible {
		gui.popupFrame.show(s)
	}
	s.Show()
}

func (gui *Gui) handleKeyDown() {
	if gui.popupFrame.visible {
		gui.popupFrame.moveCursorDown(gui.s)
	} else {
		gui.mainFrame.moveCursor(gui.s, 1)
		gui.updateStatusFrame()
	}

	gui.s.Show()
}

func (gui *Gui) handleKeyUp() {
	if gui.popupFrame.visible {
		gui.popupFrame.moveCursorUp(gui.s)
	} else {
		gui.mainFrame.moveCursor(gui.s, -1)
		gui.updateStatusFrame()
	}

	gui.s.Show()
}

func (gui *Gui) handleKeyLeft() {
	gui.mainFrame.collapseCurrentItem(gui.s)
	gui.updateStatusFrame()
	gui.s.Show()
}

func (gui *Gui) handleKeyRight() {
	gui.mainFrame.expandCurrentItem(gui.s)
	gui.s.Show()
}

func (gui *Gui) handleResize() {
	winWidth, winHeight := gui.s.Size()
	gui.mainFrame.resize(gui.s, winWidth, winHeight)
	gui.footerFrame.resize(gui.s, winWidth, winHeight)
	gui.s.Show()
}

func (gui *Gui) handleCollapseAll() {
	gui.mainFrame.collapseAllItems(gui.s)
	gui.updateStatusFrame()
	gui.s.Show()
}

func (gui *Gui) handleExpandAll() {
	gui.mainFrame.expandAll(gui.s)
	gui.updateStatusFrame()
	gui.s.Show()
}

func (gui *Gui) handlePageUp() {
	gui.mainFrame.pageUp(gui.s)
	gui.updateStatusFrame()
	gui.s.Show()
}

func (gui *Gui) handlePageDown() {
	gui.mainFrame.pageDown(gui.s)
	gui.updateStatusFrame()
	gui.s.Show()
}

func (gui *Gui) handleHomeKey() {
	gui.mainFrame.moveCursor(gui.s, -len(gui.mainFrame.positions)-1)
	gui.s.Show()
}

func (gui *Gui) handleEndKey() {
	gui.mainFrame.moveCursor(gui.s, len(gui.mainFrame.positions)-1)
	gui.s.Show()
}

func (gui *Gui) hidePopupFrame() {
	gui.popupFrame.visible = false
	gui.redraw(gui.s)
}

func (gui *Gui) handleEnterKey() {
	if gui.popupFrame.visible {
		selected := gui.popupFrame.items[gui.popupFrame.cursorYPos]
		gui.popupFrame.callback(selected)
		gui.popupFrame.visible = false
		gui.redraw(gui.s)
	}
}

func (gui *Gui) execToPods() {
	cmdTemplate := "kubectl --context %v -n %v exec -it %v -c %v /bin/bash"
	gui.handleCommandExec(cmdTemplate)
}

func (gui *Gui) getLogsFromPods() {
	cmdTemplate := "kubectl --context %v -n %v logs %v -c %v"
	gui.handleCommandExec(cmdTemplate)
}

func (gui *Gui) getLogsAndFollowFromPods() {
	cmdTemplate := "kubectl --context %v -n %v logs %v -c %v -f"
	gui.handleCommandExec(cmdTemplate)
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
		context := pod.podGroup.namespace.context
		nsName := pod.podGroup.namespace.name
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
			value = fmt.Sprintf("kubectl --context %v -n %v scale deployment %v --replicas=", context, nsName, pod.podGroup.name)
		}
	case TypeContainer:
		cont := item.(*Container)
		context := cont.pod.podGroup.namespace.context
		nsName := cont.pod.podGroup.namespace.name
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

func (gui *Gui) handleCommandExec(tmpl string) {
	// TODO need to do something better regarding this check.
	if len(gui.mainFrame.positions) == 0 {
		return
	}

	position := gui.mainFrame.cursorFullPosition()
	item := gui.mainFrame.positions[position]

	context, nsName, podNames, contNames := gatherContainerInfos(item)

	popupCallback := func(selected string) {
		commands := assembleCommands(tmpl, context, nsName, selected, podNames)
		if len(commands) > 0 {
			err := terminal.OpenAndExecute(commands)
			if err != nil {
				gui.statusBarCh <- err.Error()
			}
		}
	}
	gui.popupFrame = NewPopupFrame(gui.s, "Container", contNames, popupCallback)
	gui.popupFrame.visible = true
	gui.popupFrame.show(gui.s)
	gui.s.Show()
}

func gatherContainerInfos(item Item) (context, nsName string, podNames, contNames []string) {
	switch item.Type() {
	case TypePodGroup:
		pg := item.(*PodGroup)
		context = pg.namespace.context
		nsName = pg.namespace.name
		podNames = pg.podNames()
		// TODO check for empty slice
		contNames = pg.pods[0].containerNames()
	case TypePod:
		p := item.(*Pod)
		context = p.podGroup.namespace.context
		nsName = p.podGroup.namespace.name
		podNames = p.podGroup.podNames()
		contNames = p.containerNames()
	case TypeContainer:
		c := item.(*Container)
		context = c.pod.podGroup.namespace.context
		nsName = c.pod.podGroup.namespace.name
		podNames = c.pod.podGroup.podNames()
		contNames = c.pod.containerNames()
	}

	return context, nsName, podNames, contNames
}

func assembleCommands(tmpl, context, nsName, contName string, pods []string) []string {
	commands := make([]string, 0)

	for _, podName := range pods {
		cmdString := fmt.Sprintf(tmpl, context, nsName, podName, contName)
		commands = append(commands, cmdString)
	}

	return commands
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
