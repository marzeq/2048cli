package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

func enableAlternateScreen() {
	fmt.Fprint(os.Stdout, "\033[?1049h\033[H")
}

func disableAlternateScreen() {
	fmt.Fprint(os.Stdout, "\033[?1049l")
}

func clearScreen() {
	fmt.Fprint(os.Stdout, "\033[2J\033[H")
}

func die(errc int) {
	clearScreen()
	disableAlternateScreen()
	os.Exit(errc)
}

func setupSignalHandler() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-c
		die(0)
	}()
}

func getch() byte {
	exec.Command("stty", "-F", "/dev/tty", "cbreak", "min", "1").Run()
	exec.Command("stty", "-F", "/dev/tty", "-echo").Run()

	var b []byte = make([]byte, 1)
	os.Stdin.Read(b)

	exec.Command("stty", "-F", "/dev/tty", "sane").Run()

	return b[0]
}

type Winsize struct {
	Rows uint16
	Cols uint16
	// Unused, but part of the struct
	X uint16
	Y uint16
}

func GetTerminalSize() (*Winsize, error) {
	ws := &Winsize{}
	retCode, _, errno := syscall.Syscall(
		syscall.SYS_IOCTL,
		os.Stdout.Fd(),
		uintptr(syscall.TIOCGWINSZ),
		uintptr(unsafe.Pointer(ws)),
	)
	if int(retCode) == -1 {
		return nil, errno
	}
	return ws, nil
}

const (
	SIZE          = 4
	WIN_CONDITION = 2048
)

type Board [SIZE][SIZE]uint

type GameState struct {
	board               Board
	previousBoard       Board
	canGoBack           bool
	message             string
	reachedWinCondition bool
}

func NewBoard() Board {
	board := Board{}

	for i := 0; i < SIZE; i++ {
		for j := 0; j < SIZE; j++ {
			board[i][j] = 0
		}
	}

	return board
}

func NewGameState() GameState {
	g := GameState{
		board:               NewBoard(),
		previousBoard:       Board{},
		canGoBack:           false,
		message:             "",
		reachedWinCondition: false,
	}

	g.SpawnRandomNumber()
	g.SpawnRandomNumber()

	return g
}

func (g *GameState) GridString(paddingMaybe ...string) string {
	padding := ""

	if len(paddingMaybe) > 0 {
		padding = paddingMaybe[0]
	}

	s := ""

	maxNumLen := len(strconv.FormatUint(uint64(g.board.max()), 10))

	for i := 0; i < SIZE; i++ {
		dashes := SIZE*(maxNumLen+3) + 1
		dashStr := padding + strings.Repeat("-", dashes)
		s += dashStr + "\n"

		s += padding
		for j := 0; j < SIZE; j++ {
			numStr := strconv.FormatUint(uint64(g.board[i][j]), 10)
			remaining := maxNumLen - len(numStr)
			padForNum := remaining / 2
			s += "| "
			paddedNum := strings.Repeat(" ", padForNum) + numStr + strings.Repeat(" ", padForNum)
			if remaining%2 != 0 {
				paddedNum = " " + paddedNum
			}
			var num string
			switch g.board[i][j] {
			case 0:
				num = strings.Repeat(" ", maxNumLen)
			case 2:
				num = "\033[30;48;5;231m" + paddedNum + "\033[0m"
			case 4:
				num = "\033[30;48;5;230m" + paddedNum + "\033[0m"
			case 8:
				num = "\033[30;48;5;216m" + paddedNum + "\033[0m"
			case 16:
				num = "\033[30;48;5;209m" + paddedNum + "\033[0m"
			case 32:
				num = "\033[30;48;5;167m" + paddedNum + "\033[0m"
			case 64:
				num = "\033[30;48;5;203m" + paddedNum + "\033[0m"
			case 128:
				num = "\033[30;48;5;222m" + paddedNum + "\033[0m"
			case 256:
				num = "\033[30;48;5;221m" + paddedNum + "\033[0m"
			case 512:
				fallthrough
			case 1024:
				num = "\033[30;48;5;178m" + paddedNum + "\033[0m"
			case 2048:
				num = "\033[30;48;5;214m" + paddedNum + "\033[0m"
			default:
				num = "\033[1;48;5;237m" + paddedNum + "\033[0m"
			}
			s += num
			s += " "
		}
		s += "|"

		if i < SIZE-1 {
			s += "\n"
		} else {
			s += "\n" + dashStr
		}
	}

	return s
}

func (g *GameState) Display() {
	ws, err := GetTerminalSize()
	if err != nil {
		fmt.Println(err)
	}
	cols, rows := int(ws.Cols), int(ws.Rows)

	clearScreen()

	strSplit := strings.Split(g.GridString(), "\n")
	height := len(strSplit)
	width := len(strSplit[0])

	guide := "q - quit, arrows/vim keys/wasd - move, r - restart"

	if g.canGoBack {
		guide += ", b - undo"
	}

	if height > rows-3 || max(width, len(guide)) > cols {
		fmt.Printf("Terminal too small to display game state, must be at least %dx%d\n", max(width, len(guide)), height+3)
		return
	}

	leftPadLen := (cols - width) / 2
	topPadLen := (rows - height) / 2

	leftPad := strings.Repeat(" ", leftPadLen)
	topPad := strings.Repeat("\n", topPadLen)

	fmt.Println(topPad + g.GridString(leftPad))

	guidePad := strings.Repeat(" ", (cols-len(guide))/2)
	fmt.Println(guidePad + guide)

	if g.message != "" {
		msgPad := strings.Repeat(" ", (cols-len(g.message))/2)
		fmt.Println(msgPad + g.message)
	}
}

func (g *GameState) SpawnRandomNumber() bool {
	randNum := rand.Intn(10)

	var val uint = 2

	if randNum == 0 || randNum == 1 {
		val = 4
	}

	emptyPositions := make([][2]int, 0)

	for i := 0; i < SIZE; i++ {
		for j := 0; j < SIZE; j++ {
			if g.board[i][j] == 0 {
				emptyPositions = append(emptyPositions, [2]int{i, j})
			}
		}
	}

	if len(emptyPositions) == 0 {
		return false
	}

	pos := emptyPositions[rand.Intn(len(emptyPositions))]
	g.board[pos[0]][pos[1]] = val

	return true
}

func (b *Board) Equals(other Board) bool {
	for i := 0; i < SIZE; i++ {
		for j := 0; j < SIZE; j++ {
			if b[i][j] != other[i][j] {
				return false
			}
		}
	}

	return true
}

func (b *Board) CanMoveUp() bool {
	for j := 0; j < SIZE; j++ {
		for i := 1; i < SIZE; i++ {
			if b[i][j] != 0 {
				if b[i-1][j] == 0 || b[i-1][j] == b[i][j] {
					return true
				}
			}
		}
	}

	return false
}

func (b *Board) CanMoveDown() bool {
	for j := 0; j < SIZE; j++ {
		for i := SIZE - 2; i >= 0; i-- {
			if b[i][j] != 0 {
				if b[i+1][j] == 0 || b[i+1][j] == b[i][j] {
					return true
				}
			}
		}
	}

	return false
}

func (b *Board) CanMoveRight() bool {
	for i := 0; i < SIZE; i++ {
		for j := SIZE - 2; j >= 0; j-- {
			if b[i][j] != 0 {
				if b[i][j+1] == 0 || b[i][j+1] == b[i][j] {
					return true
				}
			}
		}
	}

	return false
}

func (b *Board) CanMoveLeft() bool {
	for i := 0; i < SIZE; i++ {
		for j := 1; j < SIZE; j++ {
			if b[i][j] != 0 {
				if b[i][j-1] == 0 || b[i][j-1] == b[i][j] {
					return true
				}
			}
		}
	}

	return false
}

func (g *GameState) MoveUp() bool {
	if !g.board.CanMoveUp() {
		return false
	}

	newBoard := g.board

	for j := 0; j < SIZE; j++ {
		for i := 0; i < SIZE; i++ {
			for k := i + 1; k < SIZE; k++ {
				if newBoard[k][j] != 0 {
					if newBoard[i][j] == 0 {
						newBoard[i][j] = newBoard[k][j]
						newBoard[k][j] = 0
						i--
					} else if newBoard[i][j] == newBoard[k][j] {
						newBoard[i][j] *= 2
						newBoard[k][j] = 0
					}
					break
				}
			}
		}
	}

	if !g.board.Equals(newBoard) {
		g.previousBoard = g.board
		g.board = newBoard
		return true
	}

	return false
}

func (g *GameState) MoveDown() bool {
	if !g.board.CanMoveDown() {
		return false
	}

	newBoard := g.board

	for j := 0; j < SIZE; j++ {
		for i := SIZE - 1; i >= 0; i-- {
			for k := i - 1; k >= 0; k-- {
				if newBoard[k][j] != 0 {
					if newBoard[i][j] == 0 {
						newBoard[i][j] = newBoard[k][j]
						newBoard[k][j] = 0
						i++
					} else if newBoard[i][j] == newBoard[k][j] {
						newBoard[i][j] *= 2
						newBoard[k][j] = 0
					}
					break
				}
			}
		}
	}

	if !g.board.Equals(newBoard) {
		g.previousBoard = g.board
		g.board = newBoard
		return true
	}

	return false
}

func (g *GameState) MoveRight() bool {
	if !g.board.CanMoveRight() {
		return false
	}

	newBoard := g.board

	for i := 0; i < SIZE; i++ {
		for j := SIZE - 1; j >= 0; j-- {
			for k := j - 1; k >= 0; k-- {
				if newBoard[i][k] != 0 {
					if newBoard[i][j] == 0 {
						newBoard[i][j] = newBoard[i][k]
						newBoard[i][k] = 0
						j++
					} else if newBoard[i][j] == newBoard[i][k] {
						newBoard[i][j] *= 2
						newBoard[i][k] = 0
					}
					break
				}
			}
		}
	}

	if !g.board.Equals(newBoard) {
		g.previousBoard = g.board
		g.board = newBoard
		return true
	}

	return false
}

func (g *GameState) MoveLeft() bool {
	if !g.board.CanMoveLeft() {
		return false
	}

	newBoard := g.board

	for i := 0; i < SIZE; i++ {
		for j := 0; j < SIZE; j++ {
			for k := j + 1; k < SIZE; k++ {
				if newBoard[i][k] != 0 {
					if newBoard[i][j] == 0 {
						newBoard[i][j] = newBoard[i][k]
						newBoard[i][k] = 0
						j--
					} else if newBoard[i][j] == newBoard[i][k] {
						newBoard[i][j] *= 2
						newBoard[i][k] = 0
					}
					break
				}
			}
		}
	}

	if !g.board.Equals(newBoard) {
		g.previousBoard = g.board
		g.board = newBoard
		return true
	}

	return false
}

func (b *Board) max() uint {
	m := uint(0)

	for i := 0; i < SIZE; i++ {
		for j := 0; j < SIZE; j++ {
			if b[i][j] > m {
				m = b[i][j]
			}
		}
	}

	return m
}

func main() {
	enableAlternateScreen()
	defer disableAlternateScreen()

	g := NewGameState()

	go func() {
		for {
			if !g.board.CanMoveUp() && !g.board.CanMoveDown() && !g.board.CanMoveRight() && !g.board.CanMoveLeft() {
				g.message = "game over"
				g.Display()
				for {
					confirm := getch()
					if confirm == 'r' {
						g = NewGameState()
						break
					} else if confirm == 'q' {
						die(0)
					} else if confirm == 'b' && g.canGoBack {
						g.board, g.previousBoard = g.previousBoard, g.board
						g.canGoBack = false
						break
					}
				}
			}

			if g.reachedWinCondition || g.board.max() >= WIN_CONDITION {
				g.message = "game has been won, playing in endless mode"
				g.reachedWinCondition = true
				g.Display()
			}

			ch := getch()

			if ch == 27 {
				next := getch()
				if next == 91 {
					direction := getch()
					switch direction {
					case 65:
						if g.MoveUp() {
							g.canGoBack = true
							g.SpawnRandomNumber()
						}
					case 66:
						if g.MoveDown() {
							g.canGoBack = true
							g.SpawnRandomNumber()
						}
					case 67:
						if g.MoveRight() {
							g.canGoBack = true
							g.SpawnRandomNumber()
						}
					case 68:
						if g.MoveLeft() {
							g.canGoBack = true
							g.SpawnRandomNumber()
						}
					}
				}
				continue
			}

			switch ch {
			case 'q':
				die(0)
			case 'r':
				g.message = "are you sure? (y/n)"
				g.Display()
				for {
					confirm := getch()
					if confirm == 'y' {
						g = NewGameState()
						break
					} else if confirm == 'n' {
						g.message = ""
						break
					}
				}
			case 'b':
				if g.canGoBack {
					g.board, g.previousBoard = g.previousBoard, g.board
					g.canGoBack = false
				}
			case 'w':
				fallthrough
			case 'k':
				if g.MoveUp() {
					g.canGoBack = true
					g.SpawnRandomNumber()
				}
			case 's':
				fallthrough
			case 'j':
				if g.MoveDown() {
					g.canGoBack = true
					g.SpawnRandomNumber()
				}
			case 'd':
				fallthrough
			case 'l':
				if g.MoveRight() {
					g.canGoBack = true
					g.SpawnRandomNumber()
				}
			case 'a':
				fallthrough
			case 'h':
				if g.MoveLeft() {
					g.canGoBack = true
					g.SpawnRandomNumber()
				}
			}
		}
	}()

	for {
		g.Display()
		time.Sleep(100 * time.Millisecond)
	}
}
