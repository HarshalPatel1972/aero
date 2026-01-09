/**
 * AERO Desktop Application
 * Phase 3: The Interface
 * 
 * A sophisticated, "Invisible Tech" aesthetic file transfer UI.
 * Built with React, TypeScript, and TailwindCSS.
 */

import { useEffect, useState, useCallback, useRef } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import {
  Power,
  Minus,
  X,
  Folder,
  Wifi,
  WifiOff,
  ChevronDown,
  FileCheck,
  Loader2,
  Volume2,
  Upload,
} from 'lucide-react';
import type { NetworkInterface, ServerStatus, TransferEvent } from './types';

// ═══════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════

interface Transfer {
  id: string;
  filename: string;
  status: 'started' | 'progress' | 'completed' | 'error';
  progress: number;
  timestamp: Date;
}

// ═══════════════════════════════════════════════════════════════
// AUDIO FEEDBACK
// ═══════════════════════════════════════════════════════════════

const playCompletionSound = () => {
  try {
    const audioContext = new AudioContext();
    const oscillator = audioContext.createOscillator();
    const gainNode = audioContext.createGain();
    
    oscillator.connect(gainNode);
    gainNode.connect(audioContext.destination);
    
    oscillator.frequency.setValueAtTime(880, audioContext.currentTime);
    oscillator.frequency.setValueAtTime(1100, audioContext.currentTime + 0.1);
    
    gainNode.gain.setValueAtTime(0.1, audioContext.currentTime);
    gainNode.gain.exponentialRampToValueAtTime(0.01, audioContext.currentTime + 0.3);
    
    oscillator.start(audioContext.currentTime);
    oscillator.stop(audioContext.currentTime + 0.3);
  } catch {
    // Audio not available, fail silently
  }
};

// ═══════════════════════════════════════════════════════════════
// COMPONENTS
// ═══════════════════════════════════════════════════════════════

/**
 * Custom Title Bar with drag region and window controls.
 */
function TitleBar() {
  const handleMinimize = () => window.runtime?.WindowMinimise();
  const handleClose = () => window.runtime?.WindowClose();

  return (
    <div className="wails-drag h-10 flex items-center justify-between px-3 border-b border-white/5">
      <div className="flex items-center gap-2">
        <div className="w-3 h-3 rounded-full bg-emerald-500/80" />
        <span className="text-xs font-medium text-zinc-400 tracking-wide">AERO</span>
      </div>
      
      <div className="wails-no-drag flex items-center gap-1">
        <button
          onClick={handleMinimize}
          className="p-1.5 rounded hover:bg-white/5 transition-colors"
          aria-label="Minimize"
        >
          <Minus className="w-3.5 h-3.5 text-zinc-500" />
        </button>
        <button
          onClick={handleClose}
          className="p-1.5 rounded hover:bg-red-500/20 transition-colors group"
          aria-label="Close"
        >
          <X className="w-3.5 h-3.5 text-zinc-500 group-hover:text-red-400" />
        </button>
      </div>
    </div>
  );
}

/**
 * Network Interface Selector Dropdown.
 */
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
          w-full flex items-center justify-between gap-2 px-3 py-2
          bg-zinc-900/50 border border-white/10 rounded-lg
          text-sm transition-all
          ${disabled ? 'opacity-50 cursor-not-allowed' : 'hover:border-white/20'}
        `}
      >
        <div className="flex items-center gap-2 min-w-0">
          <Wifi className="w-4 h-4 text-zinc-500 flex-shrink-0" />
          <span className="text-zinc-300 truncate">
            {selected ? `${selected.name} (${selected.ip})` : 'Select network...'}
          </span>
        </div>
        <ChevronDown className={`w-4 h-4 text-zinc-500 transition-transform ${open ? 'rotate-180' : ''}`} />
      </button>

      {open && !disabled && (
        <div className="absolute z-10 mt-1 w-full bg-zinc-900 border border-white/10 rounded-lg shadow-xl overflow-hidden animate-fade-in">
          {interfaces.map((iface) => (
            <button
              key={iface.ip}
              onClick={() => {
                onSelect(iface);
                setOpen(false);
              }}
              className={`
                w-full px-3 py-2 text-left text-sm transition-colors
                hover:bg-white/5
                ${selected?.ip === iface.ip ? 'bg-white/5 text-zinc-100' : 'text-zinc-400'}
              `}
            >
              <div className="font-medium">{iface.name}</div>
              <div className="text-xs text-zinc-500">{iface.ip}</div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

/**
 * QR Code Display with breathing animation.
 */
function QRCodeSection({ url, active }: { url: string; active: boolean }) {
  return (
    <div className="flex flex-col items-center gap-4 py-6">
      <div
        className={`
          relative p-4 bg-white rounded-2xl
          ${active ? 'animate-pulse-slow' : 'opacity-40'}
          transition-opacity duration-500
        `}
      >
        {/* Glow effect */}
        {active && (
          <div className="absolute inset-0 bg-emerald-500/20 rounded-2xl blur-xl" />
        )}
        
        <div className="relative">
          <QRCodeSVG
            value={url || 'https://aero.local'}
            size={180}
            level="M"
            bgColor="transparent"
            fgColor={active ? '#18181b' : '#71717a'}
          />
        </div>
      </div>

      {active ? (
        <p className="text-xs text-zinc-500 text-center max-w-[200px]">
          Scan with your phone to transfer files securely
        </p>
      ) : (
        <p className="text-xs text-zinc-600 text-center">
          Start server to enable transfers
        </p>
      )}
    </div>
  );
}

/**
 * Power Toggle Button.
 */
function PowerButton({
  active,
  loading,
  onClick,
}: {
  active: boolean;
  loading: boolean;
  onClick: () => void;
}) {
  return (
    <button
      onClick={onClick}
      disabled={loading}
      className={`
        relative w-14 h-14 rounded-full
        flex items-center justify-center
        transition-all duration-300
        ${active 
          ? 'bg-emerald-500/20 text-emerald-400 shadow-lg shadow-emerald-500/20' 
          : 'bg-zinc-800/50 text-zinc-500 hover:bg-zinc-800 hover:text-zinc-400'
        }
        ${loading ? 'cursor-wait' : ''}
      `}
      aria-label={active ? 'Stop server' : 'Start server'}
    >
      {loading ? (
        <Loader2 className="w-6 h-6 animate-spin" />
      ) : (
        <Power className="w-6 h-6" />
      )}
      
      {/* Active ring */}
      {active && !loading && (
        <div className="absolute inset-0 rounded-full border-2 border-emerald-500/50 animate-ping" />
      )}
    </button>
  );
}

/**
 * Transfer List Item.
 */
function TransferItem({ transfer }: { transfer: Transfer }) {
  return (
    <div className="flex items-center gap-3 px-3 py-2 bg-zinc-900/30 rounded-lg animate-slide-up">
      <div className={`
        w-8 h-8 rounded-lg flex items-center justify-center
        ${transfer.status === 'completed' ? 'bg-emerald-500/20' : 'bg-zinc-800'}
      `}>
        {transfer.status === 'completed' ? (
          <FileCheck className="w-4 h-4 text-emerald-400" />
        ) : transfer.status === 'started' || transfer.status === 'progress' ? (
          <Loader2 className="w-4 h-4 text-zinc-400 animate-spin" />
        ) : (
          <X className="w-4 h-4 text-red-400" />
        )}
      </div>
      
      <div className="flex-1 min-w-0">
        <p className="text-sm text-zinc-200 truncate">{transfer.filename}</p>
        <p className="text-xs text-zinc-500">
          {transfer.status === 'completed' && 'Transferred successfully'}
          {transfer.status === 'started' && 'Starting transfer...'}
          {transfer.status === 'progress' && `${transfer.progress}%`}
          {transfer.status === 'error' && 'Transfer failed'}
        </p>
      </div>
    </div>
  );
}

/**
 * Recent Transfers Section.
 */
function TransferList({ transfers }: { transfers: Transfer[] }) {
  if (transfers.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center">
        <p className="text-xs text-zinc-600 text-center">
          No recent transfers
        </p>
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto scrollbar-thin space-y-2 px-1">
      {transfers.map((transfer) => (
        <TransferItem key={transfer.id} transfer={transfer} />
      ))}
    </div>
  );
}

/**
 * Status Bar.
 */
function StatusBar({
  active,
  onOpenFolder,
  soundEnabled,
  onToggleSound,
}: {
  active: boolean;
  onOpenFolder: () => void;
  soundEnabled: boolean;
  onToggleSound: () => void;
}) {
  return (
    <div className="flex items-center justify-between px-3 py-2 border-t border-white/5">
      <div className="flex items-center gap-2">
        {active ? (
          <>
            <Wifi className="w-3.5 h-3.5 text-emerald-400" />
            <span className="text-xs text-emerald-400">Ready</span>
          </>
        ) : (
          <>
            <WifiOff className="w-3.5 h-3.5 text-zinc-600" />
            <span className="text-xs text-zinc-600">Offline</span>
          </>
        )}
      </div>
      
      <div className="flex items-center gap-1">
        <button
          onClick={onToggleSound}
          className={`
            p-1.5 rounded transition-colors
            ${soundEnabled ? 'text-zinc-400 hover:text-zinc-300' : 'text-zinc-600'}
          `}
          aria-label="Toggle sound"
        >
          <Volume2 className="w-3.5 h-3.5" />
        </button>
        <button
          onClick={onOpenFolder}
          className="p-1.5 rounded text-zinc-500 hover:text-zinc-300 hover:bg-white/5 transition-colors"
          aria-label="Open downloads folder"
        >
          <Folder className="w-3.5 h-3.5" />
        </button>
      </div>
    </div>
  );
}

// ═══════════════════════════════════════════════════════════════
// MAIN APP
// ═══════════════════════════════════════════════════════════════

function App() {
  // State
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([]);
  const [selectedInterface, setSelectedInterface] = useState<NetworkInterface | null>(null);
  const [serverStatus, setServerStatus] = useState<ServerStatus | null>(null);
  const [loading, setLoading] = useState(false);
  const [transfers, setTransfers] = useState<Transfer[]>([]);
  const [soundEnabled, setSoundEnabled] = useState(true);
  
  const transferIdCounter = useRef(0);

  // Load network interfaces on mount
  useEffect(() => {
    const loadInterfaces = async () => {
      try {
        const ifaces = await window.go?.main.App.GetLocalIPs();
        if (ifaces && ifaces.length > 0) {
          setInterfaces(ifaces);
          setSelectedInterface(ifaces[0]);
        }
      } catch (err) {
        console.error('Failed to load interfaces:', err);
      }
    };

    loadInterfaces();
  }, []);

  // Subscribe to Wails events
  useEffect(() => {
    const handleServerStarted = (data: unknown) => {
      const status = data as ServerStatus;
      setServerStatus(status);
      setLoading(false);
    };

    const handleServerStopped = () => {
      setServerStatus(null);
      setLoading(false);
    };

    const handleTransferProgress = (data: unknown) => {
      const event = data as TransferEvent;
      
      setTransfers((prev) => {
        // Find existing transfer
        const existing = prev.find(
          (t) => t.filename === event.filename && t.status !== 'completed'
        );

        if (existing) {
          // Update existing
          return prev.map((t) =>
            t.id === existing.id
              ? { ...t, status: event.status, progress: event.progress }
              : t
          );
        } else if (event.status === 'started') {
          // Add new transfer at the beginning
          const newTransfer: Transfer = {
            id: `transfer-${transferIdCounter.current++}`,
            filename: event.filename,
            status: event.status,
            progress: 0,
            timestamp: new Date(),
          };
          return [newTransfer, ...prev].slice(0, 10); // Keep last 10
        }
        
        return prev;
      });

      // Play sound on completion
      if (event.status === 'completed' && soundEnabled) {
        playCompletionSound();
      }
    };

    window.runtime?.EventsOn('server:started', handleServerStarted);
    window.runtime?.EventsOn('server:stopped', handleServerStopped);
    window.runtime?.EventsOn('transfer:progress', handleTransferProgress);

    return () => {
      window.runtime?.EventsOff('server:started');
      window.runtime?.EventsOff('server:stopped');
      window.runtime?.EventsOff('transfer:progress');
    };
  }, [soundEnabled]);

  // Toggle server
  const handleToggleServer = useCallback(async () => {
    if (loading) return;

    setLoading(true);

    try {
      if (serverStatus?.running) {
        await window.go?.main.App.StopServer();
      } else if (selectedInterface) {
        await window.go?.main.App.StartServer(selectedInterface.ip);
      }
    } catch (err) {
      console.error('Server toggle failed:', err);
      setLoading(false);
    }
  }, [loading, serverStatus, selectedInterface]);

  // Open downloads folder
  const handleOpenFolder = useCallback(async () => {
    try {
      await window.go?.main.App.OpenDownloadsFolder();
    } catch (err) {
      console.error('Failed to open folder:', err);
    }
  }, []);

  const isActive = serverStatus?.running ?? false;

  return (
    <div className="h-full flex flex-col bg-zinc-950/95 backdrop-blur-sm rounded-lg overflow-hidden border border-white/5">
      <TitleBar />

      <div className="flex-1 flex flex-col px-4 py-3 gap-4 overflow-hidden">
        {/* Network Selector */}
        <NetworkSelector
          interfaces={interfaces}
          selected={selectedInterface}
          onSelect={setSelectedInterface}
          disabled={isActive}
        />

        {/* QR Code */}
        <QRCodeSection url={serverStatus?.url ?? ''} active={isActive} />

        {/* Power Button */}
        <div className="flex justify-center gap-4 items-center">
          <PowerButton
            active={isActive}
            loading={loading}
            onClick={handleToggleServer}
          />
          
          {/* Send to Phone Button */}
          {isActive && (
            <button
              onClick={async () => {
                try {
                  await window.go?.main.App.SendFileToPhone();
                } catch (err) {
                  console.error('Failed to send:', err);
                }
              }}
              className="
                relative w-14 h-14 rounded-full
                flex items-center justify-center
                bg-blue-500/20 text-blue-400
                hover:bg-blue-500/30 transition-all duration-300
                shadow-lg shadow-blue-500/10
              "
              aria-label="Send file to phone"
            >
              <Upload className="w-6 h-6" />
            </button>
          )}
        </div>

        {/* Section Header */}
        <div className="flex items-center gap-2 pt-2">
          <div className="h-px flex-1 bg-white/5" />
          <span className="text-[10px] uppercase tracking-wider text-zinc-600 font-medium">
            Recent
          </span>
          <div className="h-px flex-1 bg-white/5" />
        </div>

        {/* Transfer List */}
        <TransferList transfers={transfers} />
      </div>

      <StatusBar
        active={isActive}
        onOpenFolder={handleOpenFolder}
        soundEnabled={soundEnabled}
        onToggleSound={() => setSoundEnabled(!soundEnabled)}
      />
    </div>
  );
}

export default App;
