/**
 * audio.ts - AERO Sound Engine
 * Term-Phase 9: "Glass & Air" Aesthetic
 *
 * Uses Web Audio API for real-time sound synthesis.
 * No external files - sounds are generated mathematically.
 * All sounds are subtle (-10dB) and crisp.
 *
 * Sound Palette:
 *   - "link": Peer connects (soft pop)
 *   - "ignite": Transfer starts (rising whoosh)
 *   - "land": Success (glassy chord)
 *   - "error": Failure (low thud)
 *   - "tick": UI interaction (tiny click)
 */

// ═══════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════

type SoundName = 'link' | 'ignite' | 'land' | 'error' | 'tick';

interface SoundEngineConfig {
  masterVolume: number; // 0-1
  muted: boolean;
  respectDND: boolean;
}

// ═══════════════════════════════════════════════════════════════
// SOUND ENGINE
// ═══════════════════════════════════════════════════════════════

class SoundEngine {
  private ctx: AudioContext | null = null;
  private masterGain: GainNode | null = null;
  private config: SoundEngineConfig = {
    masterVolume: 0.3, // -10dB equivalent
    muted: false,
    respectDND: true,
  };

  /**
   * Initialize the audio context.
   * Must be called after user interaction (browser policy).
   */
  init(): boolean {
    if (this.ctx) return true;

    try {
      this.ctx = new (window.AudioContext || (window as any).webkitAudioContext)();
      this.masterGain = this.ctx.createGain();
      this.masterGain.gain.value = this.config.masterVolume;
      this.masterGain.connect(this.ctx.destination);
      console.log('[AUDIO] ✅ Sound engine initialized');
      return true;
    } catch (e) {
      console.warn('[AUDIO] ⚠️ Web Audio not available');
      return false;
    }
  }

  /**
   * Play a sound by name.
   */
  play(name: SoundName): void {
    if (!this.shouldPlay()) return;
    if (!this.ctx) this.init();
    if (!this.ctx || !this.masterGain) return;

    switch (name) {
      case 'link':
        this.playPop();
        break;
      case 'ignite':
        this.playWhoosh();
        break;
      case 'land':
        this.playChord();
        break;
      case 'error':
        this.playThud();
        break;
      case 'tick':
        this.playTick();
        break;
    }
  }

  /**
   * Check if sound should play (respects mute, DND).
   */
  private shouldPlay(): boolean {
    if (this.config.muted) return false;

    // Check document visibility
    if (document.hidden && this.config.respectDND) return false;

    return true;
  }

  // ═══════════════════════════════════════════════════════════
  // SOUND GENERATORS
  // ═══════════════════════════════════════════════════════════

  /**
   * "Link" - Soft high-pitched pop (peer connects)
   * Sine wave with quick attack and decay
   */
  private playPop(): void {
    const osc = this.ctx!.createOscillator();
    const gain = this.ctx!.createGain();
    const now = this.ctx!.currentTime;

    osc.type = 'sine';
    osc.frequency.setValueAtTime(880, now); // A5
    osc.frequency.exponentialRampToValueAtTime(1760, now + 0.05); // A6

    gain.gain.setValueAtTime(0, now);
    gain.gain.linearRampToValueAtTime(0.4, now + 0.01);
    gain.gain.exponentialRampToValueAtTime(0.001, now + 0.15);

    osc.connect(gain);
    gain.connect(this.masterGain!);

    osc.start(now);
    osc.stop(now + 0.15);
  }

  /**
   * "Ignite" - Rising whoosh (transfer starts)
   * Filtered white noise with frequency sweep
   */
  private playWhoosh(): void {
    const bufferSize = this.ctx!.sampleRate * 0.3;
    const buffer = this.ctx!.createBuffer(1, bufferSize, this.ctx!.sampleRate);
    const data = buffer.getChannelData(0);

    // Generate noise
    for (let i = 0; i < bufferSize; i++) {
      data[i] = Math.random() * 2 - 1;
    }

    const noise = this.ctx!.createBufferSource();
    noise.buffer = buffer;

    const filter = this.ctx!.createBiquadFilter();
    filter.type = 'bandpass';
    filter.Q.value = 2;

    const gain = this.ctx!.createGain();
    const now = this.ctx!.currentTime;

    // Sweep filter frequency up
    filter.frequency.setValueAtTime(200, now);
    filter.frequency.exponentialRampToValueAtTime(4000, now + 0.25);

    gain.gain.setValueAtTime(0, now);
    gain.gain.linearRampToValueAtTime(0.3, now + 0.05);
    gain.gain.exponentialRampToValueAtTime(0.001, now + 0.3);

    noise.connect(filter);
    filter.connect(gain);
    gain.connect(this.masterGain!);

    noise.start(now);
    noise.stop(now + 0.3);
  }

  /**
   * "Land" - Glassy chord (success)
   * Major 7th chord with crystal texture
   */
  private playChord(): void {
    const now = this.ctx!.currentTime;
    const frequencies = [523.25, 659.25, 783.99, 987.77]; // C5 E5 G5 B5 (Cmaj7)

    frequencies.forEach((freq, i) => {
      const osc = this.ctx!.createOscillator();
      const gain = this.ctx!.createGain();

      // Mix of sine and triangle for "crystal" texture
      osc.type = i % 2 === 0 ? 'sine' : 'triangle';
      osc.frequency.value = freq;

      const delay = i * 0.02; // Slight arpeggio
      gain.gain.setValueAtTime(0, now + delay);
      gain.gain.linearRampToValueAtTime(0.15, now + delay + 0.02);
      gain.gain.exponentialRampToValueAtTime(0.001, now + delay + 0.5);

      osc.connect(gain);
      gain.connect(this.masterGain!);

      osc.start(now + delay);
      osc.stop(now + delay + 0.5);
    });
  }

  /**
   * "Error" - Low dull thud
   * Low frequency sine with quick decay
   */
  private playThud(): void {
    const osc = this.ctx!.createOscillator();
    const gain = this.ctx!.createGain();
    const now = this.ctx!.currentTime;

    osc.type = 'sine';
    osc.frequency.setValueAtTime(80, now);
    osc.frequency.exponentialRampToValueAtTime(40, now + 0.15);

    gain.gain.setValueAtTime(0.5, now);
    gain.gain.exponentialRampToValueAtTime(0.001, now + 0.2);

    osc.connect(gain);
    gain.connect(this.masterGain!);

    osc.start(now);
    osc.stop(now + 0.2);
  }

  /**
   * "Tick" - Tiny click for UI interactions
   */
  private playTick(): void {
    const osc = this.ctx!.createOscillator();
    const gain = this.ctx!.createGain();
    const now = this.ctx!.currentTime;

    osc.type = 'sine';
    osc.frequency.value = 1200;

    gain.gain.setValueAtTime(0.2, now);
    gain.gain.exponentialRampToValueAtTime(0.001, now + 0.03);

    osc.connect(gain);
    gain.connect(this.masterGain!);

    osc.start(now);
    osc.stop(now + 0.03);
  }

  // ═══════════════════════════════════════════════════════════
  // CONTROLS
  // ═══════════════════════════════════════════════════════════

  mute(): void {
    this.config.muted = true;
    if (this.masterGain) {
      this.masterGain.gain.value = 0;
    }
  }

  unmute(): void {
    this.config.muted = false;
    if (this.masterGain) {
      this.masterGain.gain.value = this.config.masterVolume;
    }
  }

  toggle(): boolean {
    if (this.config.muted) {
      this.unmute();
    } else {
      this.mute();
    }
    return !this.config.muted;
  }

  setVolume(v: number): void {
    this.config.masterVolume = Math.max(0, Math.min(1, v));
    if (this.masterGain && !this.config.muted) {
      this.masterGain.gain.value = this.config.masterVolume;
    }
  }

  get isMuted(): boolean {
    return this.config.muted;
  }
}

// ═══════════════════════════════════════════════════════════════
// SINGLETON EXPORT
// ═══════════════════════════════════════════════════════════════

export const soundEngine = new SoundEngine();

// Initialize on first user interaction
if (typeof document !== 'undefined') {
  const initOnInteraction = () => {
    soundEngine.init();
    document.removeEventListener('click', initOnInteraction);
    document.removeEventListener('touchstart', initOnInteraction);
  };
  document.addEventListener('click', initOnInteraction, { once: true });
  document.addEventListener('touchstart', initOnInteraction, { once: true });
}

export default soundEngine;
