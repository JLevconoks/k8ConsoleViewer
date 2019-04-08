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
	mutex           sync.Mutex
}

type Positions struct {
	namespaces map[int]*Namespace
	pods       map[int]*Pod
}

func (gui *Gui) redrawAll() {
	clear()
	gui.printHeaders()
	gui.printNamespace()
	gui.adjustCursorPosition()
	gui.printStatusArea()
	flush()
}

func (gui *Gui) moveCursorDown() {
	if gui.curY < gui.curBottomBorder {
		gui.moveCursor(gui.curX, gui.curY+1)
	}
}

func (gui *Gui) moveCursorUp() {
	if gui.curY > gui.curTopBorder {
		gui.moveCursor(gui.curX, gui.curY-1)
	}
}

func (gui *Gui) moveCursor(x, y int) {
	gui.mutex.Lock()
	gui.curX = x
	gui.curY = y
	gui.mutex.Unlock()

	log.Println("Cursor on: ", gui.curY)
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
	if gui.Positions.namespaces[gui.curY] != nil {
		ns := gui.Positions.namespaces[gui.curY]

		all := fmt.Sprintf("kubectl --context %v -n %v get all", gui.Context, ns.Name)
		ingress := fmt.Sprintf("kubectl --context %v -n %v get ingress", gui.Context, ns.Name)
		events := fmt.Sprintf("kubectl --context %v -n %v get ev --sort-by=.lastTimestamp", gui.Context, ns.Name)

		clearStatusArea()
		printDefaultLine(all, 0, gui.height-3)
		printDefaultLine(ingress, 0, gui.height-2)
		printDefaultLine(events, 0, gui.height-1)
	} else if gui.Positions.pods[gui.curY] != nil {
		pod := gui.Positions.pods[gui.curY]

		podLog := fmt.Sprintf("kubectl --context %v -n %v logs %v", gui.Context, pod.Namespace.Name, pod.Name)
		exec := fmt.Sprintf("kubectl --context %v -n %v exec -it %v /bin/sh", gui.Context, pod.Namespace.Name, pod.Name)

		clearStatusArea()
		printDefaultLine(podLog, 0, gui.height-3)
		printDefaultLine(exec, 0, gui.height-2)
	}
}

func (gui *Gui) printNamespace() {
	yOffset := TOP_AREA_HEIGHT + 1
	nsPositions := make(map[int]*Namespace)
	podPositions := make(map[int]*Pod)

	for nsIndex, ns := range gui.Namespaces {
		if yOffset < gui.height-STATUS_AREA_HEIGHT {
			printDefaultLine(ns.Name, 0, yOffset)
			nsPositions[yOffset] = &gui.Namespaces[nsIndex]
			yOffset++
			if ns.Error != nil {
				printLine(ns.Error.Error(), 3, yOffset, termbox.ColorYellow, termbox.ColorDefault)
				yOffset++
			} else {
				yOffset = gui.Namespaces[nsIndex].printPods(gui.height, yOffset, podPositions)
			}
		} else {
			break
		}
	}

	gui.mutex.Lock()
	gui.Positions = Positions{namespaces: nsPositions, pods: podPositions}
	gui.curBottomBorder = yOffset - 1
	gui.mutex.Unlock()
}

func (ns *Namespace) printPods(windowHeight, yOffset int, podPositions map[int]*Pod) int {
	for i, p := range ns.Pods {
		if yOffset < windowHeight-STATUS_AREA_HEIGHT {
			podPositions[yOffset] = &ns.Pods[i]
			running := p.Status == "Running"

			if running && p.Ready >= p.Total {
				p.printPodInfo(yOffset, termbox.ColorGreen)
			} else if running && p.Ready < p.Total {
				p.printPodInfo(yOffset, termbox.ColorYellow)
			} else {
				p.printPodInfo(yOffset, termbox.ColorRed)
			}
			yOffset++
		} else {
			break
		}
	}
	return yOffset
}

func (pod *Pod) printPodInfo(yOffset int, fg termbox.Attribute) {
	printLine(pod.Name, 3, yOffset, fg, termbox.ColorDefault)
	printLine(pod.readyString(), 3+NAME_COL_WIDTH, yOffset, fg, termbox.ColorDefault)
	printLine(pod.Status, 3+NAME_COL_WIDTH+READY_COL_WIDTH, yOffset, fg, termbox.ColorDefault)
	printLine(pod.Restarts, 3+NAME_COL_WIDTH+READY_COL_WIDTH+STATUS_COL_WIDTH, yOffset, fg, termbox.ColorDefault)
	printLine(pod.Age, 3+NAME_COL_WIDTH+READY_COL_WIDTH+STATUS_COL_WIDTH+RESTARTS_COL_WIDTH, yOffset, fg, termbox.ColorDefault)
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
