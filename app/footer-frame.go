package app

import (
	"github.com/gdamore/tcell"
	"strings"
)

type FooterFrame struct {
	x, y               int
	width, height      int
	lines              []string
	clipboardShortcuts map[Type][]string
	statusBar          *StringItem
	statusBarCh        chan string
}

func NewFooterFrame(s tcell.Screen) *FooterFrame {
	winWidth, winHeight := s.Size()
	sbCh := make(chan string)
	frame := FooterFrame{
		x:                  0,
		y:                  winHeight - FooterFrameHeight,
		width:              winWidth,
		height:             FooterFrameHeight,
		lines:              make([]string, FooterFrameHeight-1),
		clipboardShortcuts: make(map[Type][]string),
		statusBar:          &StringItem{x: 0, y: winHeight - 1, length: 0, value: ""},
		statusBarCh:        sbCh,
	}
	frame.lines[0] = strings.Repeat("-", 25)
	frame.listenForStatusMessages(s)
	return &frame
}

func (ff *FooterFrame) listenForStatusMessages(s tcell.Screen) {
	go func() {
		for value := range ff.statusBarCh {
			ff.statusBar.Update(s, value)
			s.Show()
		}
	}()
}

func (ff *FooterFrame) updateShortcutInfo(s tcell.Screen, i Item) {
	csInfo, ok := ff.clipboardShortcuts[i.Type()]

	if ok {
		if len(csInfo) != 2 {
			// Something went wrong here, should be always 2, for now.
			ff.statusBarCh <- "Error trying to generate clipboard shortcut info."
			return
		}
		ff.lines[1] = csInfo[0]
		ff.lines[2] = csInfo[1]
	} else {
		ff.lines[1] = ""
		ff.lines[2] = ""
	}
	ff.update(s)
}

func (ff *FooterFrame) update(s tcell.Screen) {
	for k, v := range ff.lines {
		drawS(s, v, 0, ff.y+k, ff.width, tcell.StyleDefault)
	}
}

func (ff *FooterFrame) resize(s tcell.Screen, winWidth, winHeight int) {
	ff.width = winWidth
	ff.y = winHeight - FooterFrameHeight
	ff.statusBar.y = winHeight - 1
	ff.update(s)
}
