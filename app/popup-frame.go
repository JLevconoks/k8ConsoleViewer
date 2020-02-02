package app

import "github.com/gdamore/tcell"

const (
	PopupItemXOffset = 2
	PopupItemYOffset = 2
)

type PopupFrame struct {
	x, y          int
	width, height int
	visible       bool
	title         string
	items         []string
	cursorYPos    int
	callback      func(string)
}

func NewPopupFrame(title string, items []string, callback func(string)) *PopupFrame {
	return &PopupFrame{
		x: 5,
		y: 5,
		// TODO: This needs to be dynamic
		width:    15,
		height:   15,
		visible:  false,
		title:    title,
		items:    items,
		callback: callback,
	}
}

func (pf *PopupFrame) show(s tcell.Screen) {
	pf.clear(s)
	pf.drawItems(s)
	pf.drawBorder(s)
}

func (pf *PopupFrame) clear(s tcell.Screen) {
	for y := pf.y; y < pf.y+pf.height; y++ {
		for x := pf.x; x < pf.x+pf.width; x++ {
			s.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		}
	}
}

func (pf *PopupFrame) moveCursorDown(s tcell.Screen) {
	if pf.cursorYPos < len(pf.items)-1 {
		pf.cursorYPos++
	} else {
		pf.cursorYPos = len(pf.items) - 1
	}
	pf.drawItems(s)
}

func (pf *PopupFrame) moveCursorUp(s tcell.Screen) {
	if pf.cursorYPos > 0 {
		pf.cursorYPos--
	} else {
		pf.cursorYPos = 0
	}
	pf.drawItems(s)
}

func (pf *PopupFrame) drawBorder(s tcell.Screen) {
	for x := 1; x < pf.width; x++ {
		s.SetContent(pf.x+x, pf.y, '_', nil, tcell.StyleDefault)
		s.SetContent(pf.x+x, pf.y+pf.height, '_', nil, tcell.StyleDefault)
	}
	for y := 1; y < pf.height+1; y++ {
		s.SetContent(pf.x, pf.y+y, '|', nil, tcell.StyleDefault)
		s.SetContent(pf.x+pf.width, pf.y+y, '|', nil, tcell.StyleDefault)
	}

	draw(s, pf.title, pf.x+3, pf.y, len(pf.title), tcell.StyleDefault)
}

func (pf *PopupFrame) drawItems(s tcell.Screen) {
	for index, item := range pf.items {
		style := tcell.StyleDefault
		if pf.cursorYPos == index {
			style = tcell.StyleDefault.Reverse(true)
		}
		draw(s, item, pf.x+PopupItemXOffset, pf.y+PopupItemYOffset+index, len(item), style)
	}
}
