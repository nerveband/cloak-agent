import { EventEmitter } from 'node:events';
import { describe, expect, it } from 'vitest';
import { WebSocket } from 'ws';

import type { BrowserManager } from '../src/browser.js';
import { StreamServer } from '../src/stream-server.js';

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

  const browser = {
    isLaunched: () => true,
    startScreencast: async (callback: (data: string) => void) => {
      calls.push({ method: 'startScreencast' });
      onFrame = callback;
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
    expect(frame.metadata).toEqual({ timestamp: expect.any(Number) });
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
