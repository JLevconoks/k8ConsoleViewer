package main

import (
	"fmt"
	"github.com/gdamore/tcell"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/kubernetes/pkg/util/node"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	NamespaceXOffset           = 0
	NamespaceErrorXOffset      = 2
	NamespaceMessageXOffset    = 2
	PodXOffset                 = 2
	ContainerXOffset           = 4
	ColumnSpacing              = 2
	NameColumnDefaultWidth     = 25 + ColumnSpacing
	ReadyColumnDefaultWidth    = 5 + ColumnSpacing
	StatusColumnDefaultWidth   = 6 + ColumnSpacing
	RestartsColumnDefaultWidth = 8 + ColumnSpacing
	AgeColumnDefaultWidth      = 3 + ColumnSpacing
	MainFrameStartY            = 5
	StatusFrameHeight          = 4
)

type Type int

const (
	TypeNamespace Type = iota
	TypePod
	TypeContainer
	TypeNamespaceError
	TypeNamespaceMessage
)

type Item interface {
	Type() Type
	Expanded(b bool)
	IsExpanded() bool
}

type Namespace struct {
	name       string
	context    string
	pods       []Pod
	nsError    NamespaceError
	nsMessage  NamespaceMessage
	isExpanded bool
}

type Pod struct {
	name         string
	ready        int
	total        int
	status       string
	restarts     int
	age          string
	creationTime time.Time
	containers   []Container
	isExpanded   bool
	namespace    *Namespace
}

type Container struct {
	name       string
	image      string
	message    string
	ready      bool
	isExpanded bool
	pod        *Pod
}

type NamespaceError struct {
	error      error
	isExpanded bool
	namespace  *Namespace
}

type NamespaceMessage struct {
	message    string
	isExpanded bool
	namespace  *Namespace
}

func (n *Namespace) Type() Type {
	return TypeNamespace
}

func (n *Namespace) Expanded(b bool) {
	n.isExpanded = b
}

func (n *Namespace) IsExpanded() bool {
	return n.isExpanded
}

func (n *Namespace) FullName() string {
	return fmt.Sprintf("%v / %v", n.name, n.context)
}

func (p *Pod) Type() Type {
	return TypePod
}

func (p *Pod) Expanded(b bool) {
	p.isExpanded = b
}

func (p *Pod) IsExpanded() bool {
	return p.isExpanded
}

func (p *Pod) ReadyString() string {
	return fmt.Sprintf("%d/%d", p.ready, p.total)
}

func (c Container) Type() Type {
	return TypeContainer
}

func (c Container) Expanded(b bool) {
	c.isExpanded = b
}

func (c Container) IsExpanded() bool {
	return c.isExpanded
}

func (nse NamespaceError) Type() Type {
	return TypeNamespaceError
}

func (nse NamespaceError) Expanded(b bool) {
	nse.isExpanded = b
}

func (nse NamespaceError) IsExpanded() bool {
	return nse.isExpanded
}

func (nsm NamespaceMessage) Type() Type {
	return TypeNamespaceMessage
}

func (nsm NamespaceMessage) Expanded(b bool) {
	nsm.isExpanded = b
}

func (nsm NamespaceMessage) IsExpanded() bool {
	return nsm.isExpanded
}

type StringItem struct {
	x, y, length int
	value        string
}

func (i *StringItem) Draw(s tcell.Screen) {
	draw(s, i.value, i.x, i.y, i.Len(s))
}

func (i *StringItem) Len(s tcell.Screen) int {
	if i.length == 0 {
		x, _ := s.Size()
		return x - i.x
	}
	return i.length
}

func (i *StringItem) Update(s tcell.Screen, newValue string) {
	i.value = newValue
	i.Draw(s)
}

type Frame struct {
	x, y             int
	width, height    int
	cursorX, cursorY int
	scrollYOffset    int
	namespaceHeader  StringItem
	podHeader        StringItem
	positions        []Item
	nsItems          []Namespace
	nameColWidth     int
	readyColWidth    int
	statusColWidth   int
	restartsColWidth int
	ageColWidth      int
	sync.Mutex
}

type StatusFrame struct {
	x, y          int
	width, height int
	lines         []string
}

type Gui struct {
	s           tcell.Screen
	currentTime StringItem
	execLabel   StringItem
	execTime    StringItem
	groupName   StringItem
	mainFrame   Frame
	statusFrame StatusFrame
}

func NewGui(s tcell.Screen, name string) Gui {
	sx, sy := s.Size()

	currentTime := StringItem{0, 0, 30, time.Now().Format(time.RFC1123Z)}
	execLabel := StringItem{currentTime.length + 3, 0, 17, "Time to execute: "}
	execTime := StringItem{execLabel.x + execLabel.length, 0, 0, "0ms"}
	groupName := StringItem{0, 1, 0, fmt.Sprintf("Group: %v", name)}
	nsHeader := StringItem{NamespaceXOffset, 3, 0, "NAMESPACE / CONTEXT"}
	podHeader := StringItem{PodXOffset, 4, 0, "NAME  READY  STATUS  RESTARTS  AGE"}

	sb := StatusFrame{
		x:      0,
		y:      sy - StatusFrameHeight,
		width:  sx,
		height: StatusFrameHeight,
		lines:  make([]string, StatusFrameHeight),
	}

	frame := Frame{
		x:                0,
		y:                MainFrameStartY,
		width:            sx,
		height:           sy - MainFrameStartY - StatusFrameHeight,
		cursorX:          0,
		cursorY:          0,
		scrollYOffset:    0,
		namespaceHeader:  nsHeader,
		podHeader:        podHeader,
		positions:        []Item{},
		nsItems:          []Namespace{},
		nameColWidth:     NameColumnDefaultWidth,
		readyColWidth:    ReadyColumnDefaultWidth,
		statusColWidth:   StatusColumnDefaultWidth,
		restartsColWidth: RestartsColumnDefaultWidth,
		ageColWidth:      AgeColumnDefaultWidth,
	}

	return Gui{
		s:           s,
		currentTime: currentTime,
		execLabel:   execLabel,
		execTime:    execTime,
		groupName:   groupName,
		mainFrame:   frame,
		statusFrame: sb,
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
	gui.execTime.Update(s, timeToExec.String())
	gui.mainFrame.refresh(s)
	gui.mainFrame.Mutex.Unlock()
	gui.updateStatusBar()
}

func (gui *Gui) handleKeyDown() {
	gui.mainFrame.moveCursor(gui.s, 1)
	gui.updateStatusBar()
}

func (gui *Gui) handleKeyUp() {
	gui.mainFrame.moveCursor(gui.s, -1)
	gui.updateStatusBar()
}

func (gui *Gui) handleKeyLeft() {
	position := gui.mainFrame.cursorY + gui.mainFrame.scrollYOffset
	if len(gui.mainFrame.positions) == 0 || position >= len(gui.mainFrame.positions) {
		return
	}
	if gui.mainFrame.positions[position].IsExpanded() {
		gui.mainFrame.positions[position].Expanded(false)
		gui.mainFrame.refresh(gui.s)
	} else {
		newCursorY := gui.mainFrame.cursorY
		newOffsetY := gui.mainFrame.scrollYOffset
		for i := position; i > 0; i-- {
			if newCursorY > 0 {
				newCursorY--
			} else {
				newOffsetY--
			}
			if gui.mainFrame.positions[position].Type() > gui.mainFrame.positions[newCursorY+newOffsetY].Type() {
				break
			}
		}
		gui.mainFrame.cursorY = newCursorY
		if gui.mainFrame.scrollYOffset != newOffsetY {
			gui.mainFrame.scrollYOffset = newOffsetY
			gui.mainFrame.refresh(gui.s)
		} else {
			gui.mainFrame.updateCursor(gui.s)
		}
	}
}

func (gui *Gui) handleKeyRight() {
	position := gui.mainFrame.cursorY + gui.mainFrame.scrollYOffset
	if len(gui.mainFrame.positions) != 0 && position < len(gui.mainFrame.positions) {
		if !gui.mainFrame.positions[position].IsExpanded() {
			gui.mainFrame.positions[position].Expanded(true)
			gui.mainFrame.refresh(gui.s)
		}
	}
}

func (gui *Gui) handleResize() {
	gx, gy := gui.s.Size()
	gui.mainFrame.width = gx
	gui.mainFrame.height = gy - MainFrameStartY - StatusFrameHeight

	gui.statusFrame.y = gy - StatusFrameHeight
	gui.statusFrame.width = gx

	gui.mainFrame.refresh(gui.s)
	gui.statusFrame.update(gui.s)
}

func (gui *Gui) collapseAllItems() {
	for index, _ := range gui.mainFrame.positions {
		gui.mainFrame.positions[index].Expanded(false)
	}
	gui.mainFrame.cursorY = 0
	gui.mainFrame.scrollYOffset = 0
	gui.mainFrame.refresh(gui.s)
	gui.updateStatusBar()
}

func (gui *Gui) expandAllNs() {
	for index, _ := range gui.mainFrame.nsItems {
		gui.mainFrame.nsItems[index].Expanded(true)
	}
	gui.mainFrame.refresh(gui.s)
	gui.updateStatusBar()
}

func (gui *Gui) handlePageUp() {
	tempCursorPos := 0
	if gui.mainFrame.scrollYOffset > 0 {
		tempCursorPos = gui.mainFrame.cursorY
	}

	gui.mainFrame.moveCursor(gui.s, -gui.mainFrame.height-gui.mainFrame.cursorY)
	gui.mainFrame.cursorY = tempCursorPos
	gui.mainFrame.updateCursor(gui.s)

	gui.updateStatusBar()
}

func (gui *Gui) handlePageDown() {
	tempCursorPos := gui.mainFrame.cursorY
	gui.mainFrame.moveCursor(gui.s, 2*gui.mainFrame.height-gui.mainFrame.cursorY-1)
	if gui.mainFrame.cursorY+gui.mainFrame.scrollYOffset != len(gui.mainFrame.positions)-1 {
		gui.mainFrame.cursorY = tempCursorPos
	}
	gui.mainFrame.updateCursor(gui.s)
	gui.updateStatusBar()
}

func (gui *Gui) handleHomeKey() {
	gui.mainFrame.moveCursor(gui.s, -len(gui.mainFrame.positions)-1)
}

func (gui *Gui) handleEndKey() {
	gui.mainFrame.moveCursor(gui.s, len(gui.mainFrame.positions)-1)
}

func (gui *Gui) updateStatusBar() {
	if len(gui.mainFrame.positions) == 0 {
		//Special case triggered by resize event being sent on app load and before positions were calculated for namespaces
		return
	}
	position := gui.mainFrame.cursorY + gui.mainFrame.scrollYOffset
	item := gui.mainFrame.positions[position]
	lines := make([]string, 4)
	switch item.Type() {
	case TypeNamespace:
		ns := item.(*Namespace)
		lines[0] = fmt.Sprintf("kubectl --context %v -n %v get all", ns.context, ns.name)
		lines[1] = fmt.Sprintf("kubectl --context %v -n %v get ingress", ns.context, ns.name)
		lines[2] = fmt.Sprintf("kubectl --context %v -n %v get ev --sort-by=.lastTimestamp", ns.context, ns.name)
	case TypePod:
		pod := item.(*Pod)
		lines[0] = fmt.Sprintf("kubectl --context %v -n %v logs %v", pod.namespace.context, pod.namespace.name, pod.name)
		lines[1] = fmt.Sprintf("kubectl --context %v -n %v exec -it %v /bin/sh", pod.namespace.context, pod.namespace.name, pod.name)
	case TypeContainer:
		cont := item.(*Container)
		lines[0] = fmt.Sprintf("kubectl --context %v -n %v logs %v --container %v", cont.pod.namespace.context, cont.pod.namespace.name, cont.pod.name, cont.name)
		lines[1] = fmt.Sprintf("kubectl --context %v -n %v exec -it %v --container %v /bin/sh", cont.pod.namespace.context, cont.pod.namespace.name, cont.pod.name, cont.name)
	}

	lines[3] = "Collapse/expand namespace info: Left and Right for individual, 'c' and 'e' for all"
	gui.statusFrame.lines = lines
	gui.statusFrame.update(gui.s)
	gui.s.Show()
}

// moveCursor will move cursor n positions from current position.
func (f *Frame) moveCursor(s tcell.Screen, ny int) {
	if len(f.positions) == 0 {
		return
	}
	newCurY := f.cursorY
	newOffset := f.scrollYOffset
	if ny > 0 {
		// Moving down
		if f.cursorY+f.scrollYOffset+ny > len(f.positions)-1 {
			// if we are outside positions, change to max available movement
			ny = len(f.positions) - 1 - f.cursorY - f.scrollYOffset
		}
		if f.cursorY+ny <= f.height-1 {
			newCurY += ny
		} else {
			addToY := f.height - 1 - f.cursorY
			newCurY += addToY
			newOffset += ny - addToY
		}
	} else if ny < 0 {
		// Moving up
		ny = -ny
		if f.cursorY+f.scrollYOffset <= ny {
			newCurY = 0
			newOffset = 0
		} else if f.cursorY >= ny {
			newCurY = f.cursorY - ny
		} else {
			subFromOffset := ny - f.cursorY
			newCurY = 0
			newOffset = f.scrollYOffset - subFromOffset
		}
	}

	f.cursorY = newCurY
	f.updateCursor(s)
	if f.scrollYOffset != newOffset {
		f.scrollYOffset = newOffset
		f.refresh(s)
	}
}

func (f *Frame) refresh(s tcell.Screen) {
	f.clear(s)
	f.updatePositions()
	f.updatePodHeader(s)
	f.updateInfoFrame(s)
	f.updateCursor(s)
	//s.Show() <- part of f.updateCursor(s)
}

func (f *Frame) clear(s tcell.Screen) {
	for y := 0; y <= f.height-1; y++ {
		for x := 0; x <= f.width; x++ {
			s.SetContent(x+f.x, y+f.y, ' ', nil, tcell.StyleDefault)
		}
	}
}

// updateNamespaces will get all expanded item names, apply expanded flag on new namespaces infos and replace existing f.nsItems with
// new namespaces.
// frame positions will need to be updated straight after to avoid errors.
func (f *Frame) updateNamespaces(podListResults []PodListResult) {
	expandedNs := make(map[string]struct{}, 0)
	expandedPods := make(map[string]struct{}, 0)

	for nsIndex, _ := range f.nsItems {
		if f.nsItems[nsIndex].IsExpanded() {
			expandedNs[f.nsItems[nsIndex].FullName()] = struct{}{}
		}

		for podIndex, _ := range f.nsItems[nsIndex].pods {
			if f.nsItems[nsIndex].pods[podIndex].IsExpanded() {
				expandedPods[f.nsItems[nsIndex].pods[podIndex].name] = struct{}{}
			}
		}
	}

	newNamespaces := make([]Namespace, len(podListResults))
	for index, plr := range podListResults {
		newNamespaces[index] = toNamespace(&plr)
	}

	sort.Slice(newNamespaces, func(i, j int) bool {
		if newNamespaces[i].context > newNamespaces[j].context {
			return true
		}
		if newNamespaces[i].context < newNamespaces[j].context {
			return false
		}
		return newNamespaces[i].name < newNamespaces[j].name
	})

	for nsIndex, _ := range newNamespaces {
		_, ok := expandedNs[newNamespaces[nsIndex].FullName()]
		if ok {
			newNamespaces[nsIndex].isExpanded = true
		}

		for podIndex, _ := range newNamespaces[nsIndex].pods {
			_, ok := expandedPods[newNamespaces[nsIndex].pods[podIndex].name]
			if ok {
				newNamespaces[nsIndex].pods[podIndex].isExpanded = true
			}
		}
	}
	f.nsItems = newNamespaces
}

func (f *Frame) updatePositions() {
	positions := make([]Item, 0)
	for nsIndex := range f.nsItems {
		positions = append(positions, &f.nsItems[nsIndex])
		if f.nsItems[nsIndex].isExpanded {
			if f.nsItems[nsIndex].nsMessage.message != "" {
				positions = append(positions, &(f.nsItems[nsIndex].nsMessage))
			}
			if f.nsItems[nsIndex].nsError.error != nil {
				positions = append(positions, &(f.nsItems[nsIndex].nsError))
			}
			for pIndex := range f.nsItems[nsIndex].pods {
				positions = append(positions, &(f.nsItems[nsIndex].pods[pIndex]))

				if f.nsItems[nsIndex].pods[pIndex].isExpanded {
					for pContainer := range f.nsItems[nsIndex].pods[pIndex].containers {
						positions = append(positions, &(f.nsItems[nsIndex].pods[pIndex].containers[pContainer]))
					}
				}
			}
		}
	}
	f.positions = positions
}

func (f *Frame) updateInfoFrame(s tcell.Screen) {
	for posIndex, position := range f.positions[f.scrollYOffset:] {
		if posIndex > f.height-1 {
			break
		}
		switch position.Type() {
		case TypeNamespace:
			f.printNamespace(s, position.(*Namespace), posIndex)
		case TypeNamespaceMessage:
			f.printNamespaceMessage(s, position.(*NamespaceMessage), posIndex)
		case TypeNamespaceError:
			f.printNamespaceError(s, position.(*NamespaceError), posIndex)
		case TypePod:
			f.printPod(s, position.(*Pod), posIndex)
		case TypeContainer:
			f.printContainer(s, position.(*Container), posIndex)
		}
	}
}

func (f *Frame) printNamespace(s tcell.Screen, ns *Namespace, yPos int) {
	style := tcell.StyleDefault

	if !ns.isExpanded {
		readyCount := 0
		for podIndex, _ := range ns.pods {
			if ns.pods[podIndex].ready == ns.pods[podIndex].total {
				readyCount++
			}
		}
		if readyCount != len(ns.pods) || ns.nsError.error != nil {
			style = style.Foreground(tcell.ColorRed)
		}
		readyColPos := f.nameColWidth - NamespaceXOffset + PodXOffset
		drawS(s, ns.FullName(), NamespaceXOffset, f.y+yPos, readyColPos, style)
		drawS(s, fmt.Sprintf("%v/%v", readyCount, len(ns.pods)), readyColPos, f.y+yPos, f.width-readyColPos, style)
	} else {
		drawS(s, ns.FullName(), NamespaceXOffset, f.y+yPos, f.width-NamespaceXOffset, style)
	}
}

func (f *Frame) printNamespaceError(s tcell.Screen, nse *NamespaceError, yPos int) {
	drawS(s, nse.error.Error(), NamespaceErrorXOffset, f.y+yPos, f.width, tcell.StyleDefault.Foreground(tcell.ColorYellow))
}

func (f *Frame) printNamespaceMessage(s tcell.Screen, nse *NamespaceMessage, yPos int) {
	drawS(s, nse.message, NamespaceMessageXOffset, f.y+yPos, f.width, tcell.StyleDefault.Foreground(tcell.ColorYellow))
}

func (f *Frame) printPod(s tcell.Screen, p *Pod, yPos int) {
	running := p.status == "Running"
	style := tcell.StyleDefault
	if running && p.ready >= p.total {
		style = style.Foreground(tcell.ColorGreen)
	} else if running && p.ready < p.total {
		style = style.Foreground(tcell.ColorYellow)
	} else {
		style = style.Foreground(tcell.ColorRed)
	}

	xOffset := PodXOffset
	drawS(s, p.name, xOffset, f.y+yPos, f.nameColWidth, style)
	xOffset += f.nameColWidth
	drawS(s, p.ReadyString(), xOffset, f.y+yPos, f.readyColWidth, style)
	xOffset += f.readyColWidth
	drawS(s, p.status, xOffset, f.y+yPos, f.statusColWidth, style)
	xOffset += f.statusColWidth
	drawS(s, strconv.Itoa(p.restarts), xOffset, f.y+yPos, f.restartsColWidth, style)
	xOffset += f.restartsColWidth
	drawS(s, p.age, xOffset, f.y+yPos, f.width-xOffset, style)
}

func (f *Frame) printContainer(s tcell.Screen, c *Container, yPos int) {
	style := tcell.StyleDefault.Foreground(tcell.ColorGreen)
	if !c.ready {
		style = style.Foreground(tcell.ColorRed)
	}

	drawS(s, c.name, ContainerXOffset, f.y+yPos, f.width-ContainerXOffset, style)
}

func (f *Frame) updatePodHeader(s tcell.Screen) {
	f.nameColWidth = NameColumnDefaultWidth
	f.readyColWidth = ReadyColumnDefaultWidth
	f.statusColWidth = StatusColumnDefaultWidth
	f.restartsColWidth = RestartsColumnDefaultWidth
	f.ageColWidth = AgeColumnDefaultWidth

	for nsIndex, _ := range f.nsItems {
		if f.nameColWidth < ColumnSpacing+len(f.nsItems[nsIndex].FullName()) {
			f.nameColWidth = ColumnSpacing + len(f.nsItems[nsIndex].FullName())
		}
		if f.nsItems[nsIndex].IsExpanded() {
			for pIndex, _ := range f.nsItems[nsIndex].pods {
				if f.nameColWidth < ColumnSpacing+len(f.nsItems[nsIndex].pods[pIndex].name) {
					f.nameColWidth = ColumnSpacing + len(f.nsItems[nsIndex].pods[pIndex].name)
				}
				if f.readyColWidth < ColumnSpacing+len(f.nsItems[nsIndex].pods[pIndex].ReadyString()) {
					f.readyColWidth = ColumnSpacing + len(f.nsItems[nsIndex].pods[pIndex].ReadyString())
				}
				if f.statusColWidth < ColumnSpacing+len(f.nsItems[nsIndex].pods[pIndex].status) {
					f.statusColWidth = ColumnSpacing + len(f.nsItems[nsIndex].pods[pIndex].status)
				}
				if f.restartsColWidth < ColumnSpacing+len(strconv.Itoa(f.nsItems[nsIndex].pods[pIndex].restarts)) {
					f.restartsColWidth = ColumnSpacing + len(strconv.Itoa(f.nsItems[nsIndex].pods[pIndex].restarts))
				}
				if f.ageColWidth < ColumnSpacing+len(f.nsItems[nsIndex].pods[pIndex].age) {
					f.ageColWidth = ColumnSpacing + len(f.nsItems[nsIndex].pods[pIndex].age)
				}
			}
		}
	}

	toPrint :=
		"NAME" + strings.Repeat(" ", f.nameColWidth-4) +
			"READY" + strings.Repeat(" ", f.readyColWidth-5) +
			"STATUS" + strings.Repeat(" ", f.statusColWidth-6) +
			"RESTARTS" + strings.Repeat(" ", f.restartsColWidth-8) +
			"AGE"
	f.podHeader.Update(s, toPrint)
}

func (f *Frame) updateCursor(s tcell.Screen) {
	if f.cursorY+f.scrollYOffset > len(f.positions)-1 {
		diff := f.cursorY + f.scrollYOffset - (len(f.positions) - 1)
		f.moveCursor(s, -diff)
	}
	s.ShowCursor(f.x+f.cursorX, f.y+f.cursorY)
	s.Show()
}

func (sb *StatusFrame) update(s tcell.Screen) {
	for k, v := range sb.lines {
		drawS(s, v, 0, sb.y+k, sb.width, tcell.StyleDefault)
	}
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

func draw(s tcell.Screen, value string, x, y, length int) {
	drawS(s, value, x, y, length, tcell.StyleDefault)
}

func toNamespace(plr *PodListResult) Namespace {
	ns := Namespace{
		name:    plr.namespace,
		context: plr.context,
	}
	ns.nsError = NamespaceError{
		error:     plr.error,
		namespace: &ns,
	}

	if len(plr.Items) == 0 {
		ns.nsMessage = NamespaceMessage{
			message:   "No resources found.",
			namespace: &ns,
		}
	}

	pods := make([]Pod, 0)
	for _, p := range plr.Items {
		pods = append(pods, toPod(p, &ns))
	}

	ns.pods = pods
	return ns
}

func toPod(p v1.Pod, parent *Namespace) Pod {
	pod := Pod{name: p.Name, namespace: parent}

	status, ready, total, restarts, creationTime := podStats(&p)

	pod.status = status
	pod.ready = ready
	pod.total = total
	pod.restarts = restarts
	pod.creationTime = creationTime
	pod.age = translateTimestampSince(creationTime)

	containers := make([]Container, 0)
	for _, c := range p.Status.ContainerStatuses {
		containers = append(containers, toContainer(c, &pod))
	}

	pod.containers = containers
	return pod
}

func toContainer(cs v1.ContainerStatus, parent *Pod) Container {
	msg := ""
	if cs.State.Waiting != nil && cs.State.Waiting.Reason != "" {
		msg = cs.State.Waiting.Message
	} else if cs.State.Terminated != nil && cs.State.Terminated.Reason != "" {
		msg = cs.State.Terminated.Message
	}
	return Container{
		name:    cs.Name,
		image:   cs.Image,
		message: msg,
		ready:   cs.Ready,
		pod:     parent,
	}
}

// This logic is pretty much a copy of https://github.com/kubernetes/kubernetes/tree/master/pkg/printers/internalversion/printers.go
// printPod() function
func podStats(pod *v1.Pod) (status string, ready int, total int, restarts int, creationTime time.Time) {
	restarts = 0
	totalContainers := len(pod.Spec.Containers)
	readyContainers := 0

	reason := string(pod.Status.Phase)
	if pod.Status.Reason != "" {
		reason = pod.Status.Reason
	}

	// TODO: Check Job pod statuses.
	//switch pod.Status.Phase {
	//case v1.PodSucceeded:
	//	row.Conditions = podSuccessConditions
	//case api.PodFailed:
	//	row.Conditions = podFailedConditions
	//}

	initializing := false
	for i := range pod.Status.InitContainerStatuses {
		container := pod.Status.InitContainerStatuses[i]
		restarts += int(container.RestartCount)
		switch {
		case container.State.Terminated != nil && container.State.Terminated.ExitCode == 0:
			continue
		case container.State.Terminated != nil:
			// initialization is failed
			if len(container.State.Terminated.Reason) == 0 {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Init:Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("Init:ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else {
				reason = "Init:" + container.State.Terminated.Reason
			}
			initializing = true
		case container.State.Waiting != nil && len(container.State.Waiting.Reason) > 0 && container.State.Waiting.Reason != "PodInitializing":
			reason = "Init:" + container.State.Waiting.Reason
			initializing = true
		default:
			reason = fmt.Sprintf("Init:%d/%d", i, len(pod.Spec.InitContainers))
			initializing = true
		}
		break
	}
	if !initializing {
		restarts = 0
		hasRunning := false
		for i := len(pod.Status.ContainerStatuses) - 1; i >= 0; i-- {
			container := pod.Status.ContainerStatuses[i]

			restarts += int(container.RestartCount)
			if container.State.Waiting != nil && container.State.Waiting.Reason != "" {
				reason = container.State.Waiting.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason != "" {
				reason = container.State.Terminated.Reason
			} else if container.State.Terminated != nil && container.State.Terminated.Reason == "" {
				if container.State.Terminated.Signal != 0 {
					reason = fmt.Sprintf("Signal:%d", container.State.Terminated.Signal)
				} else {
					reason = fmt.Sprintf("ExitCode:%d", container.State.Terminated.ExitCode)
				}
			} else if container.Ready && container.State.Running != nil {
				hasRunning = true
				readyContainers++
			}
		}

		// change pod status back to "Running" if there is at least one container still reporting as "Running" status
		if reason == "Completed" && hasRunning {
			reason = "Running"
		}
	}

	if pod.DeletionTimestamp != nil && pod.Status.Reason == node.NodeUnreachablePodReason {
		reason = "Unknown"
	} else if pod.DeletionTimestamp != nil {
		reason = "Terminating"
	}

	return reason, readyContainers, totalContainers, restarts, pod.CreationTimestamp.Time
}

func translateTimestampSince(timestamp time.Time) string {
	if timestamp.IsZero() {
		return "<unknown>"
	}
	return duration.HumanDuration(time.Since(timestamp))
}
