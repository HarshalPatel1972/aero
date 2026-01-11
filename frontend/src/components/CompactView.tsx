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
