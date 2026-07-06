package sgf

// Go Board simulator to replicate final state for OGP preview

type Point struct {
	X, Y int
}

type BoardState struct {
	Size        int
	Grid        [][]int // 0: empty, 1: black, 2: white
	MoveNumbers [][]int // 0: none, otherwise the move number (1-indexed)
}

func NewBoard(size int) *BoardState {
	grid := make([][]int, size)
	moveNums := make([][]int, size)
	for i := range grid {
		grid[i] = make([]int, size)
		moveNums[i] = make([]int, size)
	}
	return &BoardState{Size: size, Grid: grid, MoveNumbers: moveNums}
}

// Convert SGF coordinate (e.g. "pd") to (x, y)
func parseCoords(val string) (int, int, bool) {
	if len(val) != 2 {
		return 0, 0, false // Pass or invalid
	}
	x := int(val[0] - 'a')
	y := int(val[1] - 'a')
	return x, y, true
}

func (b *BoardState) getGroup(startX, startY int, visited [][]bool) ([]Point, int) {
	color := b.Grid[startY][startX]
	if color == 0 {
		return nil, 0
	}

	group := []Point{}
	queue := []Point{{X: startX, Y: startY}}
	visited[startY][startX] = true

	liberties := make(map[Point]bool)
	dirs := []Point{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}

	for len(queue) > 0 {
		p := queue[0]
		queue = queue[1:]
		group = append(group, p)

		for _, d := range dirs {
			nx, ny := p.X+d.X, p.Y+d.Y
			if nx >= 0 && nx < b.Size && ny >= 0 && ny < b.Size {
				if b.Grid[ny][nx] == 0 {
					liberties[Point{nx, ny}] = true
				} else if b.Grid[ny][nx] == color && !visited[ny][nx] {
					visited[ny][nx] = true
					queue = append(queue, Point{nx, ny})
				}
			}
		}
	}

	return group, len(liberties)
}

// PlaceStoneWithNumber places a stone, records its move number, and resolves captures/suicide.
func (b *BoardState) PlaceStoneWithNumber(x, y int, color int, moveNum int) {
	if x < 0 || x >= b.Size || y < 0 || y >= b.Size {
		return
	}
	b.Grid[y][x] = color
	b.MoveNumbers[y][x] = moveNum

	opponent := 1
	if color == 1 {
		opponent = 2
	}

	dirs := []Point{{0, 1}, {0, -1}, {1, 0}, {-1, 0}}
	captured := []Point{}
	globalVisited := make([][]bool, b.Size)
	for i := range globalVisited {
		globalVisited[i] = make([]bool, b.Size)
	}

	// 1. Check adjacent opponent groups for capture
	for _, d := range dirs {
		nx, ny := x+d.X, y+d.Y
		if nx >= 0 && nx < b.Size && ny >= 0 && ny < b.Size {
			if b.Grid[ny][nx] == opponent && !globalVisited[ny][nx] {
				group, libs := b.getGroup(nx, ny, globalVisited)
				if libs == 0 {
					captured = append(captured, group...)
				}
			}
		}
	}

	// Remove captured stones
	for _, p := range captured {
		b.Grid[p.Y][p.X] = 0
		b.MoveNumbers[p.Y][p.X] = 0
	}

	// 2. Suicide rule: check if our own group has 0 liberties
	selfVisited := make([][]bool, b.Size)
	for i := range selfVisited {
		selfVisited[i] = make([]bool, b.Size)
	}
	selfGroup, selfLibs := b.getGroup(x, y, selfVisited)
	if selfLibs == 0 {
		// Suicide, remove the group
		for _, p := range selfGroup {
			b.Grid[p.Y][p.X] = 0
			b.MoveNumbers[p.Y][p.X] = 0
		}
	}
}

// PlaceStone places a stone and resolves captures and suicide rules.
func (b *BoardState) PlaceStone(x, y int, color int) {
	b.PlaceStoneWithNumber(x, y, color, 0)
}

// ReplicateGame traverses the SGF tree and populates final board state
func (b *BoardState) ReplicateGame(root *Node) {
	// Replicate handicap / setup stones first
	// AB: Add Black, AW: Add White
	if ab, ok := root.Properties["AB"]; ok {
		for _, val := range ab {
			if x, y, ok := parseCoords(val); ok {
				b.Grid[y][x] = 1
				b.MoveNumbers[y][x] = 0
			}
		}
	}
	if aw, ok := root.Properties["AW"]; ok {
		for _, val := range aw {
			if x, y, ok := parseCoords(val); ok {
				b.Grid[y][x] = 2
				b.MoveNumbers[y][x] = 0
			}
		}
	}

	// Traverse the primary path (first child)
	curr := root
	moveNum := 0
	for len(curr.Children) > 0 {
		curr = curr.Children[0]
		moveNum++

		// Check for moves (B / W)
		if bVals, ok := curr.Properties["B"]; ok && len(bVals) > 0 {
			if x, y, ok := parseCoords(bVals[0]); ok {
				b.PlaceStoneWithNumber(x, y, 1, moveNum)
			}
		} else if wVals, ok := curr.Properties["W"]; ok && len(wVals) > 0 {
			if x, y, ok := parseCoords(wVals[0]); ok {
				b.PlaceStoneWithNumber(x, y, 2, moveNum)
			}
		}
	}
}
