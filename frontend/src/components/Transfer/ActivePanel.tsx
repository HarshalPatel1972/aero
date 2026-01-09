/**
 * ActivePanel Component
 * Term-Phase 2: Sci-Fi Telemetry HUD
 * 
 * Heads-Up Display for active file transfers.
 * Features: Real-time graph, quantum progress bar, telemetry grid.
 */

import { memo } from 'react';
import { ArrowUpRight, ArrowDownLeft, Zap } from 'lucide-react';
import { TelemetryGraph } from './TelemetryGraph';
import { TransferState, formatBytes, formatSpeed, formatEta } from '../../hooks/useTransfer';

interface ActivePanelProps {
  transfer: TransferState;
  onTest?: () => void;
}

/**
 * Quantum Progress Bar
 * Segmented progress with leading edge glow
 */
const QuantumProgressBar = memo(function QuantumProgressBar({ 
  progress 
}: { 
  progress: number 
}) {
  const segments = 20;
  const filledSegments = Math.floor((progress / 100) * segments);
  
  return (
    <div className="relative h-2 w-full flex gap-0.5">
      {/* Segments */}
      {Array.from({ length: segments }).map((_, i) => {
        const isFilled = i < filledSegments;
        const isLeading = i === filledSegments - 1 && progress < 100;
        
        return (
          <div
            key={i}
            className={`
              flex-1 rounded-sm transition-all duration-150
              ${isFilled 
                ? isLeading 
                  ? 'bg-aero-cyan shadow-glow-cyan' 
                  : 'bg-aero-cyan/70' 
                : 'bg-void-border/50'}
            `}
            style={{
              boxShadow: isLeading ? '0 0 12px #00E5FF' : undefined,
            }}
          />
        );
      })}
      
      {/* Percentage overlay */}
      <div className="absolute inset-0 flex items-center justify-center">
        <span className="text-[10px] font-bold text-white/80 
          tabular-nums tracking-wider
          drop-shadow-lg">
          {progress.toFixed(1)}%
        </span>
      </div>
    </div>
  );
});

/**
 * Telemetry Stat Item
 */
const TelemetryStat = memo(function TelemetryStat({
  label,
  value,
  highlight = false,
}: {
  label: string;
  value: string;
  highlight?: boolean;
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-[10px] uppercase tracking-wider text-white/40 font-medium">
        {label}
      </span>
      <span className={`
        text-sm font-bold tabular-nums tracking-tight
        font-mono
        ${highlight ? 'text-aero-cyan' : 'text-white'}
      `}>
        {value}
      </span>
    </div>
  );
});

/**
 * ActivePanel - Main HUD Container
 */
export const ActivePanel = memo(function ActivePanel({
  transfer,
  onTest,
}: ActivePanelProps) {
  const { active, filename, totalBytes, transferredBytes, speed, speedHistory, eta, direction } = transfer;

  // If no active transfer, show test button
  if (!active && transfer.progress < 100) {
    return (
      <div className="flex flex-col items-center justify-center gap-4 py-8
        bg-void-surface/30 backdrop-blur-sm rounded-xl border border-void-border/50">
        <p className="text-sm text-white/40">No active transfer</p>
        {onTest && (
          <button
            onClick={onTest}
            className="
              flex items-center gap-2 px-4 py-2
              bg-aero-cyan/10 text-aero-cyan
              rounded-lg border border-aero-cyan/30
              hover:bg-aero-cyan/20 hover:border-aero-cyan/50
              transition-all duration-200
              active:scale-95 transform-gpu
              text-sm font-medium
            "
          >
            <Zap className="w-4 h-4" />
            Test Transfer
          </button>
        )}
      </div>
    );
  }

  // Completed state
  if (!active && transfer.progress >= 100) {
    return (
      <div className="flex flex-col items-center justify-center gap-3 py-6
        bg-aero-cyan/5 backdrop-blur-sm rounded-xl border border-aero-cyan/20">
        <div className="w-10 h-10 rounded-full bg-aero-cyan/20 flex items-center justify-center">
          {direction === 'send' 
            ? <ArrowUpRight className="w-5 h-5 text-aero-cyan" />
            : <ArrowDownLeft className="w-5 h-5 text-aero-cyan" />
          }
        </div>
        <p className="text-sm font-medium text-aero-cyan">Transfer Complete</p>
        <p className="text-xs text-white/40 truncate max-w-[200px]">{filename}</p>
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-4 p-4
      bg-void-surface/40 backdrop-blur-glass
      rounded-xl border border-void-border/50
      animate-scale-in">
      
      {/* Header: Filename + Direction */}
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 min-w-0">
          <div className={`
            w-8 h-8 rounded-lg flex items-center justify-center flex-shrink-0
            ${direction === 'send' ? 'bg-blue-500/20' : 'bg-aero-cyan/20'}
          `}>
            {direction === 'send' 
              ? <ArrowUpRight className="w-4 h-4 text-blue-400" />
              : <ArrowDownLeft className="w-4 h-4 text-aero-cyan" />
            }
          </div>
          <div className="min-w-0">
            <p className="text-sm font-medium text-white truncate">{filename}</p>
            <p className="text-xs text-white/40">
              {direction === 'send' ? 'Sending' : 'Receiving'}
            </p>
          </div>
        </div>
        
        {/* Live indicator */}
        <div className="flex items-center gap-1.5">
          <div className="w-2 h-2 rounded-full bg-aero-cyan animate-pulse" />
          <span className="text-xs font-medium text-aero-cyan uppercase tracking-wider">Live</span>
        </div>
      </div>

      {/* Quantum Progress Bar */}
      <QuantumProgressBar progress={transfer.progress} />

      {/* Telemetry Graph */}
      <div className="relative">
        <TelemetryGraph 
          data={speedHistory} 
          width={280} 
          height={48}
          className="w-full"
        />
      </div>

      {/* Telemetry Grid */}
      <div className="grid grid-cols-3 gap-4">
        <TelemetryStat 
          label="Speed" 
          value={formatSpeed(speed)} 
          highlight 
        />
        <TelemetryStat 
          label="Size" 
          value={`${formatBytes(transferredBytes)} / ${formatBytes(totalBytes)}`} 
        />
        <TelemetryStat 
          label="ETA" 
          value={formatEta(eta)} 
        />
      </div>
    </div>
  );
});
