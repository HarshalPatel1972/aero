/**
 * ws.ts - WebSocket Client for Aero Hub
 * Term-Phase 5: Multi-Peer Presence
 * 
 * Handles connection to the Aero Hub for:
 *   - Peer presence (join/leave notifications)
 *   - Peer list updates
 *   - Relay signaling
 */

// ═══════════════════════════════════════════════════════════════
// TYPES
// ═══════════════════════════════════════════════════════════════

export interface PeerInfo {
  id: string;
  name: string;
  isHost: boolean;
}

export interface HubEvent {
  event: string;
  data: unknown;
}

export interface WelcomeData {
  you: PeerInfo;
  peers: PeerInfo[];
}

export interface RelayData {
  from: PeerInfo;
  payload: unknown;
}

export interface IncomingRelayData {
  filename: string;
  size: string;
  url: string;
}

// ═══════════════════════════════════════════════════════════════
// EVENT HANDLERS TYPE
// ═══════════════════════════════════════════════════════════════

export interface HubEventHandlers {
  onWelcome?: (data: WelcomeData) => void;
  onPeerJoined?: (peer: PeerInfo) => void;
  onPeerLeft?: (peer: PeerInfo) => void;
  onPeerList?: (peers: PeerInfo[]) => void;
  onIncomingRelay?: (data: IncomingRelayData) => void;
  onRelayFrom?: (data: RelayData) => void;
  onDisconnect?: () => void;
  onError?: (error: Event) => void;
}

// ═══════════════════════════════════════════════════════════════
// HUB CLIENT
// ═══════════════════════════════════════════════════════════════

export class HubClient {
  private ws: WebSocket | null = null;
  private handlers: HubEventHandlers = {};
  private myInfo: PeerInfo | null = null;
  private peers: Map<string, PeerInfo> = new Map();
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;

  constructor(private url: string) {}

  /**
   * Connect to the Aero Hub
   */
  connect(handlers: HubEventHandlers = {}): Promise<WelcomeData> {
    this.handlers = handlers;

    return new Promise((resolve, reject) => {
      try {
        this.ws = new WebSocket(this.url);

        this.ws.onopen = () => {
          console.log('[HUB] ✅ Connected');
          this.reconnectAttempts = 0;
        };

        this.ws.onmessage = (event) => {
          try {
            const msg: HubEvent = JSON.parse(event.data);
            this.handleEvent(msg, resolve);
          } catch (e) {
            console.error('[HUB] Failed to parse message:', e);
          }
        };

        this.ws.onerror = (error) => {
          console.error('[HUB] ❌ Error:', error);
          this.handlers.onError?.(error);
          reject(error);
        };

        this.ws.onclose = () => {
          console.log('[HUB] 👋 Disconnected');
          this.handlers.onDisconnect?.();
          this.attemptReconnect();
        };

      } catch (error) {
        reject(error);
      }
    });
  }

  /**
   * Handle incoming hub events
   */
  private handleEvent(msg: HubEvent, welcomeResolve?: (data: WelcomeData) => void) {
    switch (msg.event) {
      case 'welcome': {
        const data = msg.data as WelcomeData;
        this.myInfo = data.you;
        this.peers.clear();
        data.peers.forEach(p => this.peers.set(p.id, p));
        this.handlers.onWelcome?.(data);
        welcomeResolve?.(data);
        break;
      }

      case 'peer_joined': {
        const peer = msg.data as PeerInfo;
        this.peers.set(peer.id, peer);
        this.handlers.onPeerJoined?.(peer);
        break;
      }

      case 'peer_left': {
        const peer = msg.data as PeerInfo;
        this.peers.delete(peer.id);
        this.handlers.onPeerLeft?.(peer);
        break;
      }

      case 'peer_list': {
        const peers = msg.data as PeerInfo[];
        this.peers.clear();
        peers.forEach(p => this.peers.set(p.id, p));
        this.handlers.onPeerList?.(peers);
        break;
      }

      case 'incoming_relay': {
        const data = msg.data as IncomingRelayData;
        this.handlers.onIncomingRelay?.(data);
        break;
      }

      case 'relay_from': {
        const data = msg.data as RelayData;
        this.handlers.onRelayFrom?.(data);
        break;
      }
    }
  }

  /**
   * Attempt to reconnect on disconnect
   */
  private attemptReconnect() {
    if (this.reconnectAttempts >= this.maxReconnectAttempts) {
      console.log('[HUB] Max reconnect attempts reached');
      return;
    }

    this.reconnectAttempts++;
    const delay = Math.min(1000 * Math.pow(2, this.reconnectAttempts), 10000);
    
    console.log(`[HUB] Reconnecting in ${delay}ms (attempt ${this.reconnectAttempts})`);
    
    setTimeout(() => {
      this.connect(this.handlers).catch(() => {});
    }, delay);
  }

  /**
   * Request peer list
   */
  requestPeers() {
    this.send({ action: 'get_peers' });
  }

  /**
   * Send a message to another peer via relay
   */
  relayTo(targetId: string, payload: unknown) {
    this.send({ action: 'relay_to', target: targetId, payload });
  }

  /**
   * Send raw message
   */
  send(data: unknown) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(data));
    }
  }

  /**
   * Disconnect from hub
   */
  disconnect() {
    this.maxReconnectAttempts = 0; // Prevent reconnect
    this.ws?.close();
    this.ws = null;
  }

  // Getters
  get me(): PeerInfo | null {
    return this.myInfo;
  }

  get peerList(): PeerInfo[] {
    return Array.from(this.peers.values());
  }

  get isConnected(): boolean {
    return this.ws?.readyState === WebSocket.OPEN;
  }
}

// ═══════════════════════════════════════════════════════════════
// FACTORY
// ═══════════════════════════════════════════════════════════════

/**
 * Create a HubClient connected to the current host
 */
export function createHubClient(): HubClient {
  const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const url = `${protocol}//${window.location.host}/hub`;
  return new HubClient(url);
}
