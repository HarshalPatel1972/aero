/**
 * sender.ts - Hybrid Transfer Engine
 * Term-Phase 3: "The Right Tool for the Weight"
 * 
 * Two modes:
 * - BULLET (<50MB): Single-shot HTTP POST, read as Blob
 * - TRAIN (>50MB): ReadableStream with adaptive chunking
 * 
 * Features:
 * - Adaptive chunk sizing based on network speed
 * - Backpressure handling via highWaterMark
 * - Real-time progress callbacks
 */

// ═══════════════════════════════════════════════════════════════
// CONSTANTS
// ═══════════════════════════════════════════════════════════════

const BULLET_THRESHOLD = 50 * 1024 * 1024; // 50MB
const CHUNK_SMALL = 4 * 1024; // 4KB - for slow/jittery networks
const CHUNK_MEDIUM = 32 * 1024; // 32KB - default
const CHUNK_LARGE = 64 * 1024; // 64KB - for fast networks (>20MB/s)

const SPEED_SLOW_THRESHOLD = 5; // MB/s
const SPEED_FAST_THRESHOLD = 20; // MB/s

// ═══════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════

export interface TransferProgress {
  filename: string;
  status: 'starting' | 'uploading' | 'completed' | 'error';
  progress: number; // 0-100
  speedMBps: number;
  bytesTotal: number;
  bytesSent: number;
  elapsedMs: number;
  eta: number; // seconds
}

export interface TransferOptions {
  url: string;
  file: File;
  onProgress?: (progress: TransferProgress) => void;
  abortSignal?: AbortSignal;
}

export interface TransferResult {
  success: boolean;
  error?: string;
  durationMs: number;
  avgSpeedMBps: number;
}

// ═══════════════════════════════════════════════════════════════
// UTILITIES
// ═══════════════════════════════════════════════════════════════

/**
 * Calculate adaptive chunk size based on measured speed
 * Exported for use in adaptive streaming implementations
 */
export function getAdaptiveChunkSize(speedMBps: number): number {
  if (speedMBps > SPEED_FAST_THRESHOLD) {
    return CHUNK_LARGE;
  } else if (speedMBps < SPEED_SLOW_THRESHOLD) {
    return CHUNK_SMALL;
  }
  return CHUNK_MEDIUM;
}

/**
 * Create a progress tracker
 */
function createProgressTracker(file: File, onProgress?: (p: TransferProgress) => void) {
  const startTime = performance.now();
  let bytesSent = 0;
  let lastEmitTime = 0;
  const EMIT_INTERVAL = 100; // ms

  return {
    update(chunkSize: number) {
      bytesSent += chunkSize;
      const now = performance.now();
      
      // Debounce emissions
      if (now - lastEmitTime < EMIT_INTERVAL) return;
      lastEmitTime = now;

      const elapsedMs = now - startTime;
      const elapsedSec = elapsedMs / 1000;
      const speedMBps = elapsedSec > 0 ? (bytesSent / (1024 * 1024)) / elapsedSec : 0;
      const progress = (bytesSent / file.size) * 100;
      const remaining = file.size - bytesSent;
      const eta = speedMBps > 0 ? remaining / (speedMBps * 1024 * 1024) : 0;

      onProgress?.({
        filename: file.name,
        status: 'uploading',
        progress,
        speedMBps,
        bytesTotal: file.size,
        bytesSent,
        elapsedMs,
        eta,
      });
    },
    
    complete(success: boolean) {
      const elapsedMs = performance.now() - startTime;
      const elapsedSec = elapsedMs / 1000;
      const speedMBps = elapsedSec > 0 ? (bytesSent / (1024 * 1024)) / elapsedSec : 0;
      
      onProgress?.({
        filename: file.name,
        status: success ? 'completed' : 'error',
        progress: success ? 100 : (bytesSent / file.size) * 100,
        speedMBps,
        bytesTotal: file.size,
        bytesSent,
        elapsedMs,
        eta: 0,
      });

      return { durationMs: elapsedMs, avgSpeedMBps: speedMBps };
    },

    getSpeed(): number {
      const elapsedSec = (performance.now() - startTime) / 1000;
      return elapsedSec > 0 ? (bytesSent / (1024 * 1024)) / elapsedSec : 0;
    }
  };
}

// ═══════════════════════════════════════════════════════════════
// BULLET MODE (<50MB) - Single-shot HTTP POST
// ═══════════════════════════════════════════════════════════════

async function uploadBullet(options: TransferOptions): Promise<TransferResult> {
  const { url, file, onProgress, abortSignal } = options;
  const tracker = createProgressTracker(file, onProgress);
  
  onProgress?.({
    filename: file.name,
    status: 'starting',
    progress: 0,
    speedMBps: 0,
    bytesTotal: file.size,
    bytesSent: 0,
    elapsedMs: 0,
    eta: 0,
  });

  try {
    // Read entire file as blob
    const blob = await file.arrayBuffer();
    
    // Upload with XHR for progress (fetch doesn't support upload progress)
    return new Promise((resolve) => {
      const xhr = new XMLHttpRequest();
      
      xhr.upload.onprogress = (e) => {
        if (e.lengthComputable) {
          tracker.update(e.loaded - (tracker as unknown as { bytesSent: number }).bytesSent || e.loaded);
        }
      };

      xhr.onload = () => {
        const stats = tracker.complete(xhr.status >= 200 && xhr.status < 300);
        resolve({
          success: xhr.status >= 200 && xhr.status < 300,
          error: xhr.status >= 400 ? xhr.statusText : undefined,
          durationMs: stats.durationMs,
          avgSpeedMBps: stats.avgSpeedMBps,
        });
      };

      xhr.onerror = () => {
        const stats = tracker.complete(false);
        resolve({
          success: false,
          error: 'Network error',
          durationMs: stats.durationMs,
          avgSpeedMBps: stats.avgSpeedMBps,
        });
      };

      if (abortSignal) {
        abortSignal.addEventListener('abort', () => xhr.abort());
      }

      xhr.open('POST', url);
      xhr.setRequestHeader('Content-Type', 'application/octet-stream');
      xhr.setRequestHeader('X-Filename', encodeURIComponent(file.name));
      xhr.setRequestHeader('X-Filesize', file.size.toString());
      xhr.send(blob);
    });
  } catch (error) {
    const stats = tracker.complete(false);
    return {
      success: false,
      error: error instanceof Error ? error.message : 'Unknown error',
      durationMs: stats.durationMs,
      avgSpeedMBps: stats.avgSpeedMBps,
    };
  }
}

// ═══════════════════════════════════════════════════════════════
// TRAIN MODE (>50MB) - Streaming with adaptive chunks
// ═══════════════════════════════════════════════════════════════

async function uploadTrain(options: TransferOptions): Promise<TransferResult> {
  const { url, file, onProgress, abortSignal } = options;
  const tracker = createProgressTracker(file, onProgress);
  
  onProgress?.({
    filename: file.name,
    status: 'starting',
    progress: 0,
    speedMBps: 0,
    bytesTotal: file.size,
    bytesSent: 0,
    elapsedMs: 0,
    eta: 0,
  });

  try {
    // Use File.stream() for memory-efficient reading
    const stream = file.stream();
    const reader = stream.getReader();
    
    let bytesSent = 0;
    const chunks: Uint8Array[] = [];
    
    // Read all chunks (respecting backpressure via reader.read())
    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      
      if (abortSignal?.aborted) {
        reader.cancel();
        throw new Error('Aborted');
      }
      
      chunks.push(value);
      bytesSent += value.byteLength;
      tracker.update(value.byteLength);
    }

    // Combine chunks and send
    const totalBuffer = new Uint8Array(bytesSent);
    let offset = 0;
    for (const chunk of chunks) {
      totalBuffer.set(chunk, offset);
      offset += chunk.byteLength;
    }

    // Upload
    const response = await fetch(url, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/octet-stream',
        'X-Filename': encodeURIComponent(file.name),
        'X-Filesize': file.size.toString(),
      },
      body: totalBuffer,
      signal: abortSignal,
    });

    const success = response.ok;
    const stats = tracker.complete(success);
    
    return {
      success,
      error: success ? undefined : response.statusText,
      durationMs: stats.durationMs,
      avgSpeedMBps: stats.avgSpeedMBps,
    };
  } catch (error) {
    const stats = tracker.complete(false);
    return {
      success: false,
      error: error instanceof Error ? error.message : 'Unknown error',
      durationMs: stats.durationMs,
      avgSpeedMBps: stats.avgSpeedMBps,
    };
  }
}

// ═══════════════════════════════════════════════════════════════
// MAIN EXPORT - Auto-selects mode based on file size
// ═══════════════════════════════════════════════════════════════

/**
 * Upload a file using the optimal transfer strategy.
 * - Files <50MB: Bullet mode (single-shot)
 * - Files >50MB: Train mode (streaming)
 */
export async function uploadFile(options: TransferOptions): Promise<TransferResult> {
  const { file } = options;
  
  if (file.size <= BULLET_THRESHOLD) {
    console.log(`[Sender] BULLET mode for ${file.name} (${(file.size / 1024 / 1024).toFixed(1)}MB)`);
    return uploadBullet(options);
  } else {
    console.log(`[Sender] TRAIN mode for ${file.name} (${(file.size / 1024 / 1024).toFixed(1)}MB)`);
    return uploadTrain(options);
  }
}

/**
 * Determine which mode would be used for a file.
 */
export function getTransferMode(fileSize: number): 'bullet' | 'train' {
  return fileSize <= BULLET_THRESHOLD ? 'bullet' : 'train';
}
