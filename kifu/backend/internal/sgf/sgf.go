package sgf

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Node represents a single position/move in the SGF game tree.
type Node struct {
	Properties map[string][]string
	Parent     *Node
	Children   []*Node
}

// NewNode creates a new Node.
func NewNode(parent *Node) *Node {
	return &Node{
		Properties: make(map[string][]string),
		Parent:     parent,
		Children:   make([]*Node, 0),
	}
}

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
func Parse(sgf string) (*Node, error) {
	sgf = strings.TrimSpace(sgf)
	if sgf == "" {
		return nil, errors.New("empty SGF content")
	}

	var root *Node
	var current *Node
	stack := []*Node{}

	i := 0
	n := len(sgf)

	for i < n {
		char := sgf[i]

		// Skip whitespaces
		if char == ' ' || char == '\t' || char == '\r' || char == '\n' {
			i++
			continue
		}

		switch char {
		case '(':
			// Start of a sequence/branch
			stack = append(stack, current)
			i++
		case ')':
			// End of a sequence/branch
			if len(stack) == 0 {
				return nil, errors.New("unexpected ')'")
			}
			current = stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			i++
		case ';':
			// Start of a node
			newNode := NewNode(current)
			if current != nil {
				current.Children = append(current.Children, newNode)
			} else {
				if root == nil {
					root = newNode
				}
			}
			current = newNode
			i++
		default:
			// Read Property (e.g. SZ[19], AB[pd][dp])
			if current == nil {
				return nil, fmt.Errorf("property before node definition at index %d", i)
			}

			// Read property identity (uppercase letters)
			propStart := i
			for i < n && sgf[i] >= 'A' && sgf[i] <= 'Z' {
				i++
			}
			propName := sgf[propStart:i]
			if propName == "" {
				return nil, fmt.Errorf("invalid property character at index %d: %q", i, sgf[i])
			}

			// Read values enclosed in [...]
			values := []string{}
			for i < n && sgf[i] == '[' {
				i++ // skip '['
				valStart := i
				// Handle escaped brackets inside value
				escaped := false
				for i < n {
					if escaped {
						escaped = false
						i++
						continue
					}
					if sgf[i] == '\\' {
						escaped = true
						i++
						continue
					}
					if sgf[i] == ']' {
						break
					}
					i++
				}
				if i >= n {
					return nil, fmt.Errorf("unclosed bracket for property %s at index %d", propName, valStart)
				}
				val := sgf[valStart:i]
				// Replace escaped characters
				val = strings.ReplaceAll(val, "\\]", "]")
				val = strings.ReplaceAll(val, "\\\\", "\\")
				values = append(values, val)
				i++ // skip ']'
			}

			if len(values) == 0 {
				return nil, fmt.Errorf("property %s has no values at index %d", propName, i)
			}

			current.Properties[propName] = append(current.Properties[propName], values...)
		}
	}

	if root == nil {
		return nil, errors.New("no root node found in SGF")
	}

	return root, nil
}

// ExtractMetadata retrieves the metadata from the root node of the SGF tree.
func (n *Node) ExtractMetadata() *Metadata {
	meta := &Metadata{
		Size:     19, // Default size
		Komi:     0.0,
		Handicap: 0,
	}

	// Helper to get single value
	getVal := func(key string) string {
		if vals, ok := n.Properties[key]; ok && len(vals) > 0 {
			return vals[0]
		}
		return ""
	}

	if sz := getVal("SZ"); sz != "" {
		if val, err := strconv.Atoi(sz); err == nil {
			meta.Size = val
		}
	}
	if km := getVal("KM"); km != "" {
		if val, err := strconv.ParseFloat(km, 64); err == nil {
			meta.Komi = val
		}
	}
	if ha := getVal("HA"); ha != "" {
		if val, err := strconv.Atoi(ha); err == nil {
			meta.Handicap = val
		}
	}

	meta.BlackPlayer = getVal("PB")
	meta.BlackRank = getVal("BR")
	meta.WhitePlayer = getVal("PW")
	meta.WhiteRank = getVal("WR")
	meta.Date = cleanDate(getVal("DT"))
	meta.Result = getVal("RE")

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
