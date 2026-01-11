/**
 * Compact View Component
 * Horizontal strip with external-looking controls on left
 */

import { Power, Maximize2, GripVertical } from 'lucide-react';
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
      {/* Left Control Strip - looks external */}
      <div className="w-12 bg-gray-900/95 border-r border-white/20 flex flex-col items-center justify-between py-4">
        {/* Drag Handle */}
        <div className="wails-drag flex-1 flex items-center justify-center cursor-move">
          <GripVertical className="w-5 h-5 text-white/40" />
        </div>
        
        {/* Maximize Button */}
        <button
          onClick={onExpand}
          className="wails-no-drag p-2 rounded-lg hover:bg-white/10 transition-all"
          title="Expand to full view"
        >
          <Maximize2 className="w-5 h-5 text-white/60 hover:text-cyan-400" />
        </button>
      </div>

      {/* Main Horizontal Content */}
      <div className="flex-1 flex items-center justify-center gap-8 px-8 bg-void-surface/95">
        {/* QR Code */}
        <div className={`p-3 rounded-xl bg-white transition-all duration-300 ${
          isActive ? 'shadow-glow-cyan' : 'opacity-50'
        }`}>
          <QRCodeSVG 
            value={serverStatus.url || 'https://aero.app'} 
            size={80}
            level="M"
            bgColor="transparent"
            fgColor={isActive ? '#050505' : '#666666'}
          />
        </div>

        {/* Power Button */}
        <button
          onClick={onTogglePower}
          className="wails-no-drag group relative p-4 rounded-xl transition-all duration-300
            hover:scale-105 active:scale-95 transform-gpu"
          aria-label={isActive ? 'Stop Server' : 'Start Server'}
        >
          <div className={`absolute inset-0 rounded-xl blur-xl transition-all duration-300 ${
            isActive 
              ? 'bg-aero-cyan/40 group-hover:bg-aero-cyan/60' 
              : 'bg-white/5 group-hover:bg-white/10'
          }`} />
          <Power className={`relative w-8 h-8 transition-all duration-300 ${
            isActive 
              ? 'text-aero-cyan drop-shadow-glow-cyan' 
              : 'text-white/40 group-hover:text-white/60'
          }`} />
        </button>
      </div>
    </div>
  );
}
