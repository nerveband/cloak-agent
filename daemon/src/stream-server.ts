// StreamServer — WebSocket server for live viewport streaming (screencast via CDP).

import { WebSocketServer, WebSocket } from 'ws';
import type { IncomingMessage } from 'node:http';
import type { BrowserManager } from './browser.js';

// ── Types ──────────────────────────────────────────────────────────────────────

interface StatusMessage {
  type: 'status';
  connected: boolean;
  screencasting: boolean;
  viewportWidth: number;
  viewportHeight: number;
}

interface ErrorMessage {
  type: 'error';
  message: string;
}

interface FrameMessage {
  type: 'frame';
  data: string;
  metadata?: { timestamp: number };
}

interface InputMouseMessage {
  type: 'input_mouse';
  action: 'move' | 'down' | 'up' | 'click';
  x: number;
  y: number;
  button?: 'left' | 'right' | 'middle';
}

interface InputKeyboardMessage {
  type: 'input_keyboard';
  action: 'keydown' | 'keyup' | 'press';
  key: string;
  modifiers?: string[];
}

interface InputTouchMessage {
  type: 'input_touch';
  action: 'start' | 'move' | 'end' | 'cancel';
  x: number;
  y: number;
}

type ClientMessage =
  | InputMouseMessage
  | InputKeyboardMessage
  | InputTouchMessage
  | { type: 'status' };

// ── StreamServer ───────────────────────────────────────────────────────────────

export class StreamServer {
  private wss: WebSocketServer | null = null;
  private clients: Set<WebSocket> = new Set();
  private screencasting: boolean = false;
  private viewportWidth: number = 1920;
  private viewportHeight: number = 947;

  constructor(
    private browser: BrowserManager,
    private port: number,
  ) {}

  // ── Lifecycle ──────────────────────────────────────────────────────────────

  start(): void {
    this.wss = new WebSocketServer({
      port: this.port,
      verifyClient: (info: { origin: string; secure: boolean; req: IncomingMessage }) => {
        const origin = info.origin;
        // Reject connections from browser origins (http:// or https://)
        // Only allow no-origin (empty/undefined) or file:// origins
        if (origin && origin.startsWith('http://')) return false;
        if (origin && origin.startsWith('https://')) return false;
        return true;
      },
    });

    this.wss.on('connection', (ws: WebSocket) => {
      this.handleConnection(ws);
    });
  }

  stop(): void {
    // Close all clients
    for (const client of this.clients) {
      try {
        client.close(1001, 'Server shutting down');
      } catch {
        // ignore
      }
    }
    this.clients.clear();

    // Stop screencasting
    if (this.screencasting) {
      this.stopScreencast().catch(() => {});
    }

    // Close the server
    if (this.wss) {
      this.wss.close();
      this.wss = null;
    }
  }

  // ── Connection handling ────────────────────────────────────────────────────

  handleConnection(ws: WebSocket): void {
    this.clients.add(ws);

    // Send current status to new client
    this.sendStatus(ws);

    // Start screencast if this is the first client
    if (this.clients.size === 1) {
      this.startScreencast().catch((err) => {
        this.sendError(ws, `Failed to start screencast: ${err instanceof Error ? err.message : String(err)}`);
      });
    }

    ws.on('message', (data: Buffer | string) => {
      try {
        const message = JSON.parse(typeof data === 'string' ? data : data.toString('utf-8'));
        this.handleMessage(message as ClientMessage, ws);
      } catch {
        this.sendError(ws, 'Invalid message format');
      }
    });

    ws.on('close', () => {
      this.clients.delete(ws);

      // Stop screencast if no more clients
      if (this.clients.size === 0 && this.screencasting) {
        this.stopScreencast().catch(() => {});
      }
    });

    ws.on('error', () => {
      this.clients.delete(ws);
    });
  }

  handleMessage(message: ClientMessage, ws: WebSocket): void {
    switch (message.type) {
      case 'input_mouse':
        this.handleMouseInput(message);
        break;
      case 'input_keyboard':
        this.handleKeyboardInput(message);
        break;
      case 'input_touch':
        this.handleTouchInput(message);
        break;
      case 'status':
        this.sendStatus(ws);
        break;
      default:
        this.sendError(ws, `Unknown message type: ${(message as any).type}`);
    }
  }

  // ── Input handlers ─────────────────────────────────────────────────────────

  private handleMouseInput(message: InputMouseMessage): void {
    // TODO: Requires BrowserManager.injectMouseEvent(message) — needs CDP session support
    // Once BrowserManager has CDP session access, call:
    // this.browser.injectMouseEvent(message);
  }

  private handleKeyboardInput(message: InputKeyboardMessage): void {
    // TODO: Requires BrowserManager.injectKeyboardEvent(message) — needs CDP session support
    // Once BrowserManager has CDP session access, call:
    // this.browser.injectKeyboardEvent(message);
  }

  private handleTouchInput(message: InputTouchMessage): void {
    // TODO: Requires BrowserManager.injectTouchEvent(message) — needs CDP session support
    // Once BrowserManager has CDP session access, call:
    // this.browser.injectTouchEvent(message);
  }

  // ── Screencast ─────────────────────────────────────────────────────────────

  async startScreencast(): Promise<void> {
    if (this.screencasting) return;

    // TODO: Requires BrowserManager.startScreencast(callback) — needs CDP session support
    // Once BrowserManager has CDP session access:
    //   const cdpSession = await this.browser.getPage().context().newCDPSession(this.browser.getPage());
    //   cdpSession.on('Page.screencastFrame', (params) => {
    //     this.broadcastFrame(params.data);
    //     cdpSession.send('Page.screencastFrameAck', { sessionId: params.sessionId });
    //   });
    //   await cdpSession.send('Page.startScreencast', {
    //     format: 'jpeg', quality: 60,
    //     maxWidth: this.viewportWidth, maxHeight: this.viewportHeight,
    //   });

    this.screencasting = true;
  }

  async stopScreencast(): Promise<void> {
    if (!this.screencasting) return;

    // TODO: Requires BrowserManager.stopScreencast() — needs CDP session support
    // Once BrowserManager has CDP session access:
    //   await cdpSession.send('Page.stopScreencast');

    this.screencasting = false;
  }

  // ── Broadcasting ───────────────────────────────────────────────────────────

  broadcastFrame(data: string): void {
    const message: FrameMessage = {
      type: 'frame',
      data,
      metadata: { timestamp: Date.now() },
    };
    const payload = JSON.stringify(message);

    for (const client of this.clients) {
      if (client.readyState === WebSocket.OPEN) {
        client.send(payload);
      }
    }
  }

  // ── Status / Error ─────────────────────────────────────────────────────────

  sendStatus(ws: WebSocket): void {
    const message: StatusMessage = {
      type: 'status',
      connected: this.browser.isLaunched(),
      screencasting: this.screencasting,
      viewportWidth: this.viewportWidth,
      viewportHeight: this.viewportHeight,
    };

    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(message));
    }
  }

  sendError(ws: WebSocket, message: string): void {
    const errorMsg: ErrorMessage = {
      type: 'error',
      message,
    };

    if (ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(errorMsg));
    }
  }

  // ── Accessors ──────────────────────────────────────────────────────────────

  getPort(): number {
    return this.port;
  }

  getClientCount(): number {
    return this.clients.size;
  }
}
