// StreamServer — WebSocket server for live viewport streaming (screencast via CDP).

import { WebSocketServer, WebSocket } from 'ws';
import type { IncomingMessage } from 'node:http';
import type { BrowserManager } from './browser.js';
import type { StreamKeyboardInput, StreamMouseInput, StreamTouchInput } from './browser.js';

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

interface InputMouseMessage extends StreamMouseInput {
  type: 'input_mouse';
}

interface InputKeyboardMessage extends StreamKeyboardInput {
  type: 'input_keyboard';
}

interface InputTouchMessage extends StreamTouchInput {
  type: 'input_touch';
}

type ClientMessage =
  | InputMouseMessage
  | InputKeyboardMessage
  | InputTouchMessage
  | { type: 'status' };

function unknownMessageType(message: never): string {
  return typeof message === 'object' && message !== null && 'type' in message
    ? String((message as { type: unknown }).type)
    : 'unknown';
}

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

  async start(): Promise<number> {
    this.wss = new WebSocketServer({
      host: '127.0.0.1',
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

    await new Promise<void>((resolve) => {
      this.wss?.once('listening', () => resolve());
    });

    const address = this.wss.address();
    if (!address || typeof address === 'string') {
      return this.port;
    }
    this.port = address.port;
    return this.port;
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
        this.sendError(ws, `Unknown message type: ${unknownMessageType(message)}`);
    }
  }

  // ── Input handlers ─────────────────────────────────────────────────────────

  private handleMouseInput(message: InputMouseMessage): void {
    this.browser.injectMouseEvent(message).catch((err) => this.sendAllErrors(err));
  }

  private handleKeyboardInput(message: InputKeyboardMessage): void {
    this.browser.injectKeyboardEvent(message).catch((err) => this.sendAllErrors(err));
  }

  private handleTouchInput(message: InputTouchMessage): void {
    this.browser.injectTouchEvent(message).catch((err) => this.sendAllErrors(err));
  }

  // ── Screencast ─────────────────────────────────────────────────────────────

  async startScreencast(): Promise<void> {
    if (this.screencasting) return;

    await this.browser.startScreencast((data) => this.broadcastFrame(data));
    this.screencasting = true;
  }

  async stopScreencast(): Promise<void> {
    if (!this.screencasting) return;

    await this.browser.stopScreencast();
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

  private sendAllErrors(error: unknown): void {
    const message = error instanceof Error ? error.message : String(error);
    for (const client of this.clients) {
      this.sendError(client, message);
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
