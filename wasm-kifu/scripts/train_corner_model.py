#!/usr/bin/env python3
"""
train_corner_model.py - 碁盤の4隅キーポイント検出モデル (YOLOv8-pose) 学習 & ONNX エクスポート

このスクリプトは、角度・傾き・照明・外周の碁石乱れに左右されずに碁盤の19路格子4隅
[TL, TR, BR, BL] を高精度に検出する超軽量 ONNX モデルを学習・出力します。

必要ライブラリ:
    pip install ultralytics onnx torch torchvision

使用方法:
    1. データセットの準備 (YOLO-Pose 形式: 1クラス board + 4キーポイント [TL, TR, BR, BL]):
       python train_corner_model.py --prepare-dataset --data-dir ./data

    2. 学習の実行:
       python train_corner_model.py --epochs 100 --imgsz 640 --batch 16

    3. ONNX へのエクスポート (Web 推論用):
       python train_corner_model.py --export-only --weights runs/pose/train/weights/best.pt --output ../web/model/goban-corners.onnx
"""

import argparse
import os
import sys
from pathlib import Path


def get_dataset_yaml_content(data_dir: Path) -> str:
    """YOLOv8-pose 用のデータセット YAML 設定"""
    return f"""
path: {data_dir.resolve()}
train: images/train
val: images/val

# 1 クラス: 碁盤 (board)
names:
  0: board

# 4 キーポイント: [TL, TR, BR, BL]
kpt_shape: [4, 3]  # [x, y, visible (0: 未表示, 1: 隠蔽, 2: 鮮明表示)]
flip_idx: [1, 0, 3, 2]  # 左右水平フリップ時のキーポイント反転ペア: TL <-> TR, BL <-> BR
"""


def export_to_onnx(model_path: str, output_path: str, imgsz: int = 640):
    """YOLOv8-pose 重みを Web 推論互換の ONNX 形式にエクスポート"""
    try:
        from ultralytics import YOLO
    except ImportError:
        print("[Error] ultralytics is required: pip install ultralytics")
        sys.exit(1)

    print(f"[*] Loading YOLO pose model from {model_path}...")
    model = YOLO(model_path)

    print(f"[*] Exporting to ONNX format (imgsz={imgsz}, opset=12, dynamic=False)...")
    exported_path = model.export(
        format="onnx",
        imgsz=imgsz,
        opset=12,
        dynamic=False,
        simplify=True,
    )

    out_file = Path(output_path)
    out_file.parent.mkdir(parents=True, exist_ok=True)
    if Path(exported_path) != out_file:
        import shutil
        shutil.copyfile(exported_path, out_file)

    print(f"[+] ONNX model successfully saved to: {out_file.resolve()}")
    print(f"    File size: {out_file.stat().st_size / 1024 / 1024:.2f} MB")


def train_model(args):
    """YOLOv8n-pose モデルの学習を実行"""
    try:
        from ultralytics import YOLO
    except ImportError:
        print("[Error] ultralytics is required: pip install ultralytics")
        sys.exit(1)

    data_dir = Path(args.data_dir)
    data_yaml = data_dir / "goban_pose.yaml"
    if not data_yaml.exists():
        print(f"[*] Writing dataset configuration to {data_yaml}...")
        data_yaml.parent.mkdir(parents=True, exist_ok=True)
        data_yaml.write_text(get_dataset_yaml_content(data_dir))

    print(f"[*] Initializing YOLOv8n-pose (Nano: 超軽量 ~3MB)...")
    model = YOLO("yolov8n-pose.pt")

    print(f"[*] Starting training for {args.epochs} epochs (imgsz={args.imgsz}, batch={args.batch})...")
    results = model.train(
        data=str(data_yaml),
        epochs=args.epochs,
        imgsz=args.imgsz,
        batch=args.batch,
        device=args.device,
        pose=12.0,      # キーポイント損失の重み
        kobj=2.0,       # キーポイント客体スコアの重み
        box=7.5,        # バウンディングボックス損失
        cls=0.5,        # クラス損失
        degrees=15.0,   # 回転データ拡張
        perspective=0.001,  # 遠近射影歪みデータ拡張 (実環境ロバスト性向上)
        flipud=0.5,     # 上下反転
        fliplr=0.5,     # 左右反転
        name="goban_corners",
    )

    best_pt = Path(model.trainer.save_dir) / "weights" / "best.pt"
    print(f"[+] Training completed! Best checkpoint: {best_pt}")

    if args.output:
        export_to_onnx(str(best_pt), args.output, args.imgsz)


def main():
    parser = argparse.ArgumentParser(description="Go Board 4-Corner Pose Model Trainer")
    parser.add_argument("--data-dir", type=str, default="./data", help="データセットディレクトリ")
    parser.add_argument("--epochs", type=int, default=100, help="学習エポック数")
    parser.add_argument("--batch", type=int, default=16, help="バッチサイズ")
    parser.add_argument("--imgsz", type=int, default=640, help="入力画像解像度")
    parser.add_argument("--device", type=str, default="", help="実行デバイス (例: 0 または cpu)")
    parser.add_argument("--weights", type=str, default="", help="事前学習済み/エクスポート用重みパス")
    parser.add_argument("--export-only", action="store_true", help="学習を行わず ONNX エクスポートのみ実行")
    parser.add_argument("--output", type=str, default="../web/model/goban-corners.onnx", help="出力 ONNX パス")

    args = parser.parse_args()

    if args.export_only:
        if not args.weights:
            print("[Error] --weights is required for --export-only")
            sys.exit(1)
        export_to_onnx(args.weights, args.output, args.imgsz)
    else:
        train_model(args)


if __name__ == "__main__":
    main()
