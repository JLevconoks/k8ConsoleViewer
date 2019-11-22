package app

import (
	"github.com/gdamore/tcell"
	"strings"
)

type FooterFrame struct {
	x, y          int
	width, height int
	lines         []string
	statusBar     *StringItem
	statusBarCh   chan string
}

func NewFooterFrame(s tcell.Screen) *FooterFrame {
	winWidth, winHeight := s.Size()
	sbCh := make(chan string)
	frame := FooterFrame{
		x:           0,
		y:           winHeight - FooterFrameHeight,
		width:       winWidth,
		height:      FooterFrameHeight,
		lines:       make([]string, FooterFrameHeight-1),
		statusBar:   &StringItem{x: 0, y: winHeight - 1, length: 0, value: ""},
		statusBarCh: sbCh,
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
	switch i.Type() {
	case TypeNamespace:
		ff.lines[1] = "1 = get all       3 = get events   5 = get secrets"
		ff.lines[2] = "2 = get ingress   4 = describe     6 = get config map"
	case TypePod:
		ff.lines[1] = "1 = get logs      3 = describe     5 = scale"
		ff.lines[2] = "2 = exec          4 = delete pod"
	case TypeContainer:
		ff.lines[1] = "1 = get logs"
		ff.lines[2] = "2 = exec"
	}
	ff.update(s)
	s.Show()
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
