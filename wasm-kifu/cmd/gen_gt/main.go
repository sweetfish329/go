package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/sweetfish329/go/wasm-kifu/pkg/engine"
)

type GroundTruth struct {
	BoardSize int          `json:"boardSize"`
	Grid      [19][19]int8 `json:"grid"`
}

func findFile(filename string) string {
	candidates := []string{
		filename,
		filepath.Join("..", filename),
		filepath.Join("../..", filename),
		filepath.Join("wasm-kifu", filename),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return filename
}

func main() {
	sgfPath := flag.String("sgf", "test3.sgf", "Path to SGF file")
	outPath := flag.String("out", "", "Path to output ground truth JSON (default: <sgf_basename>_ground_truth.json)")
	flag.Parse()

	targetSgf := findFile(*sgfPath)
	sgfData, err := os.ReadFile(targetSgf)
	if err != nil {
		fmt.Printf("Error reading %s: %v\n", targetSgf, err)
		os.Exit(1)
	}

	ge := engine.NewGame()
	if err := ge.ImportSGF(string(sgfData)); err != nil {
		fmt.Printf("Error importing SGF %s: %v\n", targetSgf, err)
		os.Exit(1)
	}

	state := ge.GetBoardState()
	fmt.Printf("=== SGF 解析: %s ===\n", targetSgf)
	fmt.Printf("総手数: %d 手\n", ge.MoveNumber())

	var gt GroundTruth
	gt.BoardSize = 19
	blackCount := 0
	whiteCount := 0

	for y := 0; y < 19; y++ {
		for x := 0; x < 19; x++ {
			val := state.Stones[x][y]
			gt.Grid[y][x] = val
			if val == 1 {
				blackCount++
			} else if val == 2 {
				whiteCount++
			}
		}
	}

	emptyCount := 361 - (blackCount + whiteCount)
	fmt.Printf("石数内訳 - 黒石: %d, 白石: %d, 空交点: %d (合計: %d)\n",
		blackCount, whiteCount, emptyCount, 361)

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

	targetOut := *outPath
	if targetOut == "" {
		base := filepath.Base(targetSgf)
		ext := filepath.Ext(base)
		name := strings.TrimSuffix(base, ext)
		targetOut = name + "_ground_truth.json"
	}

	out, err := json.MarshalIndent(gt, "", "  ")
	if err != nil {
		panic(err)
	}

	if err := os.WriteFile(targetOut, out, 0644); err != nil {
		panic(err)
	}
	fmt.Printf("正解データを保存しました: %s\n", targetOut)
}
