/**
 * Compact View Component
 * Horizontal strip with external-looking controls on left
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
    <div className="h-full w-full flex items-stretch">
      {/* Left Control Strip */}
      <div className="w-10 bg-black/90 border-r border-white/10 flex flex-col items-center justify-between py-3">
        {/* Drag Handle - 6 dots grid */}
        <div className="wails-drag flex-1 flex items-center justify-center cursor-move">
          <div className="grid grid-cols-2 gap-1">
            {[...Array(6)].map((_, i) => (
              <div key={i} className="w-1 h-1 rounded-full bg-white/30" />
            ))}
          </div>
        </div>
        
        {/* Maximize Button */}
        <button
          onClick={onExpand}
          className="wails-no-drag p-1.5 rounded hover:bg-white/5 transition-all"
          title="Expand"
        >
          <Maximize2 className="w-4 h-4 text-white/50" />
        </button>
      </div>

      {/* Main Horizontal Content */}
      <div className="flex-1 flex items-center gap-6 px-6 bg-void-surface/95">
        {/* QR Code */}
        <div className={`p-2 rounded-lg bg-gray-700/50 transition-all duration-300 ${
          isActive ? 'shadow-glow-cyan' : 'opacity-50'
        }`}>
          <QRCodeSVG 
            value={serverStatus.url || 'https://aero.app'} 
            size={70}
            level="M"
            bgColor="transparent"
            fgColor="#E5E7EB"
          />
        </div>

        {/* Power Button */}
        <button
          onClick={onTogglePower}
          className="wails-no-drag group relative p-3 rounded-full transition-all duration-300
            hover:scale-105 active:scale-95 transform-gpu bg-gray-800/50 border border-white/10"
          aria-label={isActive ? 'Stop Server' : 'Start Server'}
        >
          <Power className={`w-6 h-6 transition-all duration-300 ${
            isActive 
              ? 'text-aero-cyan' 
              : 'text-white/40'
          }`} />
        </button>

        {/* Recent Files Label */}
        <div className="flex-1" />
        <span className="text-sm font-medium text-white/40 tracking-wider">
          RECENT FILES
        </span>
      </div>
    </div>
  );
}
