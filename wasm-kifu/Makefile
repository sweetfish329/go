.PHONY: build clean serve copy-wasm-exec test vet eval-test3 regression

WASM_OUT = web/wasm/kifu.wasm

# Go SDK の wasm_exec.js パスを取得
GOROOT_WASM_EXEC = $(shell go env GOROOT)/lib/wasm/wasm_exec.js

# WASM ビルド（wasm_exec.js コピーも実行）
build: copy-wasm-exec
	GOOS=js GOARCH=wasm go build -o $(WASM_OUT) ./cmd/wasm/

# wasm_exec.js を Go SDK からコピー
copy-wasm-exec:
	@mkdir -p web/wasm
	cp "$(GOROOT_WASM_EXEC)" web/wasm/wasm_exec.js

# 成果物をクリーン
clean:
	rm -f $(WASM_OUT) web/wasm/wasm_exec.js

# 開発用 HTTP サーバー起動
serve:
	@echo "Starting development server on http://localhost:8080"
	@cd web && python3 -m http.server 8080

# Go テスト実行
test:
	go test ./pkg/... -v

# 静的解析
vet:
	go vet ./pkg/...

# test3 (回転画像・通常画像) 認識評価テスト実行
eval-test3:
	go run ./cmd/eval_test3/main.go

# リグレッションテスト：全対局画像の認識精度を一括検証
# 終局盤面の認識率が基準値を下回った場合は exit code 1 で報告する
regression:
	go run ./cmd/regression_test/main.go
