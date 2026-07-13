package sgf

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	libsgf "github.com/rooklift/sgf"
)

// Node represents a single position/move in the SGF game tree.
type Node = libsgf.Node

// Metadata contains key information extracted from the SGF root node.
type Metadata struct {
	Size        int     `json:"size"`
	Komi        float64 `json:"komi"`
	Handicap    int     `json:"handicap"`
	BlackPlayer string  `json:"black_player"`
	BlackRank   string  `json:"black_rank"`
	WhitePlayer string  `json:"white_player"`
	WhiteRank   string  `json:"white_rank"`
	Date        string  `json:"date"`
	Result      string  `json:"result"`
}

// Parse parses an SGF string and returns the root Node of the game tree.
func Parse(sgfStr string) (*Node, error) {
	sgfStr = strings.TrimSpace(sgfStr)
	if sgfStr == "" {
		return nil, errors.New("empty SGF content")
	}
	root, err := libsgf.LoadSGF(sgfStr)
	if err != nil {
		return nil, err
	}
	return root, nil
}

// ExtractMetadata retrieves the metadata from the root node of the SGF tree.
func ExtractMetadata(n *Node) *Metadata {
	meta := &Metadata{
		Size:     19, // Default size
		Komi:     0.0,
		Handicap: 0,
	}

	if w, _ := n.RootBoardSize(); w > 0 {
		meta.Size = w
	}
	meta.Handicap = n.RootHandicap()
	meta.Komi = n.RootKomi()

	if val, ok := n.GetValue("PB"); ok {
		meta.BlackPlayer = val
	}
	if val, ok := n.GetValue("BR"); ok {
		meta.BlackRank = val
	}
	if val, ok := n.GetValue("PW"); ok {
		meta.WhitePlayer = val
	}
	if val, ok := n.GetValue("WR"); ok {
		meta.WhiteRank = val
	}
	if val, ok := n.GetValue("DT"); ok {
		meta.Date = cleanDate(val)
	}
	if val, ok := n.GetValue("RE"); ok {
		meta.Result = val
	}

	return meta
}

var dateRegex = regexp.MustCompile(`(\d{4})[-/](\d{2})[-/](\d{2})`)
var simpleDateRegex = regexp.MustCompile(`(\d{4})(\d{2})(\d{2})`)

func cleanDate(dt string) string {
	dt = strings.TrimSpace(dt)
	if dt == "" {
		return time.Now().Format("2006-01-02")
	}

	// 1. Check YYYY-MM-DD or YYYY/MM/DD
	if matches := dateRegex.FindStringSubmatch(dt); len(matches) == 4 {
		return fmt.Sprintf("%s-%s-%s", matches[1], matches[2], matches[3])
	}

	// 2. Check YYYYMMDD
	if matches := simpleDateRegex.FindStringSubmatch(dt); len(matches) == 4 {
		return fmt.Sprintf("%s-%s-%s", matches[1], matches[2], matches[3])
	}

	// 3. Check if it's just a year YYYY
	var yearRegex = regexp.MustCompile(`^(\d{4})$`)
	if yearRegex.MatchString(dt) {
		return dt + "-01-01"
	}

	// Fallback to current date
	return time.Now().Format("2006-01-02")
}
