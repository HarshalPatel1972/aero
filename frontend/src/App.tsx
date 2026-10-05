/**
 * Aero desktop app.
 *
 * Named after aerodynamics: the window is a wind tunnel's test section, the
 * QR code is the body in the flow, and files travel as the airstream.
 */

import { useCallback, useState } from 'react';
import { useAero } from './hooks/useAero';
import { Tunnel } from './components/Tunnel';
import { Deck } from './components/Deck';
import { Story } from './components/Story';
import { MiniBar } from './components/MiniBar';
import { Footer, HistorySheet, TitleBar, Toasts } from './components/Chrome';

const STORY_KEY = 'aero.story.seen';

function storySeen() {
  try {
    return localStorage.getItem(STORY_KEY) === '1';
  } catch {
    return false;
  }
}

export default function App() {
  const aero = useAero();
  const [story, setStory] = useState(() => !storySeen());
  const [history, setHistory] = useState(false);
  const [mini, setMini] = useState(false);

  const closeStory = useCallback(() => {
    setStory(false);
    try {
      localStorage.setItem(STORY_KEY, '1');
    } catch {
      /* storage unavailable */
    }
  }, []);

  const setMode = useCallback(async (on: boolean) => {
    setMini(on);
    try {
      await window.go?.main.App.SetMiniMode(on);
    } catch (e) {
      console.error(e);
    }
  }, []);

  if (mini) {
    return <MiniBar aero={aero} onExpand={() => setMode(false)} />;
  }

  return (
    <div className="shell">
      <TitleBar onStory={() => setStory(true)} onHistory={() => setHistory(true)} />
      <Tunnel aero={aero} />
      <Deck aero={aero} />
      <Footer aero={aero} onMini={() => setMode(true)} />
      <Toasts toasts={aero.toasts} />
      {history && <HistorySheet aero={aero} onClose={() => setHistory(false)} />}
      {story && <Story onDone={closeStory} />}
    </div>
  );
}
