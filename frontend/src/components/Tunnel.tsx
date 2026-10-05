import { useCallback, useMemo } from 'react';
import { QRCodeSVG } from 'qrcode.react';
import { Loader2, Power } from 'lucide-react';
import { FlowCanvas } from './FlowCanvas';
import { AeroMark } from './brand/AeroMark';
import { cylinderField, type FlowStyle } from '../lib/flow';
import { splitRate } from '../lib/format';
import type { Aero } from '../hooks/useAero';

export const TUNNEL_W = 418;
export const TUNNEL_H = 330;
const BODY_RADIUS = 116;

const SMOKE = '#E8ECF0';
const SIGNAL = '#FF5B1F';

/** Maps the tunnel's phase and live speed to how the smoke behaves. */
function flowStyle(phase: Aero['phase'], rate: number): FlowStyle {
  switch (phase) {
    case 'off':
      return { line: SMOKE, lineAlpha: 0.06, pulse: SMOKE, pulseAlpha: 0.16, pulses: 2, speed: 14, tail: 34 };
    case 'waiting':
      return { line: SMOKE, lineAlpha: 0.1, pulse: SMOKE, pulseAlpha: 0.5, pulses: 2, speed: 48, tail: 40 };
    case 'linked':
      return { line: SMOKE, lineAlpha: 0.12, pulse: SMOKE, pulseAlpha: 0.75, pulses: 3, speed: 72, tail: 46 };
    case 'busy': {
      const mb = rate / 1048576;
      const speed = Math.min(340, 80 + Math.log2(1 + mb) * 42);
      return { line: SMOKE, lineAlpha: 0.14, pulse: SIGNAL, pulseAlpha: 0.95, pulses: 4, speed, tail: 60 };
    }
  }
}

const PHASE_LABEL: Record<Aero['phase'], string> = {
  off: 'Airflow off',
  waiting: 'Airflow on',
  linked: 'Phone linked',
  busy: 'In flight',
};

/**
 * The test section: live streamlines part around the central body — the
 * start button when idle, the QR code once Aero is running.
 */
export function Tunnel({ aero }: { aero: Aero }) {
  const { phase, status, pending, active } = aero;
  const rate = active.reduce((sum, t) => sum + t.rate, 0);
  const style = useMemo(() => flowStyle(phase, rate), [phase, Math.round(rate / 262144)]); // eslint-disable-line react-hooks/exhaustive-deps
  const field = useCallback((w: number, h: number) => cylinderField(w / 2, h / 2, BODY_RADIUS), []);
  const [value, unit] = splitRate(rate);

  return (
    <section className={`tunnel ${phase === 'linked' || phase === 'busy' ? 'linked' : ''}`} aria-label="Connection">
      <FlowCanvas width={TUNNEL_W} height={TUNNEL_H} field={field} style={style} lines={26} />

      <div style={{ position: 'absolute', left: 18, right: 14, top: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center', zIndex: 3 }}>
        <span className="pill" data-state={phase === 'waiting' ? 'on' : phase} role="status" aria-live="polite">
          <span className="dot" />
          {PHASE_LABEL[phase]}
        </span>
        {status?.running && (
          <button className="icon-btn" onClick={aero.stop} disabled={pending} title="Stop Aero (destroys this session's key)" aria-label="Stop Aero">
            {pending ? <Loader2 size={16} className="spin" /> : <Power size={16} />}
          </button>
        )}
      </div>

      <div className="body-card">
        <div className="fixture" aria-hidden="true">
          <i />
          <i />
          <i />
          <i />
        </div>
        {status?.running ? (
          <div className="qr-card">
            <QRCodeSVG value={status.url} level="Q" bgColor="transparent" fgColor="#0B0D10" marginSize={0} />
            <div className="badge">
              <AeroMark size={40} tile />
            </div>
          </div>
        ) : (
          <button className="start-btn" onClick={aero.start} disabled={pending || !aero.selected}>
            {pending ? <Loader2 size={30} className="spin" /> : <AeroMark size={50} animated />}
            <span className="display" style={{ fontSize: 17 }}>
              Start
            </span>
          </button>
        )}
      </div>

      <div className="tunnel-caption" aria-hidden="true">
        <span className="label">Test section</span>
        <span className="label">
          U∞ <span style={{ color: phase === 'busy' ? 'var(--signal)' : undefined }}>{phase === 'busy' ? `${value} ${unit}` : '—'}</span>
        </span>
      </div>
    </section>
  );
}
