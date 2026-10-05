import { useCallback, useMemo } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { GripVertical, Maximize2 } from 'lucide-react';
import { FlowCanvas } from './FlowCanvas';
import { AeroMark } from './brand/AeroMark';
import type { Aero } from '../hooks/useAero';
import type { Field, FlowStyle } from '../lib/flow';
import { splitRate } from '../lib/format';

const W = 558;
const H = 110;

/** A slim, always-on-top strip: QR, live status, and a way back. */
export function MiniBar({ aero, onExpand }: { aero: Aero; onExpand: () => void }) {
  const { phase, status, active } = aero;
  const t = active[active.length - 1];
  const field = useCallback((): Field => () => [1, 0], []);
  const style = useMemo<FlowStyle>(
    () => ({
      line: '#E8ECF0',
      lineAlpha: 0.05,
      pulse: phase === 'busy' ? '#FF5B1F' : '#E8ECF0',
      pulseAlpha: phase === 'off' ? 0.1 : 0.5,
      pulses: 2,
      speed: phase === 'busy' ? 220 : phase === 'off' ? 10 : 50,
      tail: 50,
    }),
    [phase],
  );

  let title = 'Airflow off';
  let detail = 'Expand to start Aero';
  if (phase === 'waiting') {
    title = 'Scan to link';
    detail = 'Point your phone camera at the code';
  } else if (phase === 'linked') {
    title = 'Phone linked';
    detail = 'Drop files here to send';
  } else if (phase === 'busy' && t) {
    const [v, u] = splitRate(t.rate);
    title = t.name;
    detail = `${t.direction === 'receive' ? 'Phone → PC' : 'PC → Phone'} · ${Math.floor((t.bytes / Math.max(1, t.size)) * 100)}% · ${v} ${u}`;
  }

  return (
    <div className="mini">
      <FlowCanvas width={W} height={H} field={field} style={style} lines={9} />
      <div className="grip" title="Drag to move">
        <GripVertical size={16} />
      </div>
      <div className="qr" style={{ display: 'grid', placeItems: 'center', background: status?.running ? undefined : 'transparent', border: status?.running ? undefined : '1px solid var(--hairline-strong)' }}>
        {status?.running ? <QRCodeSVG value={status.url} level="M" size={76} bgColor="transparent" fgColor="#0B0D10" marginSize={0} /> : <AeroMark size={44} />}
      </div>
      <div style={{ minWidth: 0 }} aria-live="polite">
        <div className="label" style={{ color: phase === 'busy' ? 'var(--signal)' : undefined }}>
          {phase === 'busy' ? 'In flight' : 'Aero'}
        </div>
        <div className="display" style={{ fontSize: 17, marginTop: 4, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {title}
        </div>
        <div className="mono" style={{ fontSize: 11, color: 'var(--smoke-3)', marginTop: 4, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
          {detail}
        </div>
      </div>
      <button className="icon-btn" onClick={onExpand} title="Back to full view" aria-label="Expand">
        <Maximize2 size={16} />
      </button>
    </div>
  );
}
