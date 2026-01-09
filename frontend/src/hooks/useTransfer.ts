/**
 * useTransfer Hook
 * Term-Phase 2: Transfer State Engine
 * 
 * Manages real-time transfer state with speed history for sparkline,
 * ETA calculation, and simulation capabilities.
 */

import { useState, useCallback, useRef, useEffect } from 'react';

// ═══════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════

export interface TransferState {
  active: boolean;
  filename: string;
  totalBytes: number;
  transferredBytes: number;
  speed: number; // bytes per second
  speedHistory: number[]; // Last 30 data points for sparkline
  progress: number; // 0-100
  eta: number; // seconds remaining
  startTime: number;
  direction: 'send' | 'receive';
}

const HISTORY_LENGTH = 30;
const UPDATE_INTERVAL = 100; // ms

// ═══════════════════════════════════════════════════════════════
// INITIAL STATE
// ═══════════════════════════════════════════════════════════════

const initialState: TransferState = {
  active: false,
  filename: '',
  totalBytes: 0,
  transferredBytes: 0,
  speed: 0,
  speedHistory: new Array(HISTORY_LENGTH).fill(0),
  progress: 0,
  eta: 0,
  startTime: 0,
  direction: 'receive',
};

// ═══════════════════════════════════════════════════════════════
// UTILITIES
// ═══════════════════════════════════════════════════════════════

/**
 * Add Gaussian noise for realistic speed fluctuation
 */
function addNoise(value: number, variance: number): number {
  const u1 = Math.random();
  const u2 = Math.random();
  const normal = Math.sqrt(-2 * Math.log(u1)) * Math.cos(2 * Math.PI * u2);
  return Math.max(0, value + normal * variance);
}

/**
 * Format bytes to human readable
 */
export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${(bytes / Math.pow(k, i)).toFixed(1)} ${sizes[i]}`;
}

/**
 * Format speed to human readable
 */
export function formatSpeed(bytesPerSecond: number): string {
  return `${formatBytes(bytesPerSecond)}/s`;
}

/**
 * Format ETA in seconds to readable
 */
export function formatEta(seconds: number): string {
  if (seconds <= 0 || !isFinite(seconds)) return '--';
  if (seconds < 60) return `${Math.ceil(seconds)}s`;
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${Math.ceil(seconds % 60)}s`;
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`;
}

// ═══════════════════════════════════════════════════════════════
// HOOK
// ═══════════════════════════════════════════════════════════════

export function useTransfer() {
  const [state, setState] = useState<TransferState>(initialState);
  const intervalRef = useRef<number | null>(null);
  const simulationRef = useRef<{
    targetSpeed: number;
    currentSpeed: number;
  } | null>(null);

  /**
   * Start a real transfer (from backend events)
   */
  const startTransfer = useCallback((
    filename: string,
    totalBytes: number,
    direction: 'send' | 'receive'
  ) => {
    setState({
      active: true,
      filename,
      totalBytes,
      transferredBytes: 0,
      speed: 0,
      speedHistory: new Array(HISTORY_LENGTH).fill(0),
      progress: 0,
      eta: 0,
      startTime: Date.now(),
      direction,
    });
  }, []);

  /**
   * Update transfer progress
   */
  const updateProgress = useCallback((transferredBytes: number, speed: number) => {
    setState(prev => {
      if (!prev.active) return prev;
      
      const progress = Math.min(100, (transferredBytes / prev.totalBytes) * 100);
      const remaining = prev.totalBytes - transferredBytes;
      const eta = speed > 0 ? remaining / speed : 0;
      
      // Update speed history (sliding window)
      const speedHistory = [...prev.speedHistory.slice(1), speed];
      
      return {
        ...prev,
        transferredBytes,
        speed,
        speedHistory,
        progress,
        eta,
      };
    });
  }, []);

  /**
   * Complete transfer
   */
  const completeTransfer = useCallback(() => {
    setState(prev => ({
      ...prev,
      active: false,
      progress: 100,
      speed: 0,
      eta: 0,
    }));
    
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
    simulationRef.current = null;
  }, []);

  /**
   * Reset state
   */
  const reset = useCallback(() => {
    if (intervalRef.current) {
      clearInterval(intervalRef.current);
      intervalRef.current = null;
    }
    simulationRef.current = null;
    setState(initialState);
  }, []);

  /**
   * SIMULATOR: Run a fake transfer for testing
   */
  const simulateTransfer = useCallback((
    filename: string = 'test_file.zip',
    totalBytes: number = 500 * 1024 * 1024, // 500 MB
    targetSpeed: number = 50 * 1024 * 1024 // 50 MB/s
  ) => {
    // Initialize simulation
    simulationRef.current = {
      targetSpeed,
      currentSpeed: 0,
    };

    setState({
      active: true,
      filename,
      totalBytes,
      transferredBytes: 0,
      speed: 0,
      speedHistory: new Array(HISTORY_LENGTH).fill(0),
      progress: 0,
      eta: totalBytes / targetSpeed,
      startTime: Date.now(),
      direction: 'receive',
    });

    // Run simulation loop
    intervalRef.current = window.setInterval(() => {
      setState(prev => {
        if (!prev.active || !simulationRef.current) {
          if (intervalRef.current) clearInterval(intervalRef.current);
          return prev;
        }

        // Ramp up speed (ease-out)
        const sim = simulationRef.current;
        const rampFactor = 0.15;
        sim.currentSpeed += (sim.targetSpeed - sim.currentSpeed) * rampFactor;
        
        // Add noise for realism
        const speed = addNoise(sim.currentSpeed, sim.targetSpeed * 0.08);
        
        // Calculate new bytes transferred
        const bytesThisInterval = (speed * UPDATE_INTERVAL) / 1000;
        const newTransferred = Math.min(prev.totalBytes, prev.transferredBytes + bytesThisInterval);
        const progress = (newTransferred / prev.totalBytes) * 100;
        const remaining = prev.totalBytes - newTransferred;
        const eta = speed > 0 ? remaining / speed : 0;
        
        // Update speed history
        const speedHistory = [...prev.speedHistory.slice(1), speed];

        // Check completion
        if (newTransferred >= prev.totalBytes) {
          if (intervalRef.current) clearInterval(intervalRef.current);
          simulationRef.current = null;
          
          return {
            ...prev,
            active: false,
            transferredBytes: prev.totalBytes,
            speed: 0,
            speedHistory: [...prev.speedHistory.slice(1), 0],
            progress: 100,
            eta: 0,
          };
        }

        return {
          ...prev,
          transferredBytes: newTransferred,
          speed,
          speedHistory,
          progress,
          eta,
        };
      });
    }, UPDATE_INTERVAL);
  }, []);

  // Cleanup on unmount
  useEffect(() => {
    return () => {
      if (intervalRef.current) clearInterval(intervalRef.current);
    };
  }, []);

  return {
    state,
    startTransfer,
    updateProgress,
    completeTransfer,
    reset,
    simulateTransfer,
    formatBytes,
    formatSpeed,
    formatEta,
  };
}
