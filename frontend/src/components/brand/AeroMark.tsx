import { useId } from 'react';
import { CROSSBAR, STRANDS } from './geometry';

/** Fewer, bolder strands at small sizes so the A stays crisp. */
function detail(px: number) {
  if (px >= 96) return { strands: 3, width: 2.6, cross: 2.6, opacity: [1, 0.62, 0.36] };
  if (px >= 40) return { strands: 3, width: 3.2, cross: 3.2, opacity: [1, 0.6, 0.34] };
  if (px >= 24) return { strands: 2, width: 4, cross: 4, opacity: [1, 0.5] };
  return { strands: 1, width: 5.4, cross: 5.2, opacity: [1] };
}

/**
 * The Aero mark, the Streamline A: the letter A drawn only from wind-tunnel
 * smoke rising over a peak, crossed by the orange streamline files ride on.
 *
 * `animated` sends pulses of air along the strands and the crossbar.
 */
export function AeroMark({ size = 22, tile = false, animated = false }: { size?: number; tile?: boolean; animated?: boolean }) {
  const clip = useId();
  const d = detail(size);
  const art = (
    <>
      {STRANDS.slice(0, d.strands).map((path, k) => (
        <g key={k}>
          <path d={path} fill="none" stroke="var(--smoke)" strokeOpacity={d.opacity[k]} strokeWidth={d.width} strokeLinecap="round" strokeLinejoin="round" />
          {animated && (
            <path
              d={path}
              fill="none"
              stroke="#fff"
              strokeWidth={d.width * 0.9}
              strokeLinecap="round"
              pathLength={100}
              strokeDasharray="7 93"
              style={{ animation: `mark-flow ${2.4 + k * 0.5}s linear ${-k * 0.7}s infinite` }}
            />
          )}
        </g>
      ))}
      <path d={CROSSBAR} fill="none" stroke="var(--signal)" strokeWidth={d.cross} strokeLinecap="round" />
      {animated && (
        <path d={CROSSBAR} fill="none" stroke="#ffd2bf" strokeWidth={d.cross * 0.8} strokeLinecap="round" pathLength={100} strokeDasharray="10 90" style={{ animation: 'mark-flow 1.6s linear infinite' }} />
      )}
    </>
  );

  return (
    <svg width={size} height={size} viewBox="0 0 64 64" aria-hidden="true" style={tile ? undefined : { overflow: 'visible' }}>
      {tile ? (
        <>
          <defs>
            <clipPath id={clip}>
              <rect x="2" y="2" width="60" height="60" rx="14" />
            </clipPath>
          </defs>
          <rect x="2" y="2" width="60" height="60" rx="14" fill="#0E1114" />
          <g clipPath={`url(#${clip})`}>
            <g transform="translate(32 33) scale(0.74) translate(-32 -32)">{art}</g>
          </g>
        </>
      ) : (
        art
      )}
    </svg>
  );
}

export function Wordmark() {
  return <span className="wordmark">AERO</span>;
}
