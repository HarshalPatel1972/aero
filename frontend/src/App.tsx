/**
 * AERO Desktop Application
 * Term-Phase 1: Identity & Universal UX
 * 
 * "Braun Electronics meets Cyberpunk"
 * Designed for both grandma and power users.
 */

import { useEffect, useState, useCallback, useRef } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import {
  Power,
  Minus,
  X,
  FolderOpen,
  Wifi,
  WifiOff,
  ChevronDown,
  FileCheck,
  Loader2,
  Volume2,
  VolumeX,
  Send,
  ArrowUpRight,
  ArrowDownLeft,
} from 'lucide-react';
import type { NetworkInterface, ServerStatus, TransferEvent } from './types';
import { useTransfer } from './hooks/useTransfer';
import { ActivePanel } from './components/Transfer/ActivePanel';

// ═══════════════════════════════════════════════════════════════
// AERO LOGO COMPONENT
// ═══════════════════════════════════════════════════════════════

function AeroLogo({ className = '' }: { className?: string }) {
  return (
    <svg 
      viewBox="0 0 512 512" 
      fill="none" 
      className={className}
    >
      <rect x="32" y="32" width="448" height="448" rx="96" ry="96" fill="#050505"/>
      <defs>
        <linearGradient id="aeroGrad" x1="0%" y1="0%" x2="100%" y2="100%">
          <stop offset="0%" stopColor="#00E5FF"/>
          <stop offset="100%" stopColor="#00B4D8"/>
        </linearGradient>
      </defs>
      <path 
        d="M256 96 L384 384 L336 384 L304 304 L208 304 L176 384 L128 384 L256 96 Z M256 176 L224 272 L288 272 L256 176 Z" 
        fill="url(#aeroGrad)"
      />
      <path d="M128 192 L96 192" stroke="#00E5FF" strokeWidth="8" strokeLinecap="round" opacity="0.6"/>
      <path d="M144 240 L80 240" stroke="#00E5FF" strokeWidth="6" strokeLinecap="round" opacity="0.4"/>
      <path d="M136 288 L104 288" stroke="#00E5FF" strokeWidth="4" strokeLinecap="round" opacity="0.3"/>
    </svg>
  );
}

// ═══════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════

interface Transfer {
  id: string;
  filename: string;
  status: 'started' | 'progress' | 'completed' | 'error';
  progress: number;
  direction: 'send' | 'receive';
  timestamp: Date;
}

// ═══════════════════════════════════════════════════════════════
// AUDIO
// ═══════════════════════════════════════════════════════════════

const playCompletionSound = () => {
  try {
    const ctx = new AudioContext();
    const osc = ctx.createOscillator();
    const gain = ctx.createGain();
    osc.connect(gain);
    gain.connect(ctx.destination);
    osc.frequency.setValueAtTime(880, ctx.currentTime);
    osc.frequency.setValueAtTime(1100, ctx.currentTime + 0.1);
    gain.gain.setValueAtTime(0.08, ctx.currentTime);
    gain.gain.exponentialRampToValueAtTime(0.01, ctx.currentTime + 0.3);
    osc.start(ctx.currentTime);
    osc.stop(ctx.currentTime + 0.3);
  } catch { /* silent */ }
};

// ═══════════════════════════════════════════════════════════════
// CUSTOM TITLEBAR
// ═══════════════════════════════════════════════════════════════

function TitleBar() {
  return (
    <div className="wails-drag h-11 flex items-center justify-between px-4 
      bg-void-surface/80 backdrop-blur-glass border-b border-white/5">
      <div className="flex items-center gap-2.5">
        <AeroLogo className="w-5 h-5" />
        <span className="text-sm font-semibold text-white/90 tracking-tight">Aero</span>
      </div>
      
      <div className="wails-no-drag flex items-center gap-0.5">
        <button
          onClick={() => window.runtime?.WindowMinimise()}
          className="p-2 rounded-lg hover:bg-white/5 transition-all duration-200 
            active:scale-95 transform-gpu"
          aria-label="Minimize"
        >
          <Minus className="w-4 h-4 text-white/40" />
        </button>
        <button
          onClick={() => window.runtime?.WindowClose()}
          className="p-2 rounded-lg hover:bg-red-500/20 transition-all duration-200 
            group active:scale-95 transform-gpu"
          aria-label="Close"
        >
          <X className="w-4 h-4 text-white/40 group-hover:text-red-400" />
        </button>
      </div>
    </div>
  );
}

// ═══════════════════════════════════════════════════════════════
// NETWORK SELECTOR
// ═══════════════════════════════════════════════════════════════

function NetworkSelector({
  interfaces,
  selected,
  onSelect,
  disabled,
}: {
  interfaces: NetworkInterface[];
  selected: NetworkInterface | null;
  onSelect: (iface: NetworkInterface) => void;
  disabled: boolean;
}) {
  const [open, setOpen] = useState(false);

  return (
    <div className="relative">
      <button
        onClick={() => !disabled && setOpen(!open)}
        disabled={disabled}
        className={`
          w-full flex items-center justify-between gap-3 px-4 py-3
          bg-void-surface/50 backdrop-blur-glass
          border border-void-border rounded-xl
          text-base transition-all duration-200
          ${disabled ? 'opacity-50 cursor-not-allowed' : 'hover:border-aero-cyan/30 hover:bg-void-elevated/50'}
          active:scale-[0.99] transform-gpu
        `}
      >
        <div className="flex items-center gap-3 min-w-0">
          <Wifi className="w-5 h-5 text-white/40 flex-shrink-0" />
          <div className="text-left">
            <p className="text-white/90 font-medium truncate">
              {selected ? selected.name : 'Select Network'}
            </p>
            <p className="text-xs text-white/40">
              {selected ? selected.ip : 'Choose your connection'}
            </p>
          </div>
        </div>
        <ChevronDown className={`w-5 h-5 text-white/40 transition-transform duration-200 ${open ? 'rotate-180' : ''}`} />
      </button>

      {open && !disabled && (
        <div className="absolute z-20 mt-2 w-full 
          bg-void-surface/95 backdrop-blur-glass
          border border-void-border rounded-xl 
          shadow-2xl shadow-black/50 overflow-hidden animate-scale-in">
          {interfaces.map((iface) => (
            <button
              key={iface.ip}
              onClick={() => { onSelect(iface); setOpen(false); }}
              className={`
                w-full px-4 py-3 text-left transition-all duration-150
                hover:bg-aero-cyan/10 active:scale-[0.99] transform-gpu
                ${selected?.ip === iface.ip ? 'bg-aero-cyan/10' : ''}
              `}
            >
              <p className="text-base font-medium text-white/90">{iface.name}</p>
              <p className="text-sm text-white/40">{iface.ip}</p>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

// ═══════════════════════════════════════════════════════════════
// QR CODE ZONE (Zone A - Receive Mode)
// ═══════════════════════════════════════════════════════════════

function QRZone({ url, active }: { url: string; active: boolean }) {
  return (
    <div className="flex flex-col items-center gap-5">
      {/* Big Label */}
      <h2 className="text-xl font-bold text-white/90 tracking-tight">
        {active ? 'Receive Files' : 'Ready to Connect'}
      </h2>
      
      {/* QR Container */}
      <div className={`
        relative p-5 rounded-3xl
        bg-white transition-all duration-500
        ${active ? 'shadow-glow-cyan animate-breathe' : 'opacity-50'}
      `}>
        {/* Glow */}
        {active && (
          <div className="absolute inset-0 bg-aero-cyan/30 rounded-3xl blur-2xl -z-10" />
        )}
        
        <QRCodeSVG
          value={url || 'https://aero.app'}
          size={160}
          level="M"
          bgColor="transparent"
          fgColor={active ? '#050505' : '#666666'}
        />
      </div>

      {/* Instructions */}
      <p className="text-base text-white/50 text-center max-w-[240px]">
        {active 
          ? 'Scan with your phone camera to start transferring' 
          : 'Start server to connect devices'}
      </p>
    </div>
  );
}

// ═══════════════════════════════════════════════════════════════
// POWER & SEND BUTTONS
// ═══════════════════════════════════════════════════════════════

function ActionButtons({
  active,
  loading,
  onToggle,
  onSendToPhone,
}: {
  active: boolean;
  loading: boolean;
  onToggle: () => void;
  onSendToPhone: () => void;
}) {
  return (
    <div className="flex items-center justify-center gap-4">
      {/* Power Button */}
      <button
        onClick={onToggle}
        disabled={loading}
        className={`
          relative w-16 h-16 rounded-2xl
          flex items-center justify-center
          transition-all duration-300
          active:scale-95 transform-gpu
          ${active 
            ? 'bg-aero-cyan/20 text-aero-cyan shadow-glow-cyan' 
            : 'bg-void-elevated text-white/40 hover:bg-void-border hover:text-white/60'}
          ${loading ? 'cursor-wait' : ''}
        `}
        aria-label={active ? 'Stop' : 'Start'}
      >
        {loading ? (
          <Loader2 className="w-7 h-7 animate-spin" />
        ) : (
          <Power className="w-7 h-7" />
        )}
        
        {active && !loading && (
          <div className="absolute inset-0 rounded-2xl border-2 border-aero-cyan/40 animate-ping" />
        )}
      </button>

      {/* Send to Phone Button */}
      {active && (
        <button
          onClick={onSendToPhone}
          className="
            w-16 h-16 rounded-2xl
            flex items-center justify-center
            bg-void-elevated text-white/60
            hover:bg-aero-cyan/10 hover:text-aero-cyan
            transition-all duration-300
            active:scale-95 transform-gpu
          "
          aria-label="Send file to phone"
        >
          <Send className="w-6 h-6" />
        </button>
      )}
    </div>
  );
}

// ═══════════════════════════════════════════════════════════════
// RECENT FILES (Zone B)
// ═══════════════════════════════════════════════════════════════

function TransferItem({ transfer }: { transfer: Transfer }) {
  const isSend = transfer.direction === 'send';
  
  return (
    <div className="flex items-center gap-4 px-4 py-3 
      bg-void-surface/30 backdrop-blur-sm
      rounded-xl border border-void-border/50
      animate-slide-up">
      {/* Icon */}
      <div className={`
        w-10 h-10 rounded-xl flex items-center justify-center
        ${transfer.status === 'completed' 
          ? 'bg-aero-cyan/15 text-aero-cyan' 
          : 'bg-void-elevated text-white/40'}
      `}>
        {transfer.status === 'completed' ? (
          <FileCheck className="w-5 h-5" />
        ) : transfer.status === 'error' ? (
          <X className="w-5 h-5 text-red-400" />
        ) : (
          <Loader2 className="w-5 h-5 animate-spin" />
        )}
      </div>
      
      {/* Info */}
      <div className="flex-1 min-w-0">
        <p className="text-base font-medium text-white/90 truncate">
          {transfer.filename}
        </p>
        <p className="text-sm text-white/40">
          {transfer.status === 'completed' && 'Complete'}
          {transfer.status === 'started' && 'Starting...'}
          {transfer.status === 'progress' && `${transfer.progress}%`}
          {transfer.status === 'error' && 'Failed'}
        </p>
      </div>

      {/* Direction Badge */}
      <div className={`
        flex items-center gap-1 px-2 py-1 rounded-lg text-xs font-medium
        ${isSend ? 'bg-blue-500/15 text-blue-400' : 'bg-aero-cyan/15 text-aero-cyan'}
      `}>
        {isSend ? <ArrowUpRight className="w-3 h-3" /> : <ArrowDownLeft className="w-3 h-3" />}
        {isSend ? 'Sent' : 'Received'}
      </div>
    </div>
  );
}

function RecentFilesZone({ transfers }: { transfers: Transfer[] }) {
  return (
    <div className="flex flex-col gap-3 flex-1 min-h-0">
      <h3 className="text-sm font-semibold text-white/50 uppercase tracking-wider px-1">
        Recent Files
      </h3>
      
      <div className="flex-1 overflow-y-auto space-y-2 scrollbar-thin">
        {transfers.length === 0 ? (
          <div className="flex items-center justify-center h-full">
            <p className="text-base text-white/30">No transfers yet</p>
          </div>
        ) : (
          transfers.map(t => <TransferItem key={t.id} transfer={t} />)
        )}
      </div>
    </div>
  );
}

// ═══════════════════════════════════════════════════════════════
// STATUS BAR
// ═══════════════════════════════════════════════════════════════

function StatusBar({
  active,
  soundEnabled,
  onOpenFolder,
  onToggleSound,
}: {
  active: boolean;
  soundEnabled: boolean;
  onOpenFolder: () => void;
  onToggleSound: () => void;
}) {
  return (
    <div className="flex items-center justify-between px-4 py-3 
      bg-void-surface/50 backdrop-blur-glass border-t border-void-border">
      {/* Status */}
      <div className="flex items-center gap-2">
        {active ? (
          <>
            <div className="w-2 h-2 rounded-full bg-aero-cyan animate-glow-pulse" />
            <span className="text-sm font-medium text-aero-cyan">Connected</span>
          </>
        ) : (
          <>
            <WifiOff className="w-4 h-4 text-white/30" />
            <span className="text-sm text-white/30">Offline</span>
          </>
        )}
      </div>
      
      {/* Actions */}
      <div className="flex items-center gap-1">
        <button
          onClick={onToggleSound}
          className="p-2 rounded-lg hover:bg-white/5 transition-all active:scale-95 transform-gpu"
          aria-label="Toggle sound"
        >
          {soundEnabled 
            ? <Volume2 className="w-4 h-4 text-white/40" />
            : <VolumeX className="w-4 h-4 text-white/30" />
          }
        </button>
        <button
          onClick={onOpenFolder}
          className="p-2 rounded-lg hover:bg-white/5 transition-all active:scale-95 transform-gpu"
          aria-label="Open folder"
        >
          <FolderOpen className="w-4 h-4 text-white/40" />
        </button>
      </div>
    </div>
  );
}

// ═══════════════════════════════════════════════════════════════
// MAIN APP
// ═══════════════════════════════════════════════════════════════

function App() {
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([]);
  const [selectedInterface, setSelectedInterface] = useState<NetworkInterface | null>(null);
  const [serverStatus, setServerStatus] = useState<ServerStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [soundEnabled, setSoundEnabled] = useState(true);
  const [isDragOver, setIsDragOver] = useState(false);
  
  // Term-Phase 2: Transfer HUD state
  const { state: activeTransfer, simulateTransfer } = useTransfer();
  
  const transferIdCounter = useRef(0);

  // Load interfaces
  useEffect(() => {
    (async () => {
      try {
        const ifaces = await window.go?.main.App.GetLocalIPs();
        if (ifaces?.length) {
          setInterfaces(ifaces);
          setSelectedInterface(ifaces[0]);
        }
      } catch (err) { console.error(err); }
    })();
  }, []);

  // Events
  useEffect(() => {
    const onStarted = (data: unknown) => {
      setServerStatus(data as ServerStatus);
      setLoading(false);
    };
    const onStopped = () => {
      setServerStatus(null);
      setLoading(false);
    };
    const onProgress = (data: unknown) => {
      const e = data as TransferEvent;
      setTransfers(prev => {
        const existing = prev.find(t => t.filename === e.filename && t.status !== 'completed');
        if (existing) {
          return prev.map(t => t.id === existing.id ? { ...t, status: e.status, progress: e.progress } : t);
        } else if (e.status === 'started') {
          return [{
            id: `t-${transferIdCounter.current++}`,
            filename: e.filename,
            status: e.status,
            progress: 0,
            direction: ((e as TransferEvent & { direction?: string }).direction === 'send' ? 'send' : 'receive') as 'send' | 'receive',
            timestamp: new Date(),
          }, ...prev].slice(0, 10);
        }
        return prev;
      });
      if (e.status === 'completed' && soundEnabled) playCompletionSound();
    };

    window.runtime?.EventsOn('server:started', onStarted);
    window.runtime?.EventsOn('server:stopped', onStopped);
    window.runtime?.EventsOn('transfer:progress', onProgress);

    return () => {
      window.runtime?.EventsOff('server:started');
      window.runtime?.EventsOff('server:stopped');
      window.runtime?.EventsOff('transfer:progress');
    };
  }, [soundEnabled]);

  const handleToggle = useCallback(async () => {
    if (loading) return;
    setLoading(true);
    try {
      if (serverStatus?.running) {
        await window.go?.main.App.StopServer();
      } else if (selectedInterface) {
        await window.go?.main.App.StartServer(selectedInterface.ip);
      }
    } catch { setLoading(false); }
  }, [loading, serverStatus, selectedInterface]);

  const handleSendToPhone = useCallback(async () => {
    try { await window.go?.main.App.SendFileToPhone(); } catch (e) { console.error(e); }
  }, []);

  const handleOpenFolder = useCallback(async () => {
    try { await window.go?.main.App.OpenDownloadsFolder(); } catch {}
  }, []);

  const isActive = serverStatus?.running ?? false;

  // Drag & Drop handlers
  const handleDragOver = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(true);
  };
  const handleDragLeave = () => setIsDragOver(false);
  const handleDrop = (e: React.DragEvent) => {
    e.preventDefault();
    setIsDragOver(false);
    // Future: handle dropped files
  };

  return (
    <div 
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      className={`
        h-full flex flex-col
        bg-void-black
        rounded-xl overflow-hidden
        border-2 transition-all duration-300
        ${isDragOver 
          ? 'border-aero-cyan shadow-glow-cyan-lg' 
          : 'border-void-border'}
      `}
    >
      <TitleBar />

      <div className="flex-1 flex flex-col px-5 py-5 gap-6 overflow-hidden">
        {/* Network Selector */}
        <NetworkSelector
          interfaces={interfaces}
          selected={selectedInterface}
          onSelect={setSelectedInterface}
          disabled={isActive}
        />

        {/* Zone A: QR / Receive */}
        <QRZone url={serverStatus?.url ?? ''} active={isActive} />

        {/* Action Buttons */}
        <ActionButtons
          active={isActive}
          loading={loading}
          onToggle={handleToggle}
          onSendToPhone={handleSendToPhone}
        />

        {/* Term-Phase 2: Active Transfer HUD */}
        <ActivePanel 
          transfer={activeTransfer} 
          onTest={() => simulateTransfer()}
        />

        {/* Zone B: Recent Files */}
        <RecentFilesZone transfers={transfers} />
      </div>

      <StatusBar
        active={isActive}
        soundEnabled={soundEnabled}
        onOpenFolder={handleOpenFolder}
        onToggleSound={() => setSoundEnabled(!soundEnabled)}
      />
    </div>
  );
}

export default App;
