import { useState } from 'react';
import { ArrowDownLeft, ArrowUpRight, Check, ChevronDown, Plus, RefreshCw, Wifi, X } from 'lucide-react';
import type { Aero, HistoryItem } from '../hooks/useAero';
import { formatBytes, formatEta, splitRate, timeAgo } from '../lib/format';

/** The lower half of the window: what to do next, for the current phase. */
export function Deck({ aero }: { aero: Aero }) {
  return (
    <div className="deck">
      {aero.phase === 'off' && <Idle aero={aero} />}
      {aero.phase === 'waiting' && <Waiting aero={aero} />}
      {aero.phase === 'linked' && <Linked aero={aero} />}
      {aero.phase === 'busy' && <Busy aero={aero} />}
    </div>
  );
}

function Idle({ aero }: { aero: Aero }) {
  return (
    <>
      <div className="fade-in">
        <h1 className="display headline" style={{ margin: 0 }}>
          Zero-drag file transfer.
        </h1>
        <p className="sub">Files flow straight between your phone and this PC over your own Wi-Fi. No cables, no cloud, no accounts.</p>
      </div>
      <NetworkPicker aero={aero} />
    </>
  );
}

function NetworkPicker({ aero }: { aero: Aero }) {
  const [open, setOpen] = useState(false);
  const { interfaces, selected } = aero;
  return (
    <div style={{ position: 'relative' }}>
      <div className="label" style={{ marginBottom: 8 }}>
        Network
      </div>
      {interfaces.length === 0 ? (
        <button className="select" onClick={aero.refreshInterfaces}>
          <Wifi size={16} className="muted" />
          <span className="grow">
            <span className="name" style={{ display: 'block' }}>
              No network found
            </span>
            <span className="label" style={{ letterSpacing: '0.06em', textTransform: 'none' }}>
              Connect to Wi-Fi, then tap to retry
            </span>
          </span>
          <RefreshCw size={15} className="muted" />
        </button>
      ) : (
        <button className="select" onClick={() => setOpen((o) => !o)} aria-haspopup="listbox" aria-expanded={open}>
          <Wifi size={16} style={{ color: 'var(--signal)' }} />
          <span className="grow">
            <span className="name" style={{ display: 'block' }}>
              {selected?.name}
            </span>
            <span className="mono" style={{ fontSize: 11, color: 'var(--smoke-3)' }}>
              {selected?.ip}
            </span>
          </span>
          {interfaces.length > 1 && <ChevronDown size={16} className="muted" style={{ transform: open ? 'rotate(180deg)' : undefined, transition: 'transform .2s' }} />}
        </button>
      )}
      {open && interfaces.length > 1 && (
        <div className="menu" role="listbox" style={{ left: 0, right: 0 }}>
          {interfaces.map((i) => (
            <button
              key={i.ip}
              role="option"
              aria-selected={i.ip === selected?.ip}
              onClick={() => {
                aero.setSelected(i);
                setOpen(false);
              }}
            >
              <span>
                {i.name}
                <span className="mono" style={{ display: 'block', fontSize: 11, color: 'var(--smoke-3)' }}>
                  {i.ip}
                </span>
              </span>
              {i.ip === selected?.ip && <Check size={15} style={{ color: 'var(--signal)' }} />}
            </button>
          ))}
        </div>
      )}
      <p style={{ margin: '10px 2px 0', fontSize: 12, color: 'var(--smoke-3)', lineHeight: 1.5 }}>Your phone must be on this same network.</p>
    </div>
  );
}

function Waiting({ aero }: { aero: Aero }) {
  return (
    <div className="fade-in" key="waiting">
      <h1 className="display headline" style={{ margin: 0 }}>
        Scan to link your phone.
      </h1>
      <p className="sub">Point your phone's camera at the code. Aero opens in its browser, with nothing to install.</p>
      <ol className="steps" style={{ marginTop: 18 }}>
        <li>
          <span className="n">01</span>Open the camera app and aim at the code
        </li>
        <li>
          <span className="n">02</span>Tap the link that appears
        </li>
        <li>
          <span className="n">03</span>
          <span>
            Keep both devices on <strong style={{ color: 'var(--smoke)', fontWeight: 560 }}>{aero.selected?.name ?? 'the same Wi-Fi'}</strong>
          </span>
        </li>
      </ol>
    </div>
  );
}

function Linked({ aero }: { aero: Aero }) {
  return (
    <>
      <div className="fade-in" key="linked">
        <h1 className="display headline" style={{ margin: 0 }}>
          Phone linked.
        </h1>
        <p className="sub">Send from your phone, or drop files anywhere on this window to fly them across.</p>
      </div>
      <SendZone aero={aero} />
      <Recent items={aero.history} limit={3} />
    </>
  );
}

function SendZone({ aero }: { aero: Aero }) {
  return (
    <div className="dropzone">
      <div style={{ flex: 1 }}>
        <div style={{ fontSize: 13.5, fontWeight: 560 }}>Drop files to send</div>
        <div style={{ fontSize: 12, color: 'var(--smoke-3)', marginTop: 3 }}>They land in your phone's downloads</div>
      </div>
      <button className="btn btn-primary" onClick={aero.chooseFiles}>
        <Plus size={16} strokeWidth={2.4} />
        Choose
      </button>
    </div>
  );
}

function Busy({ aero }: { aero: Aero }) {
  const t = aero.active[aero.active.length - 1];
  const queued = aero.active.length - 1;
  const pct = t.size ? Math.min(100, (t.bytes / t.size) * 100) : 100;
  const [value, unit] = splitRate(t.rate);
  const eta = t.rate > 0 ? (t.size - t.bytes) / t.rate : NaN;
  const incoming = t.direction === 'receive';

  return (
    <>
      <div className="telemetry fade-in" aria-live="polite">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12 }}>
          <div className="file">
            <span className={`dir ${incoming ? 'in' : ''}`}>{incoming ? <ArrowDownLeft size={14} /> : <ArrowUpRight size={14} />}</span>
            <span className="name">{t.name}</span>
          </div>
          <span className="label" style={{ flex: 'none' }}>
            {incoming ? 'Phone → PC' : 'PC → Phone'}
          </span>
        </div>
        <div className="streambar" role="progressbar" aria-valuenow={Math.round(pct)} aria-valuemin={0} aria-valuemax={100}>
          <i style={{ width: `${pct}%` }} />
        </div>
        <div className="readout">
          <div>
            <div className="label">Velocity</div>
            <div className="value">
              {value}
              <small>{unit}</small>
            </div>
          </div>
          <div>
            <div className="label">Done</div>
            <div className="value">
              {Math.floor(pct)}
              <small>%</small>
            </div>
          </div>
          <div>
            <div className="label">ETA</div>
            <div className="value">{formatEta(eta)}</div>
          </div>
        </div>
        <div style={{ marginTop: 12, display: 'flex', justifyContent: 'space-between', alignItems: 'center', gap: 12 }}>
          <span className="mono" style={{ fontSize: 12, color: 'var(--smoke-3)' }}>
            {formatBytes(t.bytes)} of {formatBytes(t.size)}
            {queued > 0 && ` · ${queued} more in the air`}
          </span>
          <button className="btn btn-ghost" style={{ height: 34, padding: '0 14px', fontSize: 13 }} onClick={() => aero.cancel(t.id)} title="Stop this transfer. Nothing partial is kept on either device.">
            <X size={14} />
            Cancel
          </button>
        </div>
      </div>
      <Recent items={aero.history} limit={2} />
    </>
  );
}

export function Recent({ items, limit }: { items: HistoryItem[]; limit?: number }) {
  const shown = limit ? items.slice(0, limit) : items;
  if (shown.length === 0) return null;
  return (
    <div style={{ minHeight: 0, flex: 1, display: 'flex', flexDirection: 'column', paddingBottom: 12 }}>
      {limit && (
        <div className="label" style={{ marginBottom: 4 }}>
          Recent flights
        </div>
      )}
      <div className="recent">
        {shown.map((h) => (
          <div className="recent-row" key={h.id + h.at}>
            <span className={`dir ${h.direction === 'receive' ? 'in' : ''}`}>
              {!h.ok ? <X size={13} /> : h.direction === 'receive' ? <ArrowDownLeft size={13} /> : <ArrowUpRight size={13} />}
            </span>
            <span className="name" style={{ color: h.ok ? undefined : 'var(--smoke-3)' }}>
              {h.name}
            </span>
            <span className="meta">
              {h.ok ? formatBytes(h.size) : h.cancelled ? 'cancelled' : 'failed'}
              <br />
              {timeAgo(h.at)}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}
