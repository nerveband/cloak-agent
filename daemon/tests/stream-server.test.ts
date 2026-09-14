import { EventEmitter, once } from 'node:events';
import { describe, expect, it } from 'vitest';
import { WebSocket } from 'ws';

import type { BrowserManager } from '../src/browser.js';
import { StreamServer, isAllowedStreamHost, parseStreamMessage, streamMessageRejection } from '../src/stream-server.js';

class FakeSocket extends EventEmitter {
  readyState = WebSocket.OPEN;
  sent: string[] = [];
  closeCode?: number;
  closeReason?: string;

  send(payload: string): void {
    this.sent.push(payload);
  }

  close(code?: number, reason?: string): void {
    this.closeCode = code;
    this.closeReason = reason;
    this.readyState = WebSocket.CLOSED;
  }
}

function parseLast(socket: FakeSocket): Record<string, unknown> {
  const payload = socket.sent.at(-1);
  if (!payload) {
    throw new Error('Expected socket to receive a message.');
  }
  return JSON.parse(payload) as Record<string, unknown>;
}

function makeBrowser() {
  const calls: Array<{ method: string; payload?: unknown }> = [];
  let onFrame: ((data: string) => void) | null = null;
  let onUrl: ((url: string) => void) | null = null;

  const browser = {
    isLaunched: () => true,
    startScreencast: async (
      frameCallback: (data: string) => void,
      urlCallback?: (url: string) => void,
    ) => {
      calls.push({ method: 'startScreencast' });
      onFrame = frameCallback;
      onUrl = urlCallback ?? null;
    },
    stopScreencast: async () => {
      calls.push({ method: 'stopScreencast' });
    },
    injectMouseEvent: async (payload: unknown) => {
      calls.push({ method: 'injectMouseEvent', payload });
    },
    injectKeyboardEvent: async (payload: unknown) => {
      calls.push({ method: 'injectKeyboardEvent', payload });
    },
    injectTouchEvent: async (payload: unknown) => {
      calls.push({ method: 'injectTouchEvent', payload });
    },
  } as unknown as BrowserManager;

  return {
    browser,
    calls,
    emitFrame(data: string): void {
      if (!onFrame) {
        throw new Error('Screencast callback was not registered.');
      }
      onFrame(data);
    },
    emitUrl(url: string): void {
      if (!onUrl) {
        throw new Error('URL callback was not registered.');
      }
      onUrl(url);
    },
  };
}

describe('StreamServer', () => {
  it('starts on an assigned local port when constructed with port zero', async () => {
    const fake = makeBrowser();
    const server = new StreamServer(fake.browser, 0);

    const port = await server.start();
    server.stop();

    expect(port).toBeGreaterThan(0);
    expect(server.getPort()).toBe(port);
  });

  it('starts screencast and broadcasts frames to connected clients', async () => {
    const fake = makeBrowser();
    const server = new StreamServer(fake.browser, 0);
    const socket = new FakeSocket();

    server.handleConnection(socket as unknown as WebSocket);
    await Promise.resolve();

    expect(fake.calls.map((call) => call.method)).toContain('startScreencast');

    fake.emitFrame('jpeg-base64');

    const frame = parseLast(socket);
    expect(frame.type).toBe('frame');
    expect(frame.data).toBe('jpeg-base64');
    expect(frame.metadata).toEqual({ timestamp: expect.any(Number), url: null });
  });

  it('broadcasts main-frame URL updates and stamps them on frames', async () => {
    const fake = makeBrowser();
    const server = new StreamServer(fake.browser, 0);
    const socket = new FakeSocket();

    server.handleConnection(socket as unknown as WebSocket);
    await Promise.resolve();

    fake.emitUrl('https://example.com/one');
    expect(parseLast(socket)).toEqual({
      type: 'url',
      url: 'https://example.com/one',
      timestamp: expect.any(Number),
    });

    // Repeating the same URL would be noise for a viewer.
    const sentCount = socket.sent.length;
    fake.emitUrl('https://example.com/one');
    expect(socket.sent.length).toBe(sentCount);

    fake.emitUrl('https://example.com/two');
    fake.emitFrame('jpeg-base64');
    expect(parseLast(socket).metadata).toEqual({
      timestamp: expect.any(Number),
      url: 'https://example.com/two',
    });
  });

  it('rejects input messages that would reach CDP malformed', async () => {
    const fake = makeBrowser();
    const server = new StreamServer(fake.browser, 0);
    const socket = new FakeSocket();

    server.handleConnection(socket as unknown as WebSocket);
    socket.emit('message', JSON.stringify({ type: 'input_mouse', action: 'click', x: 'ten', y: 20 }));
    socket.emit('message', JSON.stringify({ type: 'input_keyboard', action: 'press', key: '' }));
    await Promise.resolve();

    expect(fake.calls.some((call) => call.method.startsWith('inject'))).toBe(false);
    expect(socket.sent.filter((payload) => payload.includes('Invalid'))).toHaveLength(2);
  });

  it('accepts only loopback Host headers on the websocket handshake', async () => {
    const server = new StreamServer(makeBrowser().browser, 0);
    const port = await server.start();

    const handshake = async (headers?: Record<string, string>): Promise<string> => {
      const client = new WebSocket(`ws://127.0.0.1:${port}`, headers ? { headers } : undefined);
      const outcome = await Promise.race([
        once(client, 'open').then(() => 'open'),
        once(client, 'error').then(() => 'rejected'),
        once(client, 'unexpected-response').then(() => 'rejected'),
      ]);
      client.close();
      return outcome;
    };

    expect(await handshake()).toBe('open');
    expect(await handshake({ Host: 'rebind.attacker.test' })).toBe('rejected');

    server.stop();
  });

  it('forwards client input messages to the browser manager', async () => {
    const fake = makeBrowser();
    const server = new StreamServer(fake.browser, 0);
    const socket = new FakeSocket();

    server.handleConnection(socket as unknown as WebSocket);
    socket.emit('message', JSON.stringify({ type: 'input_mouse', action: 'click', x: 10, y: 20 }));
    socket.emit('message', JSON.stringify({ type: 'input_keyboard', action: 'press', key: 'Enter' }));
    socket.emit('message', JSON.stringify({ type: 'input_touch', action: 'start', x: 5, y: 6 }));
    await Promise.resolve();

    expect(fake.calls).toContainEqual({
      method: 'injectMouseEvent',
      payload: { type: 'input_mouse', action: 'click', x: 10, y: 20 },
    });
    expect(fake.calls).toContainEqual({
      method: 'injectKeyboardEvent',
      payload: { type: 'input_keyboard', action: 'press', key: 'Enter' },
    });
    expect(fake.calls).toContainEqual({
      method: 'injectTouchEvent',
      payload: { type: 'input_touch', action: 'start', x: 5, y: 6 },
    });
  });

  it('stops screencast after the last client disconnects', async () => {
    const fake = makeBrowser();
    const server = new StreamServer(fake.browser, 0);
    const socket = new FakeSocket();

    server.handleConnection(socket as unknown as WebSocket);
    await Promise.resolve();
    socket.emit('close');
    await Promise.resolve();

    expect(fake.calls.map((call) => call.method)).toEqual(['startScreencast', 'stopScreencast']);
  });
});

describe('stream guards', () => {
  it('allows loopback hosts with the bound port only', () => {
    expect(isAllowedStreamHost('127.0.0.1:9222', 9222)).toBe(true);
    expect(isAllowedStreamHost('localhost:9222', 9222)).toBe(true);
    expect(isAllowedStreamHost('[::1]:9222', 9222)).toBe(true);
    expect(isAllowedStreamHost('127.0.0.1:9223', 9222)).toBe(false);
    expect(isAllowedStreamHost('rebind.attacker.test:9222', 9222)).toBe(false);
    expect(isAllowedStreamHost('127.0.0.1', 0)).toBe(true);
    expect(isAllowedStreamHost(undefined, 9222)).toBe(false);
  });

  it('parses valid input messages and labels rejected ones', () => {
    expect(parseStreamMessage({ type: 'input_mouse', action: 'move', x: 1, y: 2 })).not.toBeNull();
    expect(parseStreamMessage({ type: 'status' })).not.toBeNull();

    const badCoordinates = { type: 'input_mouse', action: 'move', x: Number.NaN, y: 2 };
    expect(parseStreamMessage(badCoordinates)).toBeNull();
    expect(streamMessageRejection(badCoordinates)).toBe('Invalid input_mouse message');

    const badKey = { type: 'input_keyboard', action: 'press', key: '' };
    expect(parseStreamMessage(badKey)).toBeNull();
    expect(streamMessageRejection(badKey)).toBe('Invalid input_keyboard message');

    const unknown = { type: 'input_foot' };
    expect(streamMessageRejection(unknown)).toBe('Unknown message type: input_foot');
    expect(streamMessageRejection('nope')).toBe('Invalid message format');
  });
});
