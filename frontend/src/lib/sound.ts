let ctx: AudioContext | null = null;

/**
 * A short synthesized "whoosh": band-passed noise swept upward, like air
 * rushing past. Played when a file lands.
 */
export function playWhoosh() {
  try {
    ctx ??= new AudioContext();
    const duration = 0.7;
    const buffer = ctx.createBuffer(1, Math.floor(ctx.sampleRate * duration), ctx.sampleRate);
    const data = buffer.getChannelData(0);
    for (let i = 0; i < data.length; i++) data[i] = Math.random() * 2 - 1;

    const noise = ctx.createBufferSource();
    noise.buffer = buffer;
    const band = ctx.createBiquadFilter();
    band.type = 'bandpass';
    band.Q.value = 1.4;
    const gain = ctx.createGain();
    const t = ctx.currentTime;
    band.frequency.setValueAtTime(380, t);
    band.frequency.exponentialRampToValueAtTime(2600, t + duration * 0.8);
    gain.gain.setValueAtTime(0.0001, t);
    gain.gain.exponentialRampToValueAtTime(0.16, t + 0.12);
    gain.gain.exponentialRampToValueAtTime(0.0001, t + duration);

    noise.connect(band).connect(gain).connect(ctx.destination);
    noise.start(t);
    noise.stop(t + duration);
  } catch {
    /* audio unavailable */
  }
}
