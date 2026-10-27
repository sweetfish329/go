import type { Point2D } from './board-detector.js';

export interface MLCornerDetectionResult {
  corners: Point2D[];
  confidence: number;
  method: string;
}

export class MLCornerDetector {
  constructor(options?: {
    modelPath?: string;
    inputSize?: number;
    confidenceThreshold?: number;
  });

  load(modelPath?: string | null): Promise<boolean>;
  detect(imageSource: any, fallbackWasmKifu?: any): Promise<MLCornerDetectionResult>;
}
