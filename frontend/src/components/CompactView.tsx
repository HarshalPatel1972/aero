/**
 * Compact View Component
 * Horizontal strip shown when always-on-top is enabled and window loses focus
 */

import { Power, Maximize2 } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import type { ServerStatus } from '../types';

interface CompactViewProps {
  serverStatus: ServerStatus;
  onExpand: () => void;
  onTogglePower: () => void;
}

export function CompactView({ serverStatus, onExpand, onTogglePower }: CompactViewProps) {
  const isActive = serverStatus.running;

  return (
    <div className="wails-drag h-full w-full flex items-center gap-4 px-4
      bg-gradient-to-r from-purple-900 to-blue-900 border-t border-cyan-400">
      
      {/* Mini QR Code */}
      <div className={`p-2 rounded-lg bg-white transition-all duration-300 ${
        isActive ? 'shadow-glow-cyan' : 'opacity-50'
      }`}>
        <QRCodeSVG 
          value={serverStatus.url || 'https://aero.app'} 
          size={48}
          level="M"
          bgColor="transparent"
          fgColor={isActive ? '#050505' : '#666666'}
        />
      </div>

      {/* Power Button */}
      <button
        onClick={onTogglePower}
        className="wails-no-drag group relative p-3 rounded-xl transition-all duration-300
          hover:scale-105 active:scale-95 transform-gpu"
        aria-label={isActive ? 'Stop Server' : 'Start Server'}
      >
        <div className={`absolute inset-0 rounded-xl blur-xl transition-all duration-300 ${
          isActive 
            ? 'bg-aero-cyan/40 group-hover:bg-aero-cyan/60' 
            : 'bg-white/5 group-hover:bg-white/10'
        }`} />
        <Power className={`relative w-6 h-6 transition-all duration-300 ${
          isActive 
            ? 'text-aero-cyan drop-shadow-glow-cyan' 
            : 'text-white/40 group-hover:text-white/60'
        }`} />
      </button>

      {/* Spacer */}
      <div className="flex-1" />

      {/* Expand Arrow */}
      <button
        onClick={onExpand}
        className="wails-no-drag p-2 rounded-lg hover:bg-white/10 transition-all duration-200
          active:scale-95 transform-gpu"
        aria-label="Expand to full view"
        title="Expand"
      >
        <Maximize2 className="w-4 h-4 text-white/60 hover:text-white/90" />
      </button>
    </div>
  );
}
