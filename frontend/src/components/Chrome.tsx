import { AlertCircle, CheckCircle2, FolderOpen, History, Info, Minus, PictureInPicture2, Trash2, Volume2, VolumeX, X } from 'lucide-react';
import { AeroMark, Wordmark } from './brand/AeroMark';
import { Recent } from './Deck';
import type { Aero, Toast } from '../hooks/useAero';

export function TitleBar({ onStory, onHistory }: { onStory: () => void; onHistory: () => void }) {
  return (
    <header className="titlebar">
      <div className="brand">
        <AeroMark size={24} />
        <Wordmark />
      </div>
      <div style={{ display: 'flex', gap: 2 }}>
        <button className="icon-btn" onClick={onStory} title="The Aero story & how it works" aria-label="How Aero works">
          <Info size={16} />
        </button>
        <button className="icon-btn" onClick={onHistory} title="Flight history" aria-label="Transfer history">
          <History size={16} />
        </button>
        <button className="icon-btn" onClick={() => window.runtime?.WindowMinimise()} title="Minimise" aria-label="Minimise">
          <Minus size={16} />
        </button>
        <button className="icon-btn close" onClick={() => window.runtime?.Quit()} title="Quit Aero" aria-label="Quit">
          <X size={16} />
        </button>
      </div>
    </header>
  );
}

export function Footer({ aero, onMini }: { aero: Aero; onMini: () => void }) {
  return (
    <footer className="footer">
      <button className="path" onClick={aero.openFolder} title="Open the folder received files are saved to">
        <FolderOpen size={15} />
        Downloads <span style={{ color: 'var(--smoke-4)' }}>›</span> Aero
      </button>
      <div style={{ display: 'flex', gap: 2 }}>
        <button className="icon-btn" onClick={() => aero.setSound(!aero.sound)} aria-pressed={aero.sound} title={aero.sound ? 'Mute landing sound' : 'Play a sound when files land'} aria-label="Landing sound">
          {aero.sound ? <Volume2 size={16} /> : <VolumeX size={16} />}
        </button>
        <button className="icon-btn" onClick={onMini} title="Mini mode: a slim bar that stays on top" aria-label="Mini mode">
          <PictureInPicture2 size={16} />
        </button>
      </div>
    </footer>
  );
}

export function Toasts({ toasts }: { toasts: Toast[] }) {
  return (
    <div className="toasts" role="status" aria-live="polite">
      {toasts.map((t) => (
        <div className="toast fade-in" data-kind={t.kind} key={t.key}>
          {t.kind === 'error' ? <AlertCircle size={16} color="var(--danger)" /> : t.kind === 'success' ? <CheckCircle2 size={16} /> : <Info size={16} className="muted" />}
          <span>{t.message}</span>
        </div>
      ))}
    </div>
  );
}

export function HistorySheet({ aero, onClose }: { aero: Aero; onClose: () => void }) {
  return (
    <>
      <div className="overlay fade-in" onClick={onClose} />
      <div className="sheet fade-in" role="dialog" aria-label="Flight history">
        <div className="grabber" />
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 10 }}>
          <div>
            <div className="label">Flight log</div>
            <div className="display" style={{ fontSize: 20, marginTop: 4 }}>
              History
            </div>
          </div>
          <div style={{ display: 'flex', gap: 2 }}>
            {aero.history.length > 0 && (
              <button className="icon-btn" onClick={aero.clearHistory} title="Clear history (files are not deleted)" aria-label="Clear history">
                <Trash2 size={16} />
              </button>
            )}
            <button className="icon-btn" onClick={onClose} aria-label="Close">
              <X size={16} />
            </button>
          </div>
        </div>
        {aero.history.length === 0 ? (
          <p className="muted" style={{ fontSize: 13, padding: '24px 0 32px', textAlign: 'center' }}>
            No flights yet. Transfers you make appear here.
          </p>
        ) : (
          <Recent items={aero.history} />
        )}
        <p style={{ fontSize: 11.5, color: 'var(--smoke-3)', margin: '12px 0 0' }}>History stays on this PC only.</p>
      </div>
    </>
  );
}
