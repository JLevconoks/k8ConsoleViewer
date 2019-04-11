package main

import (
	"fmt"
	"github.com/nsf/termbox-go"
	"log"
	"sync"
	"time"
)

const (
	NAME_COL_WIDTH     = 57
	READY_COL_WIDTH    = 7
	STATUS_COL_WIDTH   = 20
	RESTARTS_COL_WIDTH = 10
	STATUS_AREA_HEIGHT = 4
	INFO_AREA_START    = 5
	TOP_AREA_HEIGHT    = 4
)

type Gui struct {
	Group           string
	Context         string
	Namespaces      []Namespace
	TimeToExecute   time.Duration
	height          int
	width           int
	curX            int
	curY            int
	curTopBorder    int
	curBottomBorder int
	Positions       Positions
	scrollOffset    int
	mutex           sync.Mutex
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
	printDefaultLine(fmt.Sprintf("%v    Time to execute: %v\n", time.Now().Format(time.RFC1123Z), gui.TimeToExecute.String()), 0, 0)
	printDefaultLine(fmt.Sprintf("Group: %v", gui.Group), 0, 1)
	printDefaultLine("NAMESPACE", 0, 3)
	printDefaultLine("NAME", 3, 4)
	printDefaultLine("READY", 3+NAME_COL_WIDTH, 4)
	printDefaultLine("STATUS", 3+NAME_COL_WIDTH+READY_COL_WIDTH, 4)
	printDefaultLine("RESTARTS", 3+NAME_COL_WIDTH+READY_COL_WIDTH+STATUS_COL_WIDTH, 4)
	printDefaultLine("AGE", 3+NAME_COL_WIDTH+READY_COL_WIDTH+STATUS_COL_WIDTH+RESTARTS_COL_WIDTH, 4)
}

func (gui *Gui) printStatusArea() {
	index := gui.curY - INFO_AREA_START + gui.scrollOffset
	if gui.Positions.hasNamespace(index) {
		ns := gui.Positions.namespaces[index]

		all := fmt.Sprintf("kubectl --context %v -n %v get all", gui.Context, ns.Name)
		ingress := fmt.Sprintf("kubectl --context %v -n %v get ingress", gui.Context, ns.Name)
		events := fmt.Sprintf("kubectl --context %v -n %v get ev --sort-by=.lastTimestamp", gui.Context, ns.Name)

		clearStatusArea()
		printDefaultLine(all, 0, gui.height-3)
		printDefaultLine(ingress, 0, gui.height-2)
		printDefaultLine(events, 0, gui.height-1)
	} else if gui.Positions.hasPod(index) {
		pod := gui.Positions.pods[index]

		podLog := fmt.Sprintf("kubectl --context %v -n %v logs %v", gui.Context, pod.Namespace.Name, pod.Name)
		exec := fmt.Sprintf("kubectl --context %v -n %v exec -it %v /bin/sh", gui.Context, pod.Namespace.Name, pod.Name)

		clearStatusArea()
		printDefaultLine(podLog, 0, gui.height-3)
		printDefaultLine(exec, 0, gui.height-2)
	} else {
		clearStatusArea()
	}
}

func (gui *Gui) printMainInfo() {
	position := 0
	nsPositions := make(map[int]*Namespace)
	podPositions := make(map[int]*Pod)
	errPositions := make(map[int]string)

	//TODO Need to move position calculation out of screen refresh loop.
	for nsIndex, ns := range gui.Namespaces {
		nsPositions[position] = &gui.Namespaces[nsIndex]
		position++
		if ns.Error != nil {
			errPositions[position] = ns.Error.Error()
			position++
		}
		for podIndex := range gui.Namespaces[nsIndex].Pods {
			podPositions[position] = &gui.Namespaces[nsIndex].Pods[podIndex]
			position++
		}
	}
	gui.mutex.Lock()
	gui.Positions = Positions{namespaces: nsPositions, pods: podPositions, errors: errPositions, lastIndex: position - 1}
	gui.mutex.Unlock()

	offset := gui.scrollOffset
	yPosition := INFO_AREA_START
	for gui.Positions.hasNamespace(offset) || gui.Positions.hasPod(offset) || gui.Positions.hasError(offset) {
		if yPosition < gui.height-STATUS_AREA_HEIGHT {
			if gui.Positions.hasNamespace(offset) {
				printDefaultLine(gui.Positions.namespaces[offset].Name, 0, yPosition)
			}
			if gui.Positions.hasPod(offset) {
				gui.Positions.pods[offset].printPodInfo(yPosition)
			}
			if gui.Positions.hasError(offset) {
				printLine(gui.Positions.errors[offset], 3, yPosition, termbox.ColorYellow, termbox.ColorDefault)
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

func (p *Pod) printPodInfo(y int) {
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
	printLine(p.readyString(), 3+NAME_COL_WIDTH, y, fg, termbox.ColorDefault)
	printLine(p.Status, 3+NAME_COL_WIDTH+READY_COL_WIDTH, y, fg, termbox.ColorDefault)
	printLine(p.Restarts, 3+NAME_COL_WIDTH+READY_COL_WIDTH+STATUS_COL_WIDTH, y, fg, termbox.ColorDefault)
	printLine(p.Age, 3+NAME_COL_WIDTH+READY_COL_WIDTH+STATUS_COL_WIDTH+RESTARTS_COL_WIDTH, y, fg, termbox.ColorDefault)
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

	for i := height - STATUS_AREA_HEIGHT; i < height; i++ {
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
