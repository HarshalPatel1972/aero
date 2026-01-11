/**
 * Compact View Component
 * Shown when always-on-top is enabled and window loses focus
 */

import { Power } from 'lucide-react';
import type { ServerStatus } from '../types';

interface CompactViewProps {
  serverStatus: ServerStatus;
  onExpand: () => void;
}

export function CompactView({ serverStatus, onExpand }: CompactViewProps) {
  return (
    <div 
      onClick={onExpand}
      className="h-full flex items-center justify-center gap-3 px-4 cursor-pointer
        bg-void-surface/95 backdrop-blur-xl border border-white/10 rounded-xl
        transition-all duration-300 ease-out hover:bg-void-surface
        hover:border-cyan-400/30"
    >
      <Power 
        className={`w-5 h-5 transition-colors ${
          serverStatus.running 
            ? 'text-cyan-400' 
            : 'text-white/40'
        }`} 
      />
      <span className="text-sm font-medium text-white/80">
        Aero
      </span>
    </div>
  );
}
