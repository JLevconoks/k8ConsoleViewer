package main

import (
	"fmt"
	"github.com/nsf/termbox-go"
	"log"
	"strconv"
	"sync"
	"time"
)

const (
	NameColStartWidth   = 20
	StatusColStartWidth = 8
	ReadyColWidth       = 7
	RestartsColWidth    = 10
	StatusAreaHeight    = 5
	InfoAreaStart       = 5
	TopAreaHeight       = 4
)

type Gui struct {
	group           string
	namespaces      []Namespace
	timeToExecute   time.Duration
	height          int
	width           int
	curX            int
	curY            int
	curTopBorder    int
	curBottomBorder int
	positions       Positions
	nameWidth       int
	statusWidth     int
	scrollOffset    int
	mutex           sync.Mutex
	nsCollapsed     map[string]bool
	podExpanded     map[string]bool
}

type Positions struct {
	namespaces map[int]*Namespace
	pods       map[int]*Pod
	containers map[int]*Container
	errors     map[int]string
	message    map[int]string
	lastIndex  int
}

type Namespace struct {
	Name    string
	Context string
	Pods    []Pod
	Error   error
}

type Pod struct {
	Name       string
	Ready      int
	Total      int
	Status     string
	Restarts   int
	Age        string
	Containers []Container
	Namespace  *Namespace
}

type Container struct {
	Name    string
	Image   string
	Message string
	Ready   bool
	Pod     *Pod
}

func (gui *Gui) redrawAll() {
	clear()
	gui.printHeaders()
	gui.printMainInfo()
	gui.adjustCursorPosition()
	gui.printStatusArea()
	flush()
}

func (gui *Gui) handleLeftArrow() {
	index := gui.curY - InfoAreaStart + gui.scrollOffset
	// When pod is collapsed or on error line, move to namespace line
	if gui.positions.hasPod(index) && !gui.podExpanded[gui.positions.pods[index].Name] || gui.positions.hasError(index) {
		for !gui.positions.hasNamespace(index) {
			index--
		}
		if index < gui.scrollOffset {
			gui.scrollOffset = index
			gui.redrawAll()
		}

		gui.moveCursor(0, index+InfoAreaStart-gui.scrollOffset)
		// When on container or message line then move to pod line
	} else if gui.positions.hasContainer(index) || gui.positions.hasMessage(index) {
		for !gui.positions.hasPod(index) {
			index--
		}
		if index < gui.scrollOffset {
			gui.scrollOffset = index
			gui.redrawAll()
		}
		gui.moveCursor(0, index+InfoAreaStart-gui.scrollOffset)
	} else if gui.positions.hasNamespace(index) && !gui.nsCollapsed[gui.positions.namespaces[index].displayName()] {
		gui.collapseNamespace(index)
	} else if gui.positions.hasPod(index) && gui.podExpanded[gui.positions.pods[index].Name] {
		gui.mutex.Lock()
		gui.podExpanded[gui.positions.pods[index].Name] = false
		gui.updatePositions()
		gui.mutex.Unlock()
		gui.redrawAll()
	}
}

func (gui *Gui) handleRightArrow() {
	// TODO when already expanded, move one step lower (make sure when expanding on bottom border is handled correctly)
	index := gui.curY - InfoAreaStart + gui.scrollOffset
	if gui.positions.hasPod(index) && !gui.podExpanded[gui.positions.pods[index].Name] {
		gui.mutex.Lock()
		gui.podExpanded[gui.positions.pods[index].Name] = true
		gui.updatePositions()
		gui.mutex.Unlock()
		gui.redrawAll()
	} else if gui.positions.hasNamespace(index) && gui.nsCollapsed[gui.positions.namespaces[index].displayName()] {
		gui.mutex.Lock()
		gui.nsCollapsed[gui.positions.namespaces[index].displayName()] = false
		gui.updatePositions()
		gui.mutex.Unlock()
		gui.redrawAll()
	}
}

func (gui *Gui) collapseNamespace(index int) {
	gui.mutex.Lock()
	gui.nsCollapsed[gui.positions.namespaces[index].displayName()] = true
	gui.updatePositions()
	gui.mutex.Unlock()
	gui.redrawAll()
}

func (gui *Gui) collapseAllNS() {
	gui.mutex.Lock()
	for _, ns := range gui.positions.namespaces {
		gui.nsCollapsed[ns.displayName()] = true
	}
	gui.updatePositions()
	gui.mutex.Unlock()

	gui.cursorToStartPos()
	gui.redrawAll()
}

func (gui *Gui) expandAllNS() {
	gui.mutex.Lock()
	for _, ns := range gui.positions.namespaces {
		gui.nsCollapsed[ns.displayName()] = false
	}
	gui.updatePositions()
	gui.mutex.Unlock()

	gui.cursorToStartPos()
	gui.redrawAll()
}

func (gui *Gui) moveCursorDown() {
	if gui.curY < gui.curBottomBorder {
		gui.moveCursor(gui.curX, gui.curY+1)
	} else {
		gui.mutex.Lock()
		gui.scrollOffset++
		gui.mutex.Unlock()
		gui.redrawAll()
	}
}

func (gui *Gui) moveCursorUp() {
	if gui.curY > gui.curTopBorder {
		gui.moveCursor(gui.curX, gui.curY-1)
	} else {
		if gui.scrollOffset > 0 {
			gui.mutex.Lock()
			gui.scrollOffset--
			gui.mutex.Unlock()
			gui.redrawAll()
		}
	}
}

func (gui *Gui) moveCursor(x, y int) {
	gui.mutex.Lock()
	gui.curX = x
	gui.curY = y
	gui.mutex.Unlock()
	gui.redrawCursor()
}

func (gui *Gui) redrawCursor() {
	termbox.SetCursor(gui.curX, gui.curY)
	gui.printStatusArea()
	flush()
}

func (gui *Gui) adjustCursorPosition() {
	if gui.curY > gui.curBottomBorder {
		gui.moveCursor(gui.curX, gui.curBottomBorder)
	} else if gui.curY < gui.curTopBorder {
		gui.moveCursor(gui.curX, gui.curTopBorder)
	}
}

func (gui *Gui) printHeaders() {
	printDefaultLine(fmt.Sprintf("%v    Time to execute: %v\n", time.Now().Format(time.RFC1123Z), gui.timeToExecute.String()), 0, 0)
	printDefaultLine(fmt.Sprintf("Group: %v", gui.group), 0, 1)
	printDefaultLine("NAMESPACE / CONTEXT", 0, 3)
	printDefaultLine("NAME", 3, 4)
	printDefaultLine("READY", gui.nameWidth, 4)
	printDefaultLine("STATUS", gui.nameWidth+ReadyColWidth, 4)
	printDefaultLine("RESTARTS", gui.nameWidth+ReadyColWidth+gui.statusWidth, 4)
	printDefaultLine("AGE", gui.nameWidth+ReadyColWidth+gui.statusWidth+RestartsColWidth, 4)
}

func (gui *Gui) printStatusArea() {
	index := gui.curY - InfoAreaStart + gui.scrollOffset
	printDefaultLine("Collapse/expand namespace info: Left and Right for individual, 'c' and 'e' for all", 0, gui.height-1)
	if gui.positions.hasNamespace(index) {
		ns := gui.positions.namespaces[index]

		all := fmt.Sprintf("kubectl --context %v -n %v get all", ns.Context, ns.Name)
		ingress := fmt.Sprintf("kubectl --context %v -n %v get ingress", ns.Context, ns.Name)
		events := fmt.Sprintf("kubectl --context %v -n %v get ev --sort-by=.lastTimestamp", ns.Context, ns.Name)

		clearStatusArea()
		printDefaultLine(all, 0, gui.height-4)
		printDefaultLine(ingress, 0, gui.height-3)
		printDefaultLine(events, 0, gui.height-2)
	} else if gui.positions.hasPod(index) {
		pod := gui.positions.pods[index]

		podLog := fmt.Sprintf("kubectl --context %v -n %v logs %v", pod.Namespace.Context, pod.Namespace.Name, pod.Name)
		exec := fmt.Sprintf("kubectl --context %v -n %v exec -it %v /bin/sh", pod.Namespace.Context, pod.Namespace.Name, pod.Name)

		clearStatusArea()
		printDefaultLine(podLog, 0, gui.height-4)
		printDefaultLine(exec, 0, gui.height-3)
	} else if gui.positions.hasContainer(index) {
		cont := gui.positions.containers[index]

		contLog := fmt.Sprintf("kubectl --context %v -n %v logs %v --container %v", cont.Pod.Namespace.Context, cont.Pod.Namespace.Name, cont.Pod.Name, cont.Name)
		exec := fmt.Sprintf("kubectl --context %v -n %v exec -it %v --container %v /bin/sh", cont.Pod.Namespace.Context, cont.Pod.Namespace.Name, cont.Pod.Name, cont.Name)

		clearStatusArea()
		printDefaultLine(contLog, 0, gui.height-4)
		printDefaultLine(exec, 0, gui.height-3)
	} else {
		clearStatusArea()
	}
}

func (gui *Gui) printMainInfo() {
	offset := gui.scrollOffset
	yPosition := InfoAreaStart
	for gui.positions.hasNamespace(offset) || gui.positions.hasPod(offset) || gui.positions.hasContainer(offset) || gui.positions.hasMessage(offset) || gui.positions.hasError(offset) {
		if yPosition < gui.height-StatusAreaHeight {
			if gui.positions.hasNamespace(offset) {
				gui.printNamespace(offset, yPosition)
			}
			if gui.positions.hasPod(offset) {
				gui.positions.pods[offset].printPodInfo(yPosition, gui.nameWidth, gui.statusWidth)
			}
			if gui.positions.hasContainer(offset) {
				gui.positions.containers[offset].printContainerInfo(yPosition)
			}
			if gui.positions.hasMessage(offset) {
				printLine(gui.positions.message[offset], 9, yPosition, termbox.ColorYellow, termbox.ColorDefault)
			}
			if gui.positions.hasError(offset) {
				printLine(gui.positions.errors[offset], 3, yPosition, termbox.ColorYellow, termbox.ColorDefault)
			}

			offset++
			yPosition++
		} else {
			break
		}
	}
	gui.mutex.Lock()
	gui.curBottomBorder = yPosition - 1
	gui.mutex.Unlock()
}

func (gui *Gui) printNamespace(nsIndex, yPosition int) {
	displayName := gui.positions.namespaces[nsIndex].displayName()
	if gui.nsCollapsed[displayName] {
		totalSum := 0
		readySum := 0
		for i := range gui.positions.namespaces[nsIndex].Pods {
			totalSum += gui.positions.namespaces[nsIndex].Pods[i].Total
			readySum += gui.positions.namespaces[nsIndex].Pods[i].Ready
		}
		fgColor := termbox.ColorDefault
		if totalSum != readySum {
			fgColor = termbox.ColorRed
		}
		printLine(displayName, 0, yPosition, fgColor, termbox.ColorDefault)
		printLine(fmt.Sprintf("%v/%v", readySum, totalSum), gui.nameWidth, yPosition, fgColor, termbox.ColorDefault)
	} else {
		printDefaultLine(displayName, 0, yPosition)
	}
}

func (gui *Gui) updatePositions() {
	position := 0

	// TODO may be there is a better way of doing this for example map[int]Position{Type, interface{}} ?
	// TODO can stop after reaching screen border
	nsPositions := make(map[int]*Namespace)
	podPositions := make(map[int]*Pod)
	contPositions := make(map[int]*Container)
	msgPositions := make(map[int]string)
	errPositions := make(map[int]string)

	nameWidth := NameColStartWidth
	statusWidth := StatusColStartWidth

	for nsIndex, ns := range gui.namespaces {
		nsPositions[position] = &gui.namespaces[nsIndex]
		if len(gui.namespaces[nsIndex].displayName())+2 > nameWidth {
			nameWidth = len(gui.namespaces[nsIndex].displayName()) + 2
		}
		position++
		if gui.nsCollapsed[ns.displayName()] {
			continue
		}
		if len(gui.namespaces[nsIndex].Pods) == 0 {
			errPositions[position] = "No resources found."
			position++
		}
		if ns.Error != nil {
			errPositions[position] = ns.Error.Error()
			position++
		}

		for podIndex := range gui.namespaces[nsIndex].Pods {
			podPositions[position] = &gui.namespaces[nsIndex].Pods[podIndex]
			if len(gui.namespaces[nsIndex].Pods[podIndex].Name)+5 > nameWidth {
				nameWidth = len(gui.namespaces[nsIndex].Pods[podIndex].Name) + 5
			}
			if len(gui.namespaces[nsIndex].Pods[podIndex].Status)+3 > statusWidth {
				statusWidth = len(gui.namespaces[nsIndex].Pods[podIndex].Status) + 3
			}
			position++

			if gui.podExpanded[gui.namespaces[nsIndex].Pods[podIndex].Name] {
				for contIndex := range gui.namespaces[nsIndex].Pods[podIndex].Containers {
					contPositions[position] = &gui.namespaces[nsIndex].Pods[podIndex].Containers[contIndex]
					position++
					if gui.namespaces[nsIndex].Pods[podIndex].Containers[contIndex].Message != "" {
						msgPositions[position] = gui.namespaces[nsIndex].Pods[podIndex].Containers[contIndex].Message
						position++
					}
				}
			}
		}
	}
	gui.positions = Positions{namespaces: nsPositions, pods: podPositions, containers: contPositions, message: msgPositions, errors: errPositions, lastIndex: position - 1}
	gui.nameWidth = nameWidth
	gui.statusWidth = statusWidth
}

func (p *Pod) printPodInfo(yPos int, nameWidth, statusWidth int) {
	running := p.Status == "Running"
	fg := termbox.ColorDefault
	if running && p.Ready >= p.Total {
		fg = termbox.ColorGreen
	} else if running && p.Ready < p.Total {
		fg = termbox.ColorYellow
	} else {
		fg = termbox.ColorRed
	}

	printLine(p.Name, 3, yPos, fg, termbox.ColorDefault)
	printLine(p.readyString(), nameWidth, yPos, fg, termbox.ColorDefault)
	printLine(string(p.Status), nameWidth+ReadyColWidth, yPos, fg, termbox.ColorDefault)
	printLine(strconv.Itoa(p.Restarts), nameWidth+ReadyColWidth+statusWidth, yPos, fg, termbox.ColorDefault)
	printLine(p.Age, nameWidth+ReadyColWidth+statusWidth+RestartsColWidth, yPos, fg, termbox.ColorDefault)
}

func (c *Container) printContainerInfo(yPos int) {
	if c.Ready {
		printLine(c.Image, 6, yPos, termbox.ColorGreen, termbox.ColorDefault)
	} else {
		printLine(c.Image, 6, yPos, termbox.ColorRed, termbox.ColorDefault)
	}
}

func clearLine(x, y, endx int, fg, bg termbox.Attribute) {
	for i := x; i <= endx; i++ {
		termbox.SetCell(i, y, ' ', fg, bg)
	}
}

func printDefaultLine(line string, x, y int) {
	printLine(line, x, y, termbox.ColorDefault, termbox.ColorDefault)
}

func printLine(line string, x, y int, fg termbox.Attribute, bg termbox.Attribute) {
	for k, v := range line {
		termbox.SetCell(k+x, y, v, fg, bg)
	}
}

func (gui *Gui) updateWindowSize() {
	width, height := termbox.Size()

	gui.mutex.Lock()
	gui.width = width
	gui.height = height
	gui.mutex.Unlock()
}

func (gui *Gui) cursorToStartPos() {
	gui.mutex.Lock()
	gui.scrollOffset = 0
	gui.mutex.Unlock()
	gui.moveCursor(0, InfoAreaStart)
}

func (ns *Namespace) displayName() string {
	return fmt.Sprintf("%v / %v", ns.Name, ns.Context)
}

func (p *Pod) readyString() string {
	return fmt.Sprintf("%v/%v", p.Ready, p.Total)
}

func (p *Positions) hasNamespace(index int) bool {
	return p.namespaces[index] != nil
}

func (p *Positions) hasPod(index int) bool {
	return p.pods[index] != nil
}

func (p *Positions) hasContainer(index int) bool {
	return p.containers[index] != nil
}

func (p *Positions) hasMessage(index int) bool {
	_, ok := p.message[index]
	return ok
}

func (p *Positions) hasError(index int) bool {
	return len(p.errors[index]) > 0
}

func clear() {
	err := termbox.Clear(termbox.ColorDefault, termbox.ColorDefault)
	if err != nil {
		termbox.Close()
		log.Fatal("termbox.Clear(): ", err)
	}
}

func clearStatusArea() {
	width, height := termbox.Size()

	for i := height - StatusAreaHeight; i < height-1; i++ {
		clearLine(0, i, width, termbox.ColorDefault, termbox.ColorDefault)
	}

}

func flush() {
	err := termbox.Flush()
	if err != nil {
		termbox.Close()
		log.Fatal("termbox.Flush(): ", err)
	}
}
