import { useCallback, useEffect, useRef, useState } from 'react';
import type { Direction, NetworkInterface, Notice, ServerStatus, TransferEvent } from '../types';
import { playWhoosh } from '../lib/sound';

export interface ActiveTransfer {
  id: string;
  name: string;
  size: number;
  direction: Direction;
  bytes: number;
  rate: number; // bytes per second, smoothed
  startedAt: number;
}

export interface HistoryItem {
  id: string;
  name: string;
  size: number;
  direction: Direction;
  ok: boolean;
  cancelled?: boolean;
  at: number;
}

export interface Toast extends Notice {
  key: number;
}

/** What the tunnel is doing, from the user's point of view. */
export type Phase = 'off' | 'waiting' | 'linked' | 'busy';

const HISTORY_KEY = 'aero.history.v2';
const SOUND_KEY = 'aero.sound';

function load<T>(key: string, fallback: T): T {
  try {
    const raw = localStorage.getItem(key);
    return raw ? (JSON.parse(raw) as T) : fallback;
  } catch {
    return fallback;
  }
}

function save(key: string, value: unknown) {
  try {
    localStorage.setItem(key, JSON.stringify(value));
  } catch {
    /* storage unavailable */
  }
}

const app = () => window.go?.main.App;

export function useAero() {
  const [interfaces, setInterfaces] = useState<NetworkInterface[]>([]);
  const [selected, setSelected] = useState<NetworkInterface | null>(null);
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [pending, setPending] = useState(false);
  const [phones, setPhones] = useState(0);
  const [active, setActive] = useState<ActiveTransfer[]>([]);
  const [history, setHistory] = useState<HistoryItem[]>(() => load(HISTORY_KEY, []));
  const [toasts, setToasts] = useState<Toast[]>([]);
  const [sound, setSound] = useState<boolean>(() => load(SOUND_KEY, true));

  const soundRef = useRef(sound);
  soundRef.current = sound;
  const samples = useRef(new Map<string, { t: number; bytes: number }>());
  const toastKey = useRef(0);

  const notify = useCallback((n: Notice) => {
    const key = ++toastKey.current;
    setToasts((t) => [...t.slice(-2), { ...n, key }]);
    setTimeout(() => setToasts((t) => t.filter((x) => x.key !== key)), n.kind === 'error' ? 5000 : 3200);
  }, []);

  useEffect(() => save(HISTORY_KEY, history.slice(0, 100)), [history]);
  useEffect(() => save(SOUND_KEY, sound), [sound]);

  const refreshInterfaces = useCallback(async () => {
    try {
      const list = (await app()?.GetLocalIPs()) ?? [];
      setInterfaces(list);
      setSelected((cur) => list.find((i) => i.ip === cur?.ip) ?? list[0] ?? null);
    } catch (e) {
      console.error(e);
    }
  }, []);

  useEffect(() => {
    refreshInterfaces();
    app()?.GetServerStatus().then((s) => s.running && setStatus(s)).catch(() => {});
  }, [refreshInterfaces]);

  const onProgress = useCallback((e: TransferEvent) => {
    const now = performance.now();
    if (e.status === 'started') {
      samples.current.set(e.id, { t: now, bytes: 0 });
      setActive((a) => [
        ...a.filter((x) => x.id !== e.id),
        { id: e.id, name: e.filename, size: e.size, direction: e.direction, bytes: 0, rate: 0, startedAt: Date.now() },
      ]);
      return;
    }
    if (e.status === 'progress') {
      const bytes = (e.progress / 100) * e.size;
      const prev = samples.current.get(e.id);
      setActive((a) =>
        a.map((x) => {
          if (x.id !== e.id) return x;
          let rate = x.rate;
          if (prev && now > prev.t) {
            const inst = ((bytes - prev.bytes) / (now - prev.t)) * 1000;
            rate = x.rate ? x.rate * 0.6 + inst * 0.4 : inst;
          }
          return { ...x, bytes, rate };
        }),
      );
      samples.current.set(e.id, { t: now, bytes });
      return;
    }
    // completed | cancelled | error
    samples.current.delete(e.id);
    setActive((a) => a.filter((x) => x.id !== e.id));
    const ok = e.status === 'completed';
    const cancelled = e.status === 'cancelled';
    setHistory((h) => [
      { id: e.id, name: e.filename, size: e.size, direction: e.direction, ok, cancelled, at: Date.now() },
      ...h.filter((x) => x.id !== e.id),
    ]);
    if (ok && soundRef.current) playWhoosh();
    if (ok && e.direction === 'receive') notify({ kind: 'success', message: `Landed in Downloads › Aero: ${e.filename}` });
    if (cancelled) notify({ kind: 'info', message: `Cancelled ${e.filename}. Nothing partial was kept.` });
    if (e.status === 'error') notify({ kind: 'error', message: `${e.filename} didn't make it. Nothing partial was kept, so just send it again.` });
  }, [notify]);

  useEffect(() => {
    const rt = window.runtime;
    if (!rt) return;
    const offs = [
      rt.EventsOn('server:started', (d) => {
        setStatus(d as ServerStatus);
        setPending(false);
      }),
      rt.EventsOn('server:stopped', () => {
        setStatus(null);
        setPending(false);
        setActive([]);
      }),
      rt.EventsOn('server:error', (d) => {
        setStatus(null);
        setPending(false);
        notify({ kind: 'error', message: `Aero stopped: ${(d as { error: string }).error}` });
      }),
      rt.EventsOn('phone:count', (d) => setPhones(d as number)),
      rt.EventsOn('transfer:progress', (d) => onProgress(d as TransferEvent)),
      rt.EventsOn('app:notice', (d) => notify(d as Notice)),
    ];
    return () => offs.forEach((off) => typeof off === 'function' && off());
  }, [notify, onProgress]);

  const start = useCallback(async () => {
    if (!selected || pending) return;
    setPending(true);
    try {
      await app()?.StartServer(selected.ip);
    } catch (e) {
      setPending(false);
      notify({ kind: 'error', message: String(e) });
    }
  }, [selected, pending, notify]);

  const stop = useCallback(async () => {
    setPending(true);
    try {
      await app()?.StopServer();
    } catch (e) {
      setPending(false);
      notify({ kind: 'error', message: String(e) });
    }
  }, [notify]);

  const chooseFiles = useCallback(async () => {
    try {
      await app()?.SendFileToPhone();
    } catch (e) {
      notify({ kind: 'error', message: String(e) });
    }
  }, [notify]);

  const cancel = useCallback(
    async (id: string) => {
      try {
        await app()?.CancelTransfer(id);
      } catch (e) {
        notify({ kind: 'info', message: String(e) });
      }
    },
    [notify],
  );

  const openFolder = useCallback(() => {
    app()?.OpenDownloadsFolder().catch((e) => notify({ kind: 'error', message: String(e) }));
  }, [notify]);

  const running = !!status?.running;
  const phase: Phase = !running ? 'off' : active.length > 0 ? 'busy' : phones > 0 ? 'linked' : 'waiting';

  return {
    interfaces,
    selected,
    setSelected,
    refreshInterfaces,
    status,
    pending,
    phones,
    phase,
    active,
    history,
    clearHistory: () => setHistory([]),
    toasts,
    sound,
    setSound,
    start,
    stop,
    chooseFiles,
    cancel,
    openFolder,
  };
}

export type Aero = ReturnType<typeof useAero>;
