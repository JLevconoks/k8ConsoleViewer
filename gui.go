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
	context         string
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
}

type Positions struct {
	namespaces map[int]*Namespace
	pods       map[int]*Pod
	errors     map[int]string
	lastIndex  int
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
	if gui.positions.hasPod(index) || gui.positions.hasError(index) {
		for !gui.positions.hasNamespace(index) {
			index--
		}
		if index < gui.scrollOffset {
			gui.scrollOffset = index
			gui.redrawAll()
		}

		gui.moveCursor(0, index+InfoAreaStart-gui.scrollOffset)

	} else if gui.positions.hasNamespace(index) && !gui.nsCollapsed[gui.positions.namespaces[index].Name] {
		gui.collapseNamespace(index)
	}
}

func (gui *Gui) collapseNamespace(index int) {
	gui.mutex.Lock()
	gui.nsCollapsed[gui.positions.namespaces[index].Name] = true
	gui.updatePositions()
	gui.mutex.Unlock()
	gui.redrawAll()
}

func (gui *Gui) expandNamespace() {
	index := gui.curY - InfoAreaStart + gui.scrollOffset
	if gui.positions.hasNamespace(index) && gui.nsCollapsed[gui.positions.namespaces[index].Name] {
		gui.mutex.Lock()
		gui.nsCollapsed[gui.positions.namespaces[index].Name] = false
		gui.updatePositions()
		gui.mutex.Unlock()
		gui.redrawAll()
	}
}

func (gui *Gui) collapseAllNS() {
	gui.mutex.Lock()
	for _, ns := range gui.positions.namespaces {
		gui.nsCollapsed[ns.Name] = true
	}
	gui.updatePositions()
	gui.mutex.Unlock()

	gui.cursorToStartPos()
	gui.redrawAll()
}

func (gui *Gui) expandAllNS() {
	gui.mutex.Lock()
	for _, ns := range gui.positions.namespaces {
		gui.nsCollapsed[ns.Name] = false
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
	printDefaultLine("NAMESPACE", 0, 3)
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

		all := fmt.Sprintf("kubectl --context %v -n %v get all", gui.context, ns.Name)
		ingress := fmt.Sprintf("kubectl --context %v -n %v get ingress", gui.context, ns.Name)
		events := fmt.Sprintf("kubectl --context %v -n %v get ev --sort-by=.lastTimestamp", gui.context, ns.Name)

		clearStatusArea()
		printDefaultLine(all, 0, gui.height-4)
		printDefaultLine(ingress, 0, gui.height-3)
		printDefaultLine(events, 0, gui.height-2)
	} else if gui.positions.hasPod(index) {
		pod := gui.positions.pods[index]

		podLog := fmt.Sprintf("kubectl --context %v -n %v logs %v", gui.context, pod.Namespace.Name, pod.Name)
		exec := fmt.Sprintf("kubectl --context %v -n %v exec -it %v /bin/sh", gui.context, pod.Namespace.Name, pod.Name)

		clearStatusArea()
		printDefaultLine(podLog, 0, gui.height-4)
		printDefaultLine(exec, 0, gui.height-3)
	} else {
		clearStatusArea()
	}
}

func (gui *Gui) printMainInfo() {
	offset := gui.scrollOffset
	yPosition := InfoAreaStart
	for gui.positions.hasNamespace(offset) || gui.positions.hasPod(offset) || gui.positions.hasError(offset) {
		if yPosition < gui.height-StatusAreaHeight {
			if gui.positions.hasNamespace(offset) {
				gui.printNamespace(offset, yPosition)
			}
			if gui.positions.hasPod(offset) {
				gui.positions.pods[offset].printPodInfo(yPosition, gui.nameWidth, gui.statusWidth)
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
	nsName := gui.positions.namespaces[nsIndex].Name
	if gui.nsCollapsed[nsName] {
		totalSum := 0
		readySum := 0
		for k := range gui.positions.namespaces[nsIndex].Pods {
			totalSum += gui.positions.namespaces[nsIndex].Pods[k].Total
			readySum += gui.positions.namespaces[nsIndex].Pods[k].Ready
		}
		fgColor := termbox.ColorDefault
		if totalSum != readySum {
			fgColor = termbox.ColorRed
		}
		printLine(nsName, 0, yPosition, fgColor, termbox.ColorDefault)
		printLine(fmt.Sprintf("%v/%v", readySum, totalSum), gui.nameWidth, yPosition, fgColor, termbox.ColorDefault)
	} else {
		printDefaultLine(nsName, 0, yPosition)
	}
}

func (gui *Gui) updatePositions() {
	position := 0
	nsPositions := make(map[int]*Namespace)
	podPositions := make(map[int]*Pod)
	errPositions := make(map[int]string)

	nameWidth := NameColStartWidth
	statusWidth := StatusColStartWidth

	for nsIndex, ns := range gui.namespaces {
		nsPositions[position] = &gui.namespaces[nsIndex]
		if len(gui.namespaces[nsIndex].Name)+2 > nameWidth {
			nameWidth = len(gui.namespaces[nsIndex].Name) + 2
		}
		position++
		if gui.nsCollapsed[ns.Name] {
			continue
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
		}
	}
	gui.positions = Positions{namespaces: nsPositions, pods: podPositions, errors: errPositions, lastIndex: position - 1}
	gui.nameWidth = nameWidth
	gui.statusWidth = statusWidth
}

func (p *Pod) printPodInfo(y int, nameWidth, statusWidth int) {
	running := p.Status == "Running"
	fg := termbox.ColorDefault
	if running && p.Ready >= p.Total {
		fg = termbox.ColorGreen
	} else if running && p.Ready < p.Total {
		fg = termbox.ColorYellow
	} else {
		fg = termbox.ColorRed
	}

	printLine(p.Name, 3, y, fg, termbox.ColorDefault)
	printLine(p.readyString(), nameWidth, y, fg, termbox.ColorDefault)
	printLine(p.Status, nameWidth+ReadyColWidth, y, fg, termbox.ColorDefault)
	printLine(strconv.Itoa(p.Restarts), nameWidth+ReadyColWidth+statusWidth, y, fg, termbox.ColorDefault)
	printLine(p.Age, nameWidth+ReadyColWidth+statusWidth+RestartsColWidth, y, fg, termbox.ColorDefault)
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

func (pod *Pod) readyString() string {
	return fmt.Sprintf("%v/%v", pod.Ready, pod.Total)
}

func (p *Positions) hasNamespace(index int) bool {
	return p.namespaces[index] != nil
}

func (p *Positions) hasPod(index int) bool {
	return p.pods[index] != nil
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
