package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/sweetfish329/go/wasm-kifu/pkg/engine"
)

type GroundTruth struct {
	BoardSize int          `json:"boardSize"`
	Grid      [19][19]int8 `json:"grid"`
}

func main() {
	sgfData, err := os.ReadFile("../test3.sgf")
	if err != nil {
		sgfData, err = os.ReadFile("test3.sgf")
		if err != nil {
			panic(err)
		}
	}

	ge := engine.NewGame()
	if err := ge.ImportSGF(string(sgfData)); err != nil {
		panic(err)
	}

	state := ge.GetBoardState()
	fmt.Printf("Move number: %d\n", ge.MoveNumber())

	var gt GroundTruth
	gt.BoardSize = 19
	blackCount := 0
	whiteCount := 0

	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			// rooklift/sgf is State[x][y] (x=0..18, y=0..18)
			val := state.Stones[x][y]
			gt.Grid[y][x] = val
			if val == 1 {
				blackCount++
			} else if val == 2 {
				whiteCount++
			}
		}
	}

	fmt.Printf("Stones count - Black: %d, White: %d, Total: %d\n", blackCount, whiteCount, blackCount+whiteCount)

	cols := "   A B C D E F G H J K L M N O P Q R S T"
	fmt.Println(cols)
	for y := 0; y < 19; y++ {
		rowStr := fmt.Sprintf("%2d ", 19-y)
		for x := 0; x < 19; x++ {
			switch gt.Grid[y][x] {
			case 1:
				rowStr += "● "
			case 2:
				rowStr += "○ "
			default:
				rowStr += ". "
			}
		}
		fmt.Printf("%s%2d\n", rowStr, 19-y)
	}
	fmt.Println(cols)

	out, err := json.MarshalIndent(gt, "", "  ")
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile("test3_ground_truth.json", out, 0644); err != nil {
		panic(err)
	}
	fmt.Println("Saved to test3_ground_truth.json")
}
