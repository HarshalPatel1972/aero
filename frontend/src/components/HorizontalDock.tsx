/**
 * HorizontalDock Component
 * The "Mini Mode" UI strip with detached handle and horizontal layout.
 */

import { Power, Maximize2, GripVertical } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import type { ServerStatus } from '../types';

interface HorizontalDockProps {
  serverStatus: ServerStatus;
  onExpand: () => void;
  onTogglePower: () => void;
}

export function HorizontalDock({ serverStatus, onExpand, onTogglePower }: HorizontalDockProps) {
  const isActive = serverStatus.running;

  return (
    <div className="flex flex-row items-center bg-zinc-950 border border-white/10 rounded-xl h-full px-4 gap-6 w-full overflow-hidden">
      {/* Section A: The Stick (Left Control) */}
      <div className="flex flex-col gap-3 items-center text-zinc-500 py-1 border-r border-white/5 pr-4 h-full justify-center">
        {/* Drag Handle - Wails Drag Region */}
        <div className="wails-drag cursor-move hover:text-zinc-300 transition-colors p-1">
          <GripVertical className="w-5 h-5" />
        </div>
        
        {/* Expand Icon */}
        <button 
          onClick={onExpand}
          className="wails-no-drag hover:text-cyan-400 transition-colors p-1"
          title="Restore"
        >
          <Maximize2 className="w-4 h-4" />
        </button>
      </div>

      {/* Section B: Identity (QR Code) */}
      <div className={`transition-all duration-300 ${isActive ? 'opacity-100' : 'opacity-40 grayscale'}`}>
        <div className="bg-white p-1 rounded-lg">
           <QRCodeSVG 
            value={serverStatus.url || 'https://aero.app'} 
            size={72} // ~80px with padding
            level="M"
            bgColor="#FFFFFF"
            fgColor="#000000"
          />
        </div>
      </div>

      {/* Section C: Action (Power Button) */}
      <div className="flex items-center justify-center">
        <button
          onClick={onTogglePower}
          className={`wails-no-drag group relative p-3 rounded-xl transition-all duration-300 border border-white/10
            ${isActive ? 'bg-aero-cyan/10 border-aero-cyan/50' : 'bg-white/5 hover:bg-white/10'}
          `}
        >
           {isActive && <div className="absolute inset-0 bg-aero-cyan/20 blur-lg rounded-xl animate-pulse" />}
           <Power className={`w-6 h-6 z-10 relative ${isActive ? 'text-aero-cyan' : 'text-zinc-500'}`} />
        </button>
      </div>

      {/* Section D: Context (Recent Files) */}
      <div className="flex-1 flex flex-col justify-center h-full ml-2">
        <span className="text-xs uppercase tracking-widest text-zinc-600 font-medium mb-1">Recent Files</span>
        <div className="text-sm text-zinc-400 truncate w-32">
          {/* Placeholder for now, or real data */}
          No transfers yet
        </div>
      </div>
    </div>
  );
}
