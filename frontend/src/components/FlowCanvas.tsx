import { useEffect, useRef } from 'react';
import { FlowRenderer, traceStreamlines, type Field, type FlowStyle } from '../lib/flow';

interface Props {
  width: number;
  height: number;
  /** Builds the flow field for the canvas size. Must be stable (memoised). */
  field: (w: number, h: number) => Field;
  style: FlowStyle;
  lines?: number;
  className?: string;
}

/** A canvas of animated streamlines. Pauses while the window is hidden. */
export function FlowCanvas({ width, height, field, style, lines = 24, className }: Props) {
  const ref = useRef<HTMLCanvasElement>(null);
  const renderer = useRef<FlowRenderer | null>(null);

  useEffect(() => {
    const canvas = ref.current;
    if (!canvas) return;
    const r = new FlowRenderer(canvas, style);
    renderer.current = r;
    r.setLines(traceStreamlines(field(width, height), width, height, { lines }), width, height);
    r.start();

    const onVisibility = () => (document.hidden ? r.stop() : r.start());
    document.addEventListener('visibilitychange', onVisibility);
    return () => {
      document.removeEventListener('visibilitychange', onVisibility);
      r.stop();
      renderer.current = null;
    };
    // style is applied separately below so state changes ease instead of restarting
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [width, height, field, lines]);

  useEffect(() => {
    renderer.current?.setStyle(style);
  }, [style]);

  return <canvas ref={ref} className={className} aria-hidden="true" />;
}
