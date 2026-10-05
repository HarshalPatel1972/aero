// Type definitions for the Wails runtime and App bindings.

export interface NetworkInterface {
  name: string;
  ip: string;
}

export interface ServerStatus {
  running: boolean;
  url: string;
  ip: string;
  port: string;
}

export type TransferStatus = 'started' | 'progress' | 'completed' | 'cancelled' | 'error';
export type Direction = 'send' | 'receive';

/** Emitted by the Go server as "transfer:progress". */
export interface TransferEvent {
  id: string;
  filename: string;
  size: number;
  progress: number;
  speed: string;
  status: TransferStatus;
  direction: Direction;
}

export interface Notice {
  kind: 'info' | 'error' | 'success';
  message: string;
}

declare global {
  interface Window {
    runtime: {
      EventsOn: (eventName: string, callback: (data: unknown) => void) => () => void;
      EventsOff: (eventName: string) => void;
      WindowMinimise: () => void;
      Quit: () => void;
    };
    go: {
      main: {
        App: {
          GetLocalIPs: () => Promise<NetworkInterface[]>;
          StartServer: (ip: string) => Promise<void>;
          StopServer: () => Promise<void>;
          GetServerStatus: () => Promise<ServerStatus>;
          OpenDownloadsFolder: () => Promise<void>;
          SendFileToPhone: () => Promise<void>;
          SendFiles: (paths: string[]) => Promise<void>;
          CancelTransfer: (id: string) => Promise<void>;
          IsPhoneConnected: () => Promise<boolean>;
          SetMiniMode: (enabled: boolean) => Promise<void>;
        };
      };
    };
  }
}

export {};
