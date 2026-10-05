import { useCallback, useEffect, useState } from 'react';
import { ArrowRight, KeyRound } from 'lucide-react';
import { AeroMark } from './brand/AeroMark';
import { FlowCanvas } from './FlowCanvas';
import { cylinderField, ellipseField, type FlowStyle } from '../lib/flow';

const W = 418;
const H = 360;

const smoke = (alpha = 0.7, speed = 60): FlowStyle => ({
  line: '#E8ECF0',
  lineAlpha: 0.1,
  pulse: '#E8ECF0',
  pulseAlpha: alpha,
  pulses: 3,
  speed,
  tail: 50,
});
const signal: FlowStyle = { ...smoke(0.95, 90), pulse: '#FF5B1F', lineAlpha: 0.12 };

interface Slide {
  kicker: string;
  title: string;
  body: React.ReactNode;
  stage: React.ReactNode;
}

/**
 * First-run story: why it's called Aero, the idea behind it, how to use it,
 * and how it keeps files private. Reopen it from the title bar.
 */
export function Story({ onDone }: { onDone: () => void }) {
  const [i, setI] = useState(0);
  const slides = useSlides();
  const last = i === slides.length - 1;

  const next = useCallback(() => (last ? onDone() : setI((n) => n + 1)), [last, onDone]);
  const back = useCallback(() => setI((n) => Math.max(0, n - 1)), []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'ArrowRight' || e.key === 'Enter') next();
      if (e.key === 'ArrowLeft') back();
      if (e.key === 'Escape') onDone();
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [next, back, onDone]);

  const s = slides[i];
  return (
    <div className="story" role="dialog" aria-label="Welcome to Aero" aria-modal="true">
      <div className="titlebar" style={{ position: 'absolute', left: 0, right: 0, top: 0 }}>
        <span className="label">{String(i + 1).padStart(2, '0')} / {String(slides.length).padStart(2, '0')}</span>
        <button className="icon-btn" style={{ width: 'auto', padding: '0 12px', fontSize: 12.5 }} onClick={onDone}>
          Skip
        </button>
      </div>
      <div className="stage" key={`stage-${i}`}>
        {s.stage}
      </div>
      <div className="copy fade-in" key={`copy-${i}`}>
        <div className="label kicker">{s.kicker}</div>
        <h1 className="display">{s.title}</h1>
        <div>{s.body}</div>
      </div>
      <div className="nav">
        <div className="dots" aria-hidden="true">
          {slides.map((_, k) => (
            <i key={k} data-on={k === i} />
          ))}
        </div>
        <div style={{ display: 'flex', gap: 8 }}>
          {i > 0 && (
            <button className="btn btn-ghost" onClick={back}>
              Back
            </button>
          )}
          <button className="btn btn-primary" onClick={next}>
            {last ? 'Get started' : 'Next'}
            <ArrowRight size={16} />
          </button>
        </div>
      </div>
    </div>
  );
}

function useSlides(): Slide[] {
  // Air parting around the A, which sits in the flow like a model in a tunnel.
  const markField = useCallback((w: number, h: number) => ellipseField(w / 2, h / 2 + 18, 128, 100), []);
  const sealField = useCallback((w: number, h: number) => cylinderField(w / 2, h / 2 + 10, 64), []);

  return [
    {
      kicker: '01 — The name',
      title: 'Aero comes from aerodynamics.',
      body: <p>The science of moving with the least resistance. Our A is drawn from airflow itself: smoke rising over a peak, crossed by the orange streamline your files ride on.</p>,
      stage: (
        <>
          <FlowCanvas width={W} height={H} field={markField} style={smoke(0.55, 60)} lines={28} />
          <div style={{ position: 'absolute', left: '50%', top: '50%', transform: 'translate(-50%, -40%)' }}>
            <AeroMark size={210} animated />
          </div>
        </>
      ),
    },
    {
      kicker: '02 — The idea',
      title: 'We took the drag out of sharing files.',
      body: <p>Most transfers take the long way: up to a cloud, through an account, then back down. Aero flies your files in a straight line across your own Wi-Fi. Faster, and nothing stored anywhere else.</p>,
      stage: <Lanes />,
    },
    {
      kicker: '03 — How it flows',
      title: 'Three moves. No app on your phone.',
      body: (
        <ol className="steps" style={{ marginTop: 14 }}>
          <li>
            <span className="n">01</span>Press Start on this PC
          </li>
          <li>
            <span className="n">02</span>Scan the code with your phone's camera
          </li>
          <li>
            <span className="n">03</span>Send from either side. Drop files here, or tap Send on your phone
          </li>
        </ol>
      ),
      stage: <Devices />,
    },
    {
      kicker: '04 — Sealed airflow',
      title: 'Encrypted, every single flight.',
      body: <p>Each session gets a one-time key that travels only inside the QR code. Files and their names are encrypted end to end, and the key is destroyed the moment you stop Aero.</p>,
      stage: (
        <>
          <FlowCanvas width={W} height={H} field={sealField} style={signal} lines={26} />
          <div style={{ position: 'absolute', left: '50%', top: '50%', width: 112, height: 112, margin: '-46px 0 0 -56px', borderRadius: '50%', display: 'grid', placeItems: 'center', background: 'radial-gradient(circle at 50% 35%, #1a1e24, #0e1115 70%)', border: '1px solid rgba(255,91,31,.5)', boxShadow: '0 0 0 10px rgba(255,91,31,.08), 0 0 60px -10px rgba(255,91,31,.4)' }}>
            <KeyRound size={34} color="#FF5B1F" strokeWidth={1.6} />
          </div>
        </>
      ),
    },
  ];
}

/** Two routes: the turbulent detour through a cloud, and Aero's straight line. */
function Lanes() {
  // The detour climbs to the cloud and back, with a little turbulence on the way.
  const detour = 'M30 176 C 70 176, 92 150, 112 128 S 150 104, 166 108 S 190 96, 209 96 S 236 108, 252 104 S 290 120, 306 128 S 350 176, 388 176';
  return (
    <svg className="layer lanes" width={W} height={H} viewBox={`0 0 ${W} ${H}`} aria-hidden="true">
      <path d="M181 92 a16 16 0 0 1 6 -30 a22 22 0 0 1 40 -6 a15 15 0 0 1 20 16 a13 13 0 0 1 -4 20 z" fill="#0b0d10" stroke="#3a4048" strokeWidth="1.5" />
      <path d={detour} fill="none" stroke="#3a4048" strokeWidth="1.5" />
      {[0, 1].map((k) => (
        <path key={k} d={detour} fill="none" stroke="#a2abb5" strokeWidth="1.8" strokeLinecap="round" strokeDasharray="8 192" style={{ animation: `dash 3.6s linear ${-k * 1.8}s infinite` }} />
      ))}
      <circle cx="30" cy="176" r="3" fill="#646d77" />
      <circle cx="388" cy="176" r="3" fill="#646d77" />
      <text x="30" y="204">THE USUAL WAY · VIA CLOUD</text>

      <line x1="30" y1="262" x2="388" y2="262" stroke="#3a4048" strokeWidth="1.5" />
      {[0, 1, 2].map((k) => (
        <line key={k} x1="30" y1="262" x2="388" y2="262" stroke="#FF5B1F" strokeWidth="2.2" strokeLinecap="round" strokeDasharray="22 178" style={{ animation: `dash 1.1s linear ${(-k * 1.1) / 3}s infinite` }} />
      ))}
      <circle cx="30" cy="262" r="3.5" fill="#FF5B1F" />
      <circle cx="388" cy="262" r="3.5" fill="#FF5B1F" />
      <text x="30" y="290" style={{ fill: '#FF5B1F' }}>
        AERO · DIRECT
      </text>
      <text x="30" y="160">PHONE</text>
      <text x="388" y="160" textAnchor="end">
        PC
      </text>
    </svg>
  );
}

/** A phone and a PC joined by a laminar band of streamlines. */
function Devices() {
  const ys = [-18, -9, 0, 9, 18];
  return (
    <svg className="layer lanes" width={W} height={H} viewBox={`0 0 ${W} ${H}`} aria-hidden="true">
      <rect x="44" y="120" width="70" height="132" rx="14" fill="none" stroke="#a2abb5" strokeWidth="1.5" />
      <rect x="60" y="150" width="38" height="38" rx="4" fill="none" stroke="#646d77" strokeWidth="1.2" strokeDasharray="3 3" />
      <rect x="282" y="132" width="104" height="72" rx="8" fill="none" stroke="#a2abb5" strokeWidth="1.5" />
      <path d="M318 222 h32 M334 204 v18" stroke="#a2abb5" strokeWidth="1.5" />
      <rect x="312" y="146" width="44" height="44" rx="4" fill="#E8ECF0" opacity=".9" />
      {ys.map((dy, k) => (
        <g key={k}>
          <path d={`M118 ${186 + dy} C 180 ${186 + dy * 0.4}, 220 ${168 + dy * 0.4}, 278 ${168 + dy}`} fill="none" stroke="#3a4048" strokeWidth="1.2" />
          <path
            d={`M118 ${186 + dy} C 180 ${186 + dy * 0.4}, 220 ${168 + dy * 0.4}, 278 ${168 + dy}`}
            fill="none"
            stroke={k === 2 ? '#FF5B1F' : '#E8ECF0'}
            strokeWidth={k === 2 ? 2.2 : 1.4}
            strokeLinecap="round"
            strokeDasharray="18 182"
            style={{ animation: `dash ${1.4 + Math.abs(dy) / 40}s linear ${-k * 0.3}s infinite` }}
          />
        </g>
      ))}
      <text x="79" y="280" textAnchor="middle">
        SCAN
      </text>
      <text x="334" y="250" textAnchor="middle">
        START
      </text>
    </svg>
  );
}
