package app

import (
	"fmt"
	"github.com/gdamore/tcell"
	"sort"
	"strconv"
	"strings"
	"sync"
)

type InfoFrame struct {
	sync.Mutex

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
}

func NewInfoFrame(winWidth, winHeight int) *InfoFrame {
	nsHeader := StringItem{NamespaceXOffset, 3, 0, "NAMESPACE / CONTEXT"}
	podHeader := StringItem{PodXOffset, 4, 0, "NAME  READY  STATUS  RESTARTS  AGE"}
	width, height := calcInfoFrameSize(winWidth, winHeight)

	return &InfoFrame{
		x:                0,
		y:                MainFrameStartY,
		width:            width,
		height:           height,
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
}

func (f *InfoFrame) refresh(s tcell.Screen) {
	f.clear(s)
	f.updatePositions()
	f.updatePodHeader(s)
	f.updateFrameInfo(s)
	f.updateCursor(s)
}

func (f *InfoFrame) clear(s tcell.Screen) {
	for y := 0; y <= f.height-1; y++ {
		for x := 0; x <= f.width; x++ {
			s.SetContent(x+f.x, y+f.y, ' ', nil, tcell.StyleDefault)
		}
	}
}

func (f *InfoFrame) updatePositions() {
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

			for dIndex := range f.nsItems[nsIndex].deployments {
				positions = append(positions, f.nsItems[nsIndex].deployments[dIndex])
				if f.nsItems[nsIndex].deployments[dIndex].isExpanded {
					for pIndex := range f.nsItems[nsIndex].deployments[dIndex].pods {
						positions = append(positions, &(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex]))

						if f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].isExpanded {
							for pContainer := range f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].containers {
								positions = append(positions, &(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].containers[pContainer]))
							}
						}
					}
				}
			}
		}
	}
	f.positions = positions
}

func (f *InfoFrame) updatePodHeader(s tcell.Screen) {
	f.nameColWidth = NameColumnDefaultWidth
	f.readyColWidth = ReadyColumnDefaultWidth
	f.statusColWidth = StatusColumnDefaultWidth
	f.restartsColWidth = RestartsColumnDefaultWidth
	f.ageColWidth = AgeColumnDefaultWidth

	for nsIndex, _ := range f.nsItems {
		if f.nameColWidth < ColumnSpacing+len(f.nsItems[nsIndex].DisplayName()) {
			f.nameColWidth = ColumnSpacing + len(f.nsItems[nsIndex].DisplayName())
		}
		if f.nsItems[nsIndex].IsExpanded() {
			for dIndex := range f.nsItems[nsIndex].deployments {
				if f.nameColWidth < PodGroupXOffset+ColumnSpacing+len(f.nsItems[nsIndex].deployments[dIndex].name) {
					f.nameColWidth = PodGroupXOffset + ColumnSpacing + len(f.nsItems[nsIndex].deployments[dIndex].name)
				}
				if f.nsItems[nsIndex].deployments[dIndex].isExpanded {
					for pIndex := range f.nsItems[nsIndex].deployments[dIndex].pods {
						if f.nameColWidth < ColumnSpacing+len(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].name) {
							f.nameColWidth = ColumnSpacing + len(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].name)
						}
						if f.readyColWidth < ColumnSpacing+len(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].ReadyString()) {
							f.readyColWidth = ColumnSpacing + len(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].ReadyString())
						}
						if f.statusColWidth < ColumnSpacing+len(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].status) {
							f.statusColWidth = ColumnSpacing + len(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].status)
						}
						if f.restartsColWidth < ColumnSpacing+len(strconv.Itoa(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].restarts)) {
							f.restartsColWidth = ColumnSpacing + len(strconv.Itoa(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].restarts))
						}
						if f.ageColWidth < ColumnSpacing+len(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].age) {
							f.ageColWidth = ColumnSpacing + len(f.nsItems[nsIndex].deployments[dIndex].pods[pIndex].age)
						}
					}
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

func (f *InfoFrame) updateFrameInfo(s tcell.Screen) {
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
		case TypePodGroup:
			f.printPodGroup(s, position.(*PodGroup), posIndex)
		case TypePod:
			f.printPod(s, position.(*Pod), posIndex)
		case TypeContainer:
			f.printContainer(s, position.(*Container), posIndex)
		}
	}
}

func (f *InfoFrame) printNamespace(s tcell.Screen, ns *Namespace, yPos int) {
	style := tcell.StyleDefault

	if !ns.isExpanded {
		readyCount := 0
		totalCount := 0
		for dIndex := range ns.deployments {
			totalCount += len(ns.deployments[dIndex].pods)
			readyCount += ns.deployments[dIndex].countReadyPods()
		}
		if readyCount != totalCount || ns.nsError.error != nil {
			style = style.Foreground(tcell.ColorRed)
		}
		readyColPos := f.nameColWidth - NamespaceXOffset + PodXOffset
		drawS(s, ns.DisplayName(), NamespaceXOffset, f.y+yPos, readyColPos, style)
		drawS(s, fmt.Sprintf("%v/%v", readyCount, totalCount), readyColPos, f.y+yPos, f.width-readyColPos, style)
	} else {
		drawS(s, ns.DisplayName(), NamespaceXOffset, f.y+yPos, f.width-NamespaceXOffset, style)
	}
}

func (f *InfoFrame) printNamespaceError(s tcell.Screen, nse *NamespaceError, yPos int) {
	drawS(s, nse.error.Error(), NamespaceErrorXOffset, f.y+yPos, f.width, tcell.StyleDefault.Foreground(tcell.ColorYellow))
}

func (f *InfoFrame) printNamespaceMessage(s tcell.Screen, nse *NamespaceMessage, yPos int) {
	drawS(s, nse.message, NamespaceMessageXOffset, f.y+yPos, f.width, tcell.StyleDefault.Foreground(tcell.ColorYellow))
}

func (f *InfoFrame) printPodGroup(s tcell.Screen, d *PodGroup, yPos int) {
	style := tcell.StyleDefault

	if !d.isExpanded {
		total := len(d.pods)
		ready := d.countReadyPods()
		if total != ready {
			style = style.Foreground(tcell.ColorRed)
		} else {
			style = style.Foreground(tcell.ColorGreen)
		}

		readyColPos := f.nameColWidth - NamespaceXOffset + PodXOffset
		drawS(s, d.name, PodGroupXOffset, f.y+yPos, readyColPos, style)
		drawS(s, fmt.Sprintf("%v/%v", ready, total), readyColPos, f.y+yPos, f.width-readyColPos, style)
	} else {
		drawS(s, d.name, PodGroupXOffset, f.y+yPos, f.width, style)
	}
}

func (f *InfoFrame) printPod(s tcell.Screen, p *Pod, yPos int) {
	running := p.status == "Running"
	style := tcell.StyleDefault
	if !p.isExpanded {
		if running && p.ready >= p.total {
			style = style.Foreground(tcell.ColorGreen)
		} else if running && p.ready < p.total {
			style = style.Foreground(tcell.ColorYellow)
		} else {
			style = style.Foreground(tcell.ColorRed)
		}
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

func (f *InfoFrame) printContainer(s tcell.Screen, c *Container, yPos int) {
	style := tcell.StyleDefault.Foreground(tcell.ColorGreen)
	if !c.ready {
		style = style.Foreground(tcell.ColorRed)
	}

	drawS(s, c.DisplayName(), ContainerXOffset, f.y+yPos, f.width-ContainerXOffset, style)
}

// updateNamespaces will get all expanded item names, apply expanded flag on new namespaces infos and replace existing f.nsItems with
// new namespaces.
// frame positions will need to be updated straight after to avoid errors.
func (f *InfoFrame) updateNamespaces(podListResults []PodListResult) {
	expanded := make(map[string]struct{}, 0)

	for nsIndex, _ := range f.nsItems {
		nsDisplayName := f.nsItems[nsIndex].DisplayName()
		if f.nsItems[nsIndex].IsExpanded() {
			expanded[nsDisplayName] = struct{}{}
		}
		for dIndex := range f.nsItems[nsIndex].deployments {
			deploymentName := f.nsItems[nsIndex].deployments[dIndex].name
			if f.nsItems[nsIndex].deployments[dIndex].isExpanded {
				// Here we need a unique deployment name, therefore it is concatenated with namespace name.
				expanded[nsDisplayName+deploymentName] = struct{}{}
			}
			for podIndex := range f.nsItems[nsIndex].deployments[dIndex].pods {
				if f.nsItems[nsIndex].deployments[dIndex].pods[podIndex].IsExpanded() {
					expanded[f.nsItems[nsIndex].deployments[dIndex].pods[podIndex].name] = struct{}{}
				}
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
		nsDisplayName := newNamespaces[nsIndex].DisplayName()
		_, ok := expanded[nsDisplayName]
		if ok {
			newNamespaces[nsIndex].isExpanded = true
		}
		for dIndex := range newNamespaces[nsIndex].deployments {
			deploymentName := newNamespaces[nsIndex].deployments[dIndex].name
			_, ok := expanded[nsDisplayName+deploymentName]
			if ok {
				newNamespaces[nsIndex].deployments[dIndex].Expanded(true)
			}
			for podIndex := range newNamespaces[nsIndex].deployments[dIndex].pods {
				_, ok := expanded[newNamespaces[nsIndex].deployments[dIndex].pods[podIndex].name]
				if ok {
					newNamespaces[nsIndex].deployments[dIndex].pods[podIndex].isExpanded = true
				}
			}
		}
	}
	f.nsItems = newNamespaces
}

func (f *InfoFrame) updateCursor(s tcell.Screen) {
	if f.cursorY+f.scrollYOffset > len(f.positions)-1 {
		diff := f.cursorY + f.scrollYOffset - (len(f.positions) - 1)
		f.moveCursor(s, -diff)
	}
	s.ShowCursor(f.x+f.cursorX, f.y+f.cursorY)
}

// moveCursor will move cursor n positions from current position.
func (f *InfoFrame) moveCursor(s tcell.Screen, ny int) {
	if len(f.positions) == 0 {
		return
	}
	curFullPos := f.cursorY + f.scrollYOffset
	newCurY := f.cursorY
	newOffset := f.scrollYOffset
	if ny > 0 {
		// Moving down
		maxPositions := len(f.positions) - 1
		if curFullPos == maxPositions {
			return
		}
		if curFullPos+ny > maxPositions {
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
		if curFullPos <= ny {
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

func (f *InfoFrame) cursorFullPosition() int {
	return f.cursorY + f.scrollYOffset
}

func (f *InfoFrame) collapseCurrentItem(s tcell.Screen) {
	fullPos := f.cursorFullPosition()
	if len(f.positions) == 0 || fullPos >= len(f.positions) {
		return
	}
	item := f.positions[fullPos]
	if item.IsExpanded() {
		item.Expanded(false)
		f.refresh(s)
	} else {
		counter := 0
		for i := fullPos; i > 0; i-- {
			counter++
			if item.Type() > f.positions[fullPos-counter].Type() {
				break
			}
		}
		f.moveCursor(s, -counter)
	}
}

func (f *InfoFrame) expandCurrentItem(s tcell.Screen) {
	fullPos := f.cursorFullPosition()
	if len(f.positions) != 0 && fullPos < len(f.positions) {
		if !f.positions[fullPos].IsExpanded() {
			f.positions[fullPos].Expanded(true)
			f.refresh(s)
		} else {
			f.moveCursor(s, 1)
		}
	}
}

func (f *InfoFrame) collapseAllItems(s tcell.Screen) {
	for index, _ := range f.positions {
		f.positions[index].Expanded(false)
	}
	f.cursorY = 0
	f.scrollYOffset = 0
	f.refresh(s)
}

func (f *InfoFrame) expandAll(s tcell.Screen) {
	for nIndex := range f.nsItems {
		f.nsItems[nIndex].Expanded(true)
		for dIndex := range f.nsItems[nIndex].deployments {
			f.nsItems[nIndex].deployments[dIndex].Expanded(true)
		}
	}
	f.refresh(s)
}

func (f *InfoFrame) pageUp(s tcell.Screen) {
	tempCursorPos := 0
	if f.scrollYOffset > 0 {
		tempCursorPos = f.cursorY
	}

	f.moveCursor(s, -f.height-f.cursorY)
	f.cursorY = tempCursorPos
	f.updateCursor(s)
}

func (f *InfoFrame) pageDown(s tcell.Screen) {
	tempCursorPos := f.cursorY
	f.moveCursor(s, 2*f.height-f.cursorY-1)
	if f.cursorY+f.scrollYOffset != len(f.positions)-1 {
		f.cursorY = tempCursorPos
	}
	f.updateCursor(s)
}

func (f *InfoFrame) resize(s tcell.Screen, winWidth, winHeight int) {
	width, height := calcInfoFrameSize(winWidth, winHeight)
	f.width = width
	f.height = height
	f.refresh(s)
}

// calcSize will return frame size relatively to terminal window size and frame position.
func calcInfoFrameSize(winWidth, winHeight int) (width, height int) {
	return winWidth, winHeight - MainFrameStartY - FooterFrameHeight
}
