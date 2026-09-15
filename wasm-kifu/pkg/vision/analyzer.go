// Package vision は碁盤画像の多次元特徴量解析および着手推定ロジックを提供する。
// NotebookLM学術調査に基づく「ドーナツ走査・多次元特徴ベクトル (L,C,D,H,S)・動的平均石 (Dani Ferrari手法)・ミニマル差分走査」を実装。
package vision

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"

	"github.com/sweetfish329/go/wasm-kifu/pkg/engine"
)

// BoardSize は碁盤のサイズ（19路盤固定）
const BoardSize = 19

// IntersectionState は各交点の状態
type IntersectionState int8

const (
	Empty IntersectionState = 0
	Black IntersectionState = 1
	White IntersectionState = 2
)

// DetectedBoard はOpenCV.jsから受け取る簡易盤面データ (Phase 1 互換)
type DetectedBoard struct {
	// Intersections は19×19の交点状態配列 (row-major: [y*19+x])
	Intersections [BoardSize * BoardSize]int8 `json:"intersections"`
}

// IntersectionFeatures は1つの交点から抽出された多次元特徴量
type IntersectionFeatures struct {
	X     int     `json:"x"`
	Y     int     `json:"y"`
	L     float64 `json:"l"` // Luminance (輝度: 0-255)
	C     float64 `json:"c"` // Chrominance (RGB標準偏差: 木目は高, 石は低/無彩色)
	D     float64 `json:"d"` // Disuniformity (らせん不均一性: 木目+線は高, 石は低)
	H     float64 `json:"h"` // Hue (色相: 0-360)
	S     float64 `json:"s"` // Saturation (彩度: 0-255)
	R     float64 `json:"r"` // Red channel (0-255: skolchin/gbr 白石評価用)
	B     float64 `json:"b"` // Blue channel (0-255: skolchin/gbr 黒石評価用)
	RMB   float64 `json:"rmb"` // Woodness (R - B: 木目度は高, 石は低/無彩色)
	CR    float64 `json:"cR,omitempty"` // 中心画素 Red
	CB    float64 `json:"cB,omitempty"` // 中心画素 Blue
	CRMB  float64 `json:"cRmb,omitempty"` // 中心画素 R - B
	AvgR  float64 `json:"avgR,omitempty"` // 近傍平均 Red
	AvgB  float64 `json:"avgB,omitempty"` // 近傍平均 Blue
	MaxDW float64 `json:"maxDW,omitempty"` // Distance Transform max value for white (密集白石サルベージ用)
	MaxDB float64 `json:"maxDB,omitempty"` // Distance Transform max value for black (密集黒石サルベージ用)
}

// BoardFeaturesData は全交点の多次元特徴量データ
type BoardFeaturesData struct {
	Features [BoardSize * BoardSize]IntersectionFeatures `json:"features"`
}

// DetectedMove は検出された着手
type DetectedMove struct {
	X     int     `json:"x"`
	Y     int     `json:"y"`
	Color string  `json:"color"` // "B" or "W"
	Score float64 `json:"score,omitempty"`
}

// AnalysisResult は盤面解析の結果
type AnalysisResult struct {
	Moves      []DetectedMove `json:"moves"`
	Removed    []DetectedMove `json:"removed,omitempty"` // 取られた石（アゲハマ）
	Errors     []string       `json:"errors,omitempty"`
	Confidence float64        `json:"confidence"`
}

// FeatureVector は計算用の特徴ベクトル
type FeatureVector struct {
	L float64
	C float64
	D float64
	H float64
	S float64
}

// MeanStoneModel は動的に算出された「平均石」と「平均空交点」のモデル
type MeanStoneModel struct {
	BlackMean  FeatureVector
	WhiteMean  FeatureVector
	EmptyMean  FeatureVector
	BlackCount int
	WhiteCount int
	EmptyCount int
}

// defaultPriors は初期状態（石が少ない場合）の事前分布パラメータ
var defaultBlackPrior = FeatureVector{L: 35.0, C: 4.0, D: 25.0, H: 0.0, S: 15.0}
var defaultWhitePrior = FeatureVector{L: 215.0, C: 6.0, D: 30.0, H: 45.0, S: 25.0}
var defaultEmptyPrior = FeatureVector{L: 140.0, C: 28.0, D: 160.0, H: 38.0, S: 120.0}

// computeFeatureDistance は2つの特徴ベクトルの正規化重み付きユークリッド距離を計算する
func computeFeatureDistance(v1, v2 FeatureVector) float64 {
	dL := (v1.L - v2.L) / 255.0
	dC := (v1.C - v2.C) / 60.0  // 色度差（木目 vs 石）
	dD := (v1.D - v2.D) / 300.0 // 不均一性差（線・木目テクスチャ vs 平滑な石）
	dS := (v1.S - v2.S) / 255.0 // 彩度差

	// クロミナンス(C)と不均一性(D)に高い重みを与え、影と黒石を強固に分離
	wL := 1.2
	wC := 2.2
	wD := 1.8
	wS := 1.0

	return math.Sqrt(wL*dL*dL + wC*dC*dC + wD*dD*dD + wS*dS*dS)
}

// buildDynamicMeanModel は現在の盤面状態と特徴量から動的平均モデルを構築する
func buildDynamicMeanModel(currentState *engine.BoardState, features *[BoardSize * BoardSize]IntersectionFeatures) MeanStoneModel {
	model := MeanStoneModel{}

	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			idx := y*BoardSize + x
			f := features[idx]
			stone := currentState.Stones[x][y]

			fv := FeatureVector{L: f.L, C: f.C, D: f.D, H: f.H, S: f.S}

			switch stone {
			case 1: // 黒石
				model.BlackMean.L += fv.L
				model.BlackMean.C += fv.C
				model.BlackMean.D += fv.D
				model.BlackMean.S += fv.S
				model.BlackCount++
			case 2: // 白石
				model.WhiteMean.L += fv.L
				model.WhiteMean.C += fv.C
				model.WhiteMean.D += fv.D
				model.WhiteMean.S += fv.S
				model.WhiteCount++
			default: // 空交点
				model.EmptyMean.L += fv.L
				model.EmptyMean.C += fv.C
				model.EmptyMean.D += fv.D
				model.EmptyMean.S += fv.S
				model.EmptyCount++
			}
		}
	}

	// 平均値の算出（存在しない場合は事前分布で補完/ブレンド）
	if model.BlackCount > 0 {
		model.BlackMean.L /= float64(model.BlackCount)
		model.BlackMean.C /= float64(model.BlackCount)
		model.BlackMean.D /= float64(model.BlackCount)
		model.BlackMean.S /= float64(model.BlackCount)
	} else {
		model.BlackMean = defaultBlackPrior
	}

	if model.WhiteCount > 0 {
		model.WhiteMean.L /= float64(model.WhiteCount)
		model.WhiteMean.C /= float64(model.WhiteCount)
		model.WhiteMean.D /= float64(model.WhiteCount)
		model.WhiteMean.S /= float64(model.WhiteCount)
	} else {
		model.WhiteMean = defaultWhitePrior
	}

	if model.EmptyCount > 0 {
		model.EmptyMean.L /= float64(model.EmptyCount)
		model.EmptyMean.C /= float64(model.EmptyCount)
		model.EmptyMean.D /= float64(model.EmptyCount)
		model.EmptyMean.S /= float64(model.EmptyCount)
	} else {
		model.EmptyMean = defaultEmptyPrior
	}

	return model
}

type candidateScore struct {
	x, y       int
	distStone  float64
	distEmpty  float64
	distRatio  float64 // distStone / distEmpty
	color      string
}

// AnalyzeBoardFeatures は多次元特徴量と動的平均石（Dani Ferrari手法）および
// ミニマル・アプローチを用いて新しい着手を高精度に推定する
func AnalyzeBoardFeatures(ge *engine.GameEngine, featuresJSON string) string {
	var data BoardFeaturesData
	if err := json.Unmarshal([]byte(featuresJSON), &data); err != nil {
		result := AnalysisResult{
			Errors: []string{fmt.Sprintf("JSON parse error: %v", err)},
		}
		res, _ := json.Marshal(result)
		return string(res)
	}

	currentState := ge.GetBoardState()

	expectedColor := "B"
	if currentState.Player == 2 {
		expectedColor = "W"
	}

	// 1. 動的平均石モデルを構築
	model := buildDynamicMeanModel(&currentState, &data.Features)

	// 2. ミニマル・アプローチ: 空の交点のみを候補として走査
	var candidates []candidateScore

	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			if currentState.Stones[x][y] != 0 {
				continue // 既存の石はスキップ
			}

			f := data.Features[y*BoardSize+x]
			// 特徴量の補正フォールバック
			if f.L == 0 && (f.CR > 0 || f.CB > 0) {
				f.L = (f.CR + f.CB) / 2.0
			}
			if f.R == 0 && f.AvgR > 0 {
				f.R = f.AvgR
			}
			if f.B == 0 && f.AvgB > 0 {
				f.B = f.AvgB
			}
			if f.RMB == 0 && (f.AvgR > 0 || f.AvgB > 0) {
				f.RMB = f.AvgR - f.AvgB
			}

			fv := FeatureVector{L: f.L, C: f.C, D: f.D, H: f.H, S: f.S}

			// 期待される手番の石との距離
			targetStoneMean := model.BlackMean
			if expectedColor == "W" {
				targetStoneMean = model.WhiteMean
			}

			dStone := computeFeatureDistance(fv, targetStoneMean)
			dEmpty := computeFeatureDistance(fv, model.EmptyMean)

			// 赤青チャンネル分離 ＋ 距離変換ピークによる光学スコア判定
			opticalStoneMatch := false
			if expectedColor == "B" {
				// 黒石判定: Blue低輝度, Red低輝度, または MaxDB が突出
				if (f.CB < 90 && f.CR < 120 && f.RMB < 28) || (f.MaxDB >= 2.5 && f.MaxDB > f.MaxDW && f.RMB < 30) || (f.CR < 90 && f.CB < 80 && f.CRMB < 25) {
					opticalStoneMatch = true
				}
			} else {
				// 白石判定: Red高輝度, または MaxDW が突出
				if (f.CR > 175 && f.CB > 160 && f.CRMB < 18) || (f.MaxDW >= 2.0 && f.MaxDW > f.MaxDB && f.RMB <= 25) || (f.AvgR >= 135 && f.RMB <= 18) {
					opticalStoneMatch = true
				}
			}

			if opticalStoneMatch {
				// 光学的に手番の石と極めて強く合致する場合、距離を大幅に縮小
				dStone = math.Min(dStone * 0.35, 0.12)
			}

			ratio := 999.0
			if dEmpty > 1e-6 {
				ratio = dStone / dEmpty
			}

			candidates = append(candidates, candidateScore{
				x:         x,
				y:         y,
				distStone: dStone,
				distEmpty: dEmpty,
				distRatio: ratio,
				color:     expectedColor,
			})
		}
	}

	result := AnalysisResult{
		Confidence: 1.0,
	}

	if len(candidates) == 0 {
		result.Errors = append(result.Errors, "no empty intersections available")
		res, _ := json.Marshal(result)
		return string(res)
	}

	// 石への距離が近い順にソート
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].distStone < candidates[j].distStone
	})

	// 3. Dani Ferrari の 3 判定基準を評価
	best := candidates[0]

	// 基準 (a): 石候補への距離が空の交点への距離の 2/3 以下、または明確に石に近い
	criterionA := (best.distRatio <= 0.67) || (best.distStone < 0.28 && best.distStone < best.distEmpty)

	// 基準 (b): 後続交点群の平均距離との比較（突出性の確認）
	sampleCount := 15
	if len(candidates)-1 < sampleCount {
		sampleCount = len(candidates) - 1
	}

	criterionB := false
	if sampleCount > 0 {
		sumSubsequent := 0.0
		for i := 1; i <= sampleCount; i++ {
			sumSubsequent += candidates[i].distStone
		}
		avgSubsequent := sumSubsequent / float64(sampleCount)
		criterionB = best.distStone <= avgSubsequent*0.67
	} else {
		criterionB = true
	}

	if criterionA && criterionB {
		result.Moves = append(result.Moves, DetectedMove{
			X:     best.x,
			Y:     best.y,
			Color: best.color,
			Score: best.distStone,
		})
	} else if best.distStone < 0.22 && best.distEmpty > 0.30 {
		// 強い石の特徴（セーフティネット）
		result.Moves = append(result.Moves, DetectedMove{
			X:     best.x,
			Y:     best.y,
			Color: best.color,
			Score: best.distStone,
		})
		result.Confidence = 0.85
	} else {
		result.Errors = append(result.Errors, "no new moves identified by dynamic mean criteria")
	}

	// 4. アゲハマ（取られた石）の検出走査
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			currentStone := currentState.Stones[x][y]
			if currentStone == 0 {
				continue
			}

			f := data.Features[y*BoardSize+x]
			fv := FeatureVector{L: f.L, C: f.C, D: f.D, H: f.H, S: f.S}

			dEmpty := computeFeatureDistance(fv, model.EmptyMean)
			var dStone float64
			if currentStone == 1 {
				dStone = computeFeatureDistance(fv, model.BlackMean)
			} else {
				dStone = computeFeatureDistance(fv, model.WhiteMean)
			}

			// 石が置かれていたはずの場所が空交点に近くなった場合
			if dEmpty < dStone*0.7 && dEmpty < 0.30 {
				colorStr := "B"
				if currentStone == 2 {
					colorStr = "W"
				}
				result.Removed = append(result.Removed, DetectedMove{
					X:     x,
					Y:     y,
					Color: colorStr,
					Score: dEmpty,
				})
			}
		}
	}

	res, _ := json.Marshal(result)
	return string(res)
}

// AnalyzeBoardDiff は従来の交点状態配列 (0,1,2) からの差分解析（互換用）
func AnalyzeBoardDiff(ge *engine.GameEngine, imageData string, width, height int) string {
	var detected DetectedBoard
	if err := json.Unmarshal([]byte(imageData), &detected); err != nil {
		result := AnalysisResult{
			Errors: []string{fmt.Sprintf("JSON parse error: %v", err)},
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	// 現在のゲーム盤面を取得
	currentState := ge.GetBoardState()

	result := AnalysisResult{
		Confidence: 1.0,
	}

	// 差分を計算
	var newStones []DetectedMove
	var removedStones []DetectedMove

	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			detectedState := detected.Intersections[y*BoardSize+x]
			currentStone := currentState.Stones[x][y]

			if detectedState != int8(currentStone) {
				if detectedState != 0 && currentStone == 0 {
					// 新しい石が置かれた
					color := "B"
					if detectedState == 2 {
						color = "W"
					}
					newStones = append(newStones, DetectedMove{
						X: x, Y: y, Color: color,
					})
				} else if detectedState == 0 && currentStone != 0 {
					// 石が取り除かれた（アゲハマ）
					color := "B"
					if currentStone == 2 {
						color = "W"
					}
					removedStones = append(removedStones, DetectedMove{
						X: x, Y: y, Color: color,
					})
				} else {
					// 石の色が変わった（誤検出の可能性）
					result.Errors = append(result.Errors,
						fmt.Sprintf("stone color changed at (%d,%d): %d -> %d", x, y, currentStone, detectedState))
					result.Confidence *= 0.5
				}
			}
		}
	}

	// 着手の妥当性チェック
	if len(newStones) == 0 && len(removedStones) == 0 {
		result.Errors = append(result.Errors, "no changes detected")
	} else if len(newStones) == 1 {
		result.Moves = newStones
		result.Removed = removedStones
	} else if len(newStones) > 1 {
		result.Moves = newStones
		result.Errors = append(result.Errors,
			fmt.Sprintf("multiple new stones detected (%d), possible multi-move or detection error", len(newStones)))
		result.Confidence *= 0.3
	}

	data, _ := json.Marshal(result)
	return string(data)
}

// FullBoardResult は静止画一括盤面解析の結果
type FullBoardResult struct {
	Grid       [BoardSize * BoardSize]int8 `json:"grid"` // 0:空, 1:黒, 2:白 (row-major: y*19+x)
	Confidence float64                     `json:"confidence"`
	BlackCount int                         `json:"blackCount"`
	WhiteCount int                         `json:"whiteCount"`
	EmptyCount int                         `json:"emptyCount"`
}

// AnalyzeFullBoard は全361交点の特徴量から静止画全体の盤面（石配置）を一括復元する。
// 局所背景適応、木目度(RMB)フィルタ、外周境界保護を組み合わせた高精度アンサンブル判定を行う。
func AnalyzeFullBoard(featuresJSON string) (string, error) {
	var featData BoardFeaturesData
	if err := json.Unmarshal([]byte(featuresJSON), &featData); err != nil {
		return "", fmt.Errorf("failed to parse board features: %w", err)
	}

	// 19x19 配列にマッピング
	type gridItem struct {
		f       IntersectionFeatures
		localBg float64
		normL   float64
		normB   float64
	}
	var grid [BoardSize][BoardSize]gridItem
	for i, f := range featData.Features {
		x := f.X
		y := f.Y
		if x < 0 || x >= BoardSize || y < 0 || y >= BoardSize {
			x = i % BoardSize
			y = i / BoardSize
		}
		grid[y][x].f = f
	}

	// 【skolchin/gbr Channel Splitting ＋ 局所背景適応】
	// 近傍 3 マスの中央値背景 (bgR, bgB) を算出し、照明ムラや影をキャンセルする比率 (normR, normB) を取得
	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			var rList, bList []float64
			for dy := -3; dy <= 3; dy++ {
				for dx := -3; dx <= 3; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && nx < BoardSize && ny >= 0 && ny < BoardSize {
						feat := grid[ny][nx].f
						rVal := feat.R
						if rVal == 0 {
							rVal = feat.L
						}
						bVal := feat.B
						if bVal == 0 {
							bVal = feat.L
						}
						rList = append(rList, rVal)
						bList = append(bList, bVal)
					}
				}
			}
			sort.Float64s(rList)
			sort.Float64s(bList)
			bgR := rList[len(rList)/2]
			bgB := bList[len(bList)/2]
			if bgR < 15 {
				bgR = 15
			}
			if bgB < 15 {
				bgB = 15
			}

			feat := grid[y][x].f
			rVal := feat.R
			if rVal == 0 {
				rVal = feat.L
			}
			bVal := feat.B
			if bVal == 0 {
				bVal = feat.L
			}

			grid[y][x].localBg = bgR
			grid[y][x].normL = rVal / bgR // normR
			grid[y][x].normB = bVal / bgB // normB
		}
	}

	var res FullBoardResult
	res.Confidence = 0.95

	for y := 0; y < BoardSize; y++ {
		for x := 0; x < BoardSize; x++ {
			item := grid[y][x]
			f := item.f
			normR := item.normL
			normB := item.normB
			rVal := f.R
			if rVal == 0 {
				rVal = f.L
			}
			bVal := f.B
			if bVal == 0 {
				bVal = f.L
			}
			rmb := rVal - bVal

			stone := int8(0)

			// 1. 白石判定: 低木目度 (無彩色) かつ 高輝度、局所背景青成分より高い
			if rmb < 32.0 && rVal >= 85.0 && (normB > 1.12 || (normR > 1.04 && rmb < 20.0)) {
				stone = 2
			} else if rVal >= 135.0 && rmb <= 22.0 {
				stone = 2
			}

			// 2. 黒石判定: 低木目度 (無彩色) かつ 低輝度、局所背景より暗い
			if stone == 0 {
				if rmb < 35.0 {
					if rVal < 75.0 && (normR < 0.90 || normB < 0.90) {
						stone = 1
					} else if normB < 0.82 && normR < 0.88 {
						stone = 1
					} else if bVal < 45.0 && rVal < 75.0 && rmb < 25.0 {
						stone = 1
					}
				}
			}

			// 距離変換ピークによるサルベージ（密集石）
			if stone == 0 {
				if f.MaxDW >= 4.0 && f.MaxDW > f.MaxDB && rmb <= 25.0 {
					stone = 2
				} else if f.MaxDB >= 3.5 && f.MaxDB > f.MaxDW && rmb < 30.0 && (rVal < 90.0 || normB < 0.92) {
					stone = 1
				}
			}

			res.Grid[y*BoardSize+x] = stone
		}
	}

	// カウントの集計
	res.BlackCount = 0
	res.WhiteCount = 0
	res.EmptyCount = 0
	for _, s := range res.Grid {
		switch s {
		case 1:
			res.BlackCount++
		case 2:
			res.WhiteCount++
		default:
			res.EmptyCount++
		}
	}

	data, err := json.Marshal(res)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

