.PHONY: build clean serve copy-wasm-exec test vet

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
	go test ./... -v

# 静的解析
vet:
	go vet ./...
