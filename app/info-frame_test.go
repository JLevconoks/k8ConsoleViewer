package app

import (
	"fmt"
	"github.com/gdamore/tcell"
	"strconv"
	"testing"
)

func TestMoveCursor(t *testing.T) {
	screen := tcell.NewSimulationScreen("")
	testTable := []struct {
		name           string
		frame          InfoFrame
		moveBy         int
		expectedCurY   int
		expectedOffset int
	}{
		{
			name:           "move_down_no_scroll",
			frame:          InfoFrame{height: 10, cursorY: 0, nsItems: fakeNamespaces(20)},
			moveBy:         5,
			expectedCurY:   5,
			expectedOffset: 0,
		},
		{
			name:           "move_down_bottom_border_no_scroll",
			frame:          InfoFrame{height: 10, cursorY: 8, nsItems: fakeNamespaces(20)},
			moveBy:         1,
			expectedCurY:   9,
			expectedOffset: 0,
		},
		{
			name:           "move_down_with_scroll",
			frame:          InfoFrame{height: 10, cursorY: 8, nsItems: fakeNamespaces(20)},
			moveBy:         2,
			expectedCurY:   9,
			expectedOffset: 1,
		},
		{
			name:           "move_down_with_scroll_offset_not_0",
			frame:          InfoFrame{height: 10, cursorY: 7, scrollYOffset: 3, nsItems: fakeNamespaces(20)},
			moveBy:         4,
			expectedCurY:   9,
			expectedOffset: 5,
		},
		{
			name:           "move_down_outside_positions_no_scroll",
			frame:          InfoFrame{height: 10, cursorY: 1, nsItems: fakeNamespaces(6)},
			moveBy:         10,
			expectedCurY:   5,
			expectedOffset: 0,
		},
		{
			name:           "move_down_with_scroll_outside_positions",
			frame:          InfoFrame{height: 10, cursorY: 7, scrollYOffset: 2, nsItems: fakeNamespaces(15)},
			moveBy:         10,
			expectedCurY:   9,
			expectedOffset: 5,
		},
		{
			name:           "move_down_with_empty_namespaces",
			frame:          InfoFrame{height: 10, cursorY: 0, scrollYOffset: 0, nsItems: nil},
			moveBy:         1,
			expectedCurY:   0,
			expectedOffset: 0,
		},
		{
			name:           "move_up_no_scroll",
			frame:          InfoFrame{height: 10, cursorY: 7, scrollYOffset: 0, nsItems: fakeNamespaces(20)},
			moveBy:         -2,
			expectedCurY:   5,
			expectedOffset: 0,
		},
		{
			name:           "move_up_to_border_now_scroll",
			frame:          InfoFrame{height: 10, cursorY: 2, scrollYOffset: 5, nsItems: fakeNamespaces(20)},
			moveBy:         -2,
			expectedCurY:   0,
			expectedOffset: 5,
		},
		{
			name:           "move_up_with_scroll",
			frame:          InfoFrame{height: 10, cursorY: 1, scrollYOffset: 5, nsItems: fakeNamespaces(20)},
			moveBy:         -3,
			expectedCurY:   0,
			expectedOffset: 3,
		},
		{
			name:           "move_up_too-far_with_scroll",
			frame:          InfoFrame{height: 10, cursorY: 3, scrollYOffset: 5, nsItems: fakeNamespaces(20)},
			moveBy:         -20,
			expectedCurY:   0,
			expectedOffset: 0,
		},
		{
			name:           "move_up_when_cursor_is_outside_available_positions",
			frame:          InfoFrame{height: 10, cursorY: 10, scrollYOffset: 15, nsItems: fakeNamespaces(20)},
			moveBy:         -15,
			expectedCurY:   0,
			expectedOffset: 10,
		},
	}

	for index, tc := range testTable {
		t.Run(fmt.Sprintf("%v %v", index, tc.name), func(t *testing.T) {
			tc.frame.updatePositions()
			tc.frame.moveCursor(screen, tc.moveBy)
			if tc.frame.cursorY != tc.expectedCurY {
				t.Errorf("Invalid Cursor Y position. Want: %v, Got: %v", tc.expectedCurY, tc.frame.cursorY)
			}
			if tc.frame.scrollYOffset != tc.expectedOffset {
				t.Errorf("Invalid Offset. Want: %v, Got: %v", tc.expectedOffset, tc.frame.scrollYOffset)
			}
		})
	}
}

func fakeNamespaces(count int) []Namespace {
	ns := make([]Namespace, count)
	for index, _ := range ns {
		ns[index] = Namespace{name: strconv.Itoa(index), context: "context"}
	}
	return ns
}
