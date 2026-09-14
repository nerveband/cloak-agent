// StreamServer — WebSocket server for live viewport streaming (screencast via CDP).

import { WebSocketServer, WebSocket } from 'ws';
import type { IncomingMessage } from 'node:http';
import { z } from 'zod';
import type { BrowserManager } from './browser.js';
import type { StreamKeyboardInput, StreamMouseInput, StreamTouchInput } from './browser.js';

// ── Types ──────────────────────────────────────────────────────────────────────

interface StatusMessage {
  type: 'status';
  connected: boolean;
  screencasting: boolean;
  viewportWidth: number;
  viewportHeight: number;
  url: string | null;
}

interface ErrorMessage {
  type: 'error';
  message: string;
}

interface UrlMessage {
  type: 'url';
  url: string;
  timestamp: number;
}

interface FrameMessage {
  type: 'frame';
  data: string;
  metadata: { timestamp: number; url: string | null };
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

function unknownMessageType(message: unknown): string {
  if (typeof message === 'object' && message !== null && 'type' in message) {
    return String(message.type);
  }
  return 'unknown';
}

const LOOPBACK_HOST_NAMES = new Set(['127.0.0.1', 'localhost', '::1', '[::1]']);

const KNOWN_MESSAGE_TYPES: Record<string, true> = {
  input_mouse: true,
  input_keyboard: true,
  input_touch: true,
  status: true,
};

const streamMessageSchema = z.union([
  z.object({
    type: z.literal('input_mouse'),
    action: z.enum(['move', 'down', 'up', 'click']),
    x: z.number().finite(),
    y: z.number().finite(),
    button: z.enum(['left', 'right', 'middle']).optional(),
  }),
  z.object({
    type: z.literal('input_keyboard'),
    action: z.enum(['keydown', 'keyup', 'press']),
    key: z.string().min(1).max(64),
    modifiers: z.array(z.enum(['Alt', 'Control', 'Meta', 'Shift'])).max(4).optional(),
  }),
  z.object({
    type: z.literal('input_touch'),
    action: z.enum(['start', 'move', 'end', 'cancel']),
    x: z.number().finite(),
    y: z.number().finite(),
  }),
  z.object({ type: z.literal('status') }),
]);

/**
 * Accept only loopback Host headers. The listener already binds 127.0.0.1, but a
 * page can resolve an attacker-controlled name to 127.0.0.1 (DNS rebinding) and
 * reach a local server from a browser; its handshake then carries the attacker's
 * host, which a real local client never sends.
 */
export function isAllowedStreamHost(host: string | undefined, port: number): boolean {
  if (!host) {
    return false;
  }
  const normalized = host.trim().toLowerCase();
  const match = /^(\[[^\]]+\]|[^:]+)(?::(\d+))?$/.exec(normalized);
  if (!match) {
    return false;
  }
  if (!LOOPBACK_HOST_NAMES.has(match[1])) {
    return false;
  }
  const headerPort = match[2];
  if (port > 0 && headerPort !== undefined && Number(headerPort) !== port) {
    return false;
  }
  return true;
}

/**
 * Parse a client message. Anything that would reach CDP as NaN, an oversized
 * key, or an unknown enum is rejected here rather than forwarded.
 */
export function parseStreamMessage(payload: unknown): ClientMessage | null {
  const result = streamMessageSchema.safeParse(payload);
  return result.success ? result.data : null;
}

/** Human-readable reason for a rejected payload. */
export function streamMessageRejection(payload: unknown): string {
  if (typeof payload !== 'object' || payload === null || !('type' in payload)) {
    return 'Invalid message format';
  }
  const { type } = payload;
  if (typeof type !== 'string' || !KNOWN_MESSAGE_TYPES[type]) {
    return `Unknown message type: ${unknownMessageType(payload)}`;
  }
  return `Invalid ${type} message`;
}

// ── StreamServer ───────────────────────────────────────────────────────────────

export class StreamServer {
  private wss: WebSocketServer | null = null;
  private clients: Set<WebSocket> = new Set();
  private screencasting: boolean = false;
  private viewportWidth: number = 1920;
  private viewportHeight: number = 947;
  private currentUrl: string | null = null;
  private screencastOperations: Promise<void> = Promise.resolve();
  private stopping = false;

  constructor(
    private browser: BrowserManager,
    private port: number,
  ) {}

  // ── Lifecycle ──────────────────────────────────────────────────────────────

  async start(): Promise<number> {
    this.stopping = false;
    this.wss = new WebSocketServer({
      host: '127.0.0.1',
      port: this.port,
      maxPayload: 256 * 1024,
      verifyClient: (info: { origin: string; secure: boolean; req: IncomingMessage }) => {
        const origin = info.origin;
        // This endpoint is for native clients, not web pages or opaque origins.
        // File pages and sandboxed frames both serialize their origin as null.
        if (origin || info.req?.headers?.origin !== undefined) return false;
        // Same-origin provenance: a rebinding page reaches the loopback port
        // with its own Host header, which no local client sends.
        const host = info.req?.headers?.host;
        return isAllowedStreamHost(typeof host === 'string' ? host : undefined, this.port);
      },
    });

    this.wss.on('connection', (ws: WebSocket) => {
      this.handleConnection(ws);
    });

    await new Promise<void>((resolve, reject) => {
      this.wss?.once('listening', resolve);
      this.wss?.once('error', reject);
    });

    const address = this.wss.address();
    if (!address || typeof address === 'string') {
      return this.port;
    }
    this.port = address.port;
    return this.port;
  }

  async stop(): Promise<void> {
    this.stopping = true;
    for (const client of this.clients) client.terminate();
    this.clients.clear();
    try {
      await this.stopScreencast();
    } finally {
      const server = this.wss;
      this.wss = null;
      if (server) await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    }
  }

  // ── Connection handling ────────────────────────────────────────────────────

  handleConnection(ws: WebSocket): void {
    if (this.stopping) { ws.close(1001, 'Server shutting down'); return; }
    this.clients.add(ws);

    // Send current status to new client
    this.sendStatus(ws);

    // Start screencast if this is the first client
    if (this.clients.size === 1) {
      this.startScreencast().then(async () => {
        if (this.clients.size === 0) await this.stopScreencast();
      }).catch((err) => {
        this.sendError(ws, `Failed to start screencast: ${err instanceof Error ? err.message : String(err)}`);
      });
    }

    ws.on('message', (data: Buffer | string) => {
      try {
        const payload = JSON.parse(typeof data === 'string' ? data : data.toString('utf-8'));
        this.handleMessage(payload, ws);
      } catch {
        this.sendError(ws, 'Invalid message format');
      }
    });

    ws.on('close', () => {
      this.clients.delete(ws);

      // Stop screencast if no more clients
      if (this.clients.size === 0) {
        this.stopScreencast().catch(() => {});
      }
    });

    ws.on('error', () => {
      this.clients.delete(ws);
      if (this.clients.size === 0) void this.stopScreencast().catch(() => {});
    });
  }

  handleMessage(payload: unknown, ws: WebSocket): void {
    const message = parseStreamMessage(payload);
    if (!message) {
      this.sendError(ws, streamMessageRejection(payload));
      return;
    }

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
    const operation = this.screencastOperations.then(async () => {
      if (this.screencasting || this.stopping) return;
      await this.browser.startScreencast(data => this.broadcastFrame(data), url => this.broadcastUrl(url));
      this.screencasting = true;
    });
    this.screencastOperations = operation.catch(() => {});
    return operation;
  }

  async stopScreencast(): Promise<void> {
    const operation = this.screencastOperations.then(async () => {
      if (!this.screencasting) return;
      try { await this.browser.stopScreencast(); }
      finally { this.screencasting = false; this.currentUrl = null; }
    });
    this.screencastOperations = operation.catch(() => {});
    return operation;
  }

  // ── Broadcasting ───────────────────────────────────────────────────────────

  broadcastFrame(data: string): void {
    const message: FrameMessage = {
      type: 'frame',
      data,
      metadata: { timestamp: Date.now(), url: this.currentUrl },
    };
    this.sendToClients(JSON.stringify(message));
  }

  /** Emit the streamed page's main-frame URL for full and same-document moves. */
  broadcastUrl(url: string): void {
    if (url === this.currentUrl) {
      return;
    }
    this.currentUrl = url;
    const message: UrlMessage = {
      type: 'url',
      url,
      timestamp: Date.now(),
    };
    this.sendToClients(JSON.stringify(message));
  }

  private sendToClients(payload: string): void {
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
      url: this.currentUrl,
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
