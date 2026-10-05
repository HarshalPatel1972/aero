/**
 * Aero flow engine.
 *
 * Draws wind-tunnel smoke: streamlines computed from real potential-flow
 * solutions, with bright pulses that travel along them at the local air
 * speed. Two fields are provided:
 *
 *  - cylinderField: uniform flow around a circular body (the QR card).
 *  - ellipseField:  uniform flow around an ellipse (the mark in the story).
 *
 * Coordinates are CSS pixels with y pointing down.
 */

export type Vec = [number, number];

/** Returns the flow velocity at (x, y), or null inside the body. */
export type Field = (x: number, y: number) => Vec | null;

// ───────────────────────── Complex helpers ─────────────────────────

type C = [number, number];
const csub = (a: C, b: C): C => [a[0] - b[0], a[1] - b[1]];
const cmul = (a: C, b: C): C => [a[0] * b[0] - a[1] * b[1], a[0] * b[1] + a[1] * b[0]];
const cdiv = (a: C, b: C): C => {
  const d = b[0] * b[0] + b[1] * b[1];
  return [(a[0] * b[0] + a[1] * b[1]) / d, (a[1] * b[0] - a[0] * b[1]) / d];
};
const cabs = (a: C) => Math.hypot(a[0], a[1]);
const csqrt = (a: C): C => {
  const r = cabs(a);
  const re = Math.sqrt((r + a[0]) / 2);
  const im = Math.sign(a[1] || 1) * Math.sqrt(Math.max(0, (r - a[0]) / 2));
  return [re, im];
};

// ───────────────────────────── Fields ──────────────────────────────

/** Uniform left-to-right flow of speed 1 around a cylinder. */
export function cylinderField(cx: number, cy: number, radius: number): Field {
  const r2 = radius * radius;
  return (x, y) => {
    const dx = x - cx;
    const dy = y - cy;
    const d2 = dx * dx + dy * dy;
    if (d2 < r2) return null;
    const d4 = d2 * d2;
    // dW/dz = U(1 - R²/z²)  →  u = 1 - R²(dx²-dy²)/r⁴,  v = -2R²·dx·dy/r⁴
    // (screen y is flipped, which flips the sign of v back.)
    return [1 - (r2 * (dx * dx - dy * dy)) / d4, (-2 * r2 * dx * dy) / d4];
  };
}

/**
 * Uniform left-to-right flow around an ellipse with semi-axes rx, ry
 * (rx > ry), via the Joukowski map z = ζ + c²/ζ of a circle of radius a.
 */
export function ellipseField(cx: number, cy: number, rx: number, ry: number): Field {
  const a = (rx + ry) / 2;
  const c2 = (a * (rx - ry)) / 2;
  const a2 = a * a;
  return (x, y) => {
    const z: C = [x - cx, -(y - cy)];
    const s = csqrt(csub(cmul(z, z), [4 * c2, 0]));
    const r1: C = [(z[0] + s[0]) / 2, (z[1] + s[1]) / 2];
    const r2: C = [(z[0] - s[0]) / 2, (z[1] - s[1]) / 2];
    const zeta = cabs(r1) >= cabs(r2) ? r1 : r2;
    if (cabs(zeta) < a * 1.005) return null;
    const z2 = cmul(zeta, zeta);
    const dW = csub([1, 0], cdiv([a2, 0], z2));
    const dz = csub([1, 0], cdiv([c2, 0], z2));
    if (cabs(dz) < 1e-6) return null;
    const w = cdiv(dW, dz); // u − iv
    return [w[0], w[1]]; // screen y flips the sign of v
  };
}

// ─────────────────────────── Streamlines ───────────────────────────

export interface Streamline {
  pts: Float32Array; // x0,y0,x1,y1,… spaced `step` px apart
  speed: Float32Array; // local speed / free-stream speed at each point
  count: number;
}

/** Traces streamlines from evenly spaced seeds on the left edge. */
export function traceStreamlines(
  field: Field,
  width: number,
  height: number,
  { lines = 22, step = 3, margin = 0 } = {},
): Streamline[] {
  const out: Streamline[] = [];
  const maxPts = Math.ceil((width * 3) / step);
  for (let i = 0; i < lines; i++) {
    const y0 = margin + ((height - 2 * margin) * (i + 0.5)) / lines;
    const pts: number[] = [];
    const speed: number[] = [];
    let x = -8;
    let y = y0;
    for (let n = 0; n < maxPts && x < width + 8; n++) {
      const v1 = field(x, y);
      if (!v1) break;
      const m1 = Math.hypot(v1[0], v1[1]) || 1e-6;
      // Midpoint (RK2) step of fixed arc length.
      const xm = x + (v1[0] / m1) * step * 0.5;
      const ym = y + (v1[1] / m1) * step * 0.5;
      const v2 = field(xm, ym);
      if (!v2) break;
      const m2 = Math.hypot(v2[0], v2[1]) || 1e-6;
      pts.push(x, y);
      speed.push(Math.min(m1, 3));
      x += (v2[0] / m2) * step;
      y += (v2[1] / m2) * step;
    }
    if (pts.length >= 8) {
      out.push({ pts: Float32Array.from(pts), speed: Float32Array.from(speed), count: pts.length / 2 });
    }
  }
  return out;
}

// ──────────────────────────── Renderer ─────────────────────────────

export interface FlowStyle {
  /** Base streamline colour and opacity. */
  line: string;
  lineAlpha: number;
  /** Pulse colour. */
  pulse: string;
  pulseAlpha: number;
  /** Pulses per streamline. */
  pulses: number;
  /** Free-stream pulse speed in px/s; 0 freezes the flow. */
  speed: number;
  /** Pulse length in px. */
  tail: number;
}

interface Pulse {
  line: number;
  s: number; // position in points along the line
}

/** Animates pulses along precomputed streamlines on a canvas. */
export class FlowRenderer {
  private ctx: CanvasRenderingContext2D;
  private lines: Streamline[] = [];
  private pulses: Pulse[] = [];
  private base: HTMLCanvasElement | null = null;
  private raf = 0;
  private last = 0;
  private dpr = 1;
  private step = 3;
  style: FlowStyle;
  private target: FlowStyle;

  constructor(private canvas: HTMLCanvasElement, style: FlowStyle) {
    this.ctx = canvas.getContext('2d')!;
    this.style = { ...style };
    this.target = { ...style };
  }

  setLines(lines: Streamline[], width: number, height: number, step = 3) {
    this.lines = lines;
    this.step = step;
    this.dpr = Math.min(window.devicePixelRatio || 1, 2);
    this.canvas.width = Math.round(width * this.dpr);
    this.canvas.height = Math.round(height * this.dpr);
    this.canvas.style.width = `${width}px`;
    this.canvas.style.height = `${height}px`;
    this.base = null;
    this.reseed();
  }

  /** Smoothly transitions to a new style. */
  setStyle(style: FlowStyle) {
    const reseed = style.pulses !== this.target.pulses;
    this.target = { ...style };
    // Colours switch immediately; numbers ease in tick().
    this.style.line = style.line;
    this.style.pulse = style.pulse;
    this.style.tail = style.tail;
    this.style.pulses = style.pulses;
    this.base = null;
    if (reseed) this.reseed();
  }

  private reseed() {
    this.pulses = [];
    this.lines.forEach((l, i) => {
      for (let k = 0; k < this.target.pulses; k++) {
        this.pulses.push({ line: i, s: Math.random() * l.count });
      }
    });
  }

  start() {
    if (this.raf) return;
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const loop = (t: number) => {
      const dt = this.last ? Math.min((t - this.last) / 1000, 0.05) : 0;
      this.last = t;
      this.tick(reduced ? 0 : dt);
      this.draw(reduced);
      this.raf = requestAnimationFrame(loop);
    };
    this.raf = requestAnimationFrame(loop);
  }

  stop() {
    cancelAnimationFrame(this.raf);
    this.raf = 0;
    this.last = 0;
  }

  private tick(dt: number) {
    const ease = 1 - Math.exp(-dt * 3);
    const s = this.style;
    const t = this.target;
    s.speed += (t.speed - s.speed) * ease;
    s.lineAlpha += (t.lineAlpha - s.lineAlpha) * ease;
    s.pulseAlpha += (t.pulseAlpha - s.pulseAlpha) * ease;
    if (Math.abs(t.lineAlpha - s.lineAlpha) > 0.002) this.base = null;

    for (const p of this.pulses) {
      const l = this.lines[p.line];
      if (!l) continue;
      const i = Math.min(l.count - 1, Math.max(0, Math.floor(p.s)));
      p.s += (s.speed * dt * l.speed[i]) / this.step;
      if (p.s >= l.count) p.s -= l.count;
      if (p.s < 0) p.s += l.count; // negative speed: air streams right to left
    }
  }

  /** Caches the faint base streamlines, which only change with style. */
  private drawBase() {
    const c = document.createElement('canvas');
    c.width = this.canvas.width;
    c.height = this.canvas.height;
    const g = c.getContext('2d')!;
    g.scale(this.dpr, this.dpr);
    g.strokeStyle = this.style.line;
    g.globalAlpha = this.style.lineAlpha;
    g.lineWidth = 1;
    for (const l of this.lines) {
      g.beginPath();
      g.moveTo(l.pts[0], l.pts[1]);
      for (let i = 1; i < l.count; i++) g.lineTo(l.pts[i * 2], l.pts[i * 2 + 1]);
      g.stroke();
    }
    this.base = c;
  }

  private draw(reduced: boolean) {
    const g = this.ctx;
    g.setTransform(1, 0, 0, 1, 0, 0);
    g.clearRect(0, 0, this.canvas.width, this.canvas.height);
    if (!this.base) this.drawBase();
    if (this.base) g.drawImage(this.base, 0, 0);
    if (reduced || this.style.pulseAlpha < 0.01) return;

    g.scale(this.dpr, this.dpr);
    g.strokeStyle = this.style.pulse;
    g.lineCap = 'round';
    g.lineWidth = 1.6;
    const tailPts = Math.max(2, Math.round(this.style.tail / this.step));
    const segments = 4;
    for (const p of this.pulses) {
      const l = this.lines[p.line];
      if (!l) continue;
      const head = Math.floor(p.s);
      const dir = this.style.speed < 0 ? -1 : 1;
      // Draw the tail in a few segments that fade towards the back.
      for (let k = 0; k < segments; k++) {
        const a = head - dir * Math.round((tailPts * k) / segments);
        const b = head - dir * Math.round((tailPts * (k + 1)) / segments);
        if (a <= 0 || a >= l.count - 1) break;
        g.globalAlpha = this.style.pulseAlpha * (1 - k / segments);
        g.beginPath();
        g.moveTo(l.pts[a * 2], l.pts[a * 2 + 1]);
        const end = Math.min(l.count - 1, Math.max(0, b));
        for (let i = a - dir; dir > 0 ? i >= end : i <= end; i -= dir) g.lineTo(l.pts[i * 2], l.pts[i * 2 + 1]);
        g.stroke();
      }
    }
    g.globalAlpha = 1;
  }
}
