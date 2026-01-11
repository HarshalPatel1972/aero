// Type definitions for Wails runtime and App bindings
// Strict TypeScript - no 'any' types

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

export interface TransferEvent {
  filename: string;
  progress: number;
  speed: string;
  status: 'started' | 'progress' | 'completed' | 'error';
}

// Wails runtime types
declare global {
  interface Window {
    runtime: {
      EventsOn: (eventName: string, callback: (data: unknown) => void) => void;
      EventsOff: (eventName: string) => void;
      WindowMinimise: () => void;
      WindowClose: () => void;
      WindowSetAlwaysOnTop: (alwaysOnTop: boolean) => void;
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
          IsPhoneConnected: () => Promise<boolean>;
        };
      };
    };
  }
}

export {};
