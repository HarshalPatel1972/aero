/**
 * MiniDock Component
 * A strict horizontal row docked bar.
 */

import { Power, Maximize2, GripVertical } from 'lucide-react';
import { QRCodeSVG } from 'qrcode.react';
import type { ServerStatus } from '../types';

interface MiniDockProps {
  serverStatus: ServerStatus;
  onRestore: () => void;
  onTogglePower: () => void;
}

export function MiniDock({ serverStatus, onRestore, onTogglePower }: MiniDockProps) {
  const isActive = serverStatus.running;

  return (
    <div className="w-full h-full flex flex-row items-center bg-zinc-950 border border-zinc-800 rounded-lg overflow-hidden select-none">
      
      {/* Zone 1 (The Handle - Left) */}
      <div className="w-12 h-full bg-zinc-900/80 flex flex-col items-center justify-center gap-2 border-r border-zinc-800">
        <div className="wails-drag cursor-grab active:cursor-grabbing p-1 text-zinc-600 hover:text-zinc-400 transition-colors">
          <GripVertical className="w-5 h-5" />
        </div>
        <button 
          onClick={onRestore}
          className="wails-no-drag p-1 text-zinc-600 hover:text-cyan-400 transition-colors"
          title="Restore Value"
        >
          <Maximize2 className="w-4 h-4" />
        </button>
      </div>

      {/* Zone 2 (QR - Identity) */}
      <div className="ml-4 h-20 w-20 bg-white p-1 rounded-lg flex items-center justify-center shadow-lg">
        <QRCodeSVG 
          value={serverStatus.url || 'https://aero.app'} 
          size={72}
          level="M" 
          bgColor="#FFFFFF"
          fgColor="#000000"
        />
      </div>

      {/* Zone 3 (Power - Action) */}
      <div className="ml-6 flex items-center justify-center">
        <button
          onClick={onTogglePower}
          className={`
            w-16 h-16 rounded-full flex items-center justify-center
            border transition-all duration-300 wails-no-drag
            ${isActive 
              ? 'bg-zinc-900 border-green-500/30 shadow-[0_0_15px_rgba(34,197,94,0.2)]' 
              : 'bg-zinc-900 border-zinc-700 hover:border-zinc-600'}
          `}
        >
          <Power className={`w-8 h-8 transition-colors ${isActive ? 'text-green-500' : 'text-zinc-500'}`} />
        </button>
      </div>

      {/* Zone 4 (Recent - Context) */}
      <div className="flex-1 ml-6 h-full flex flex-col justify-center pr-4">
        <span className="text-zinc-500 text-xs uppercase tracking-wider font-semibold mb-0.5">Last Transfer</span>
        <div className="text-zinc-400 text-sm font-mono truncate">
          video.mp4
        </div>
      </div>

    </div>
  );
}
