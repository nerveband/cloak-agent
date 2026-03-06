// daemon.ts — Unix socket server with session management for cloak-agent.
// The Go CLI connects to this daemon over a Unix socket (or TCP on Windows)
// and sends newline-delimited JSON commands.  The daemon manages the browser
// lifecycle via BrowserManager and dispatches actions via executeCommand.

import * as net from 'node:net';
import * as fs from 'node:fs';
import * as os from 'node:os';
import * as path from 'node:path';
import * as crypto from 'node:crypto';

import { parseCommand, serializeResponse, errorResponse } from './protocol.js';
import { executeCommand } from './actions.js';
import { BrowserManager } from './browser.js';

// ---------------------------------------------------------------------------
// Session management
// ---------------------------------------------------------------------------

let currentSession: string = process.env.CLOAK_AGENT_SESSION || 'default';

export function setSession(session: string): void {
  currentSession = session;
}

export function getSession(): string {
  return currentSession;
}

// ---------------------------------------------------------------------------
// Directory helpers
// ---------------------------------------------------------------------------

/**
 * Application directory used for socket files, PID files, etc.
 * Prefers XDG_RUNTIME_DIR/cloak-agent, falls back to ~/.cloak-agent,
 * then to os.tmpdir()/cloak-agent.
 */
export function getAppDir(): string {
  if (process.env.XDG_RUNTIME_DIR) {
    return path.join(process.env.XDG_RUNTIME_DIR, 'cloak-agent');
  }
  const home = os.homedir();
  if (home) {
    return path.join(home, '.cloak-agent');
  }
  return path.join(os.tmpdir(), 'cloak-agent');
}

/**
 * Directory where socket files live.
 * Respects CLOAK_AGENT_SOCKET_DIR env override.
 */
export function getSocketDir(): string {
  return process.env.CLOAK_AGENT_SOCKET_DIR || getAppDir();
}

/**
 * Hash a session name into a TCP port in the ephemeral range 49152-65535.
 * Used on Windows where Unix sockets are unavailable.
 */
export function getPortForSession(session: string): number {
  const hash = crypto.createHash('sha256').update(session).digest();
  const value = hash.readUInt16BE(0);
  return 49152 + (value % (65535 - 49152 + 1));
}

/**
 * Return the socket path for a given session.
 * On Windows returns a TCP port number (as a number); on Unix returns a
 * filesystem path to the .sock file.
 */
export function getSocketPath(session?: string): string | number {
  const s = session ?? currentSession;
  if (process.platform === 'win32') {
    return getPortForSession(s);
  }
  return path.join(getSocketDir(), `${s}.sock`);
}

/**
 * PID file path for the daemon of a given session.
 */
export function getPidFile(session?: string): string {
  const s = session ?? currentSession;
  return path.join(getSocketDir(), `${s}.pid`);
}

/**
 * Stream port file path for a given session.
 */
export function getStreamPortFile(session?: string): string {
  const s = session ?? currentSession;
  return path.join(getSocketDir(), `${s}.stream`);
}

// ---------------------------------------------------------------------------
// Lifecycle helpers
// ---------------------------------------------------------------------------

/**
 * Check whether a daemon is already running for the given session by
 * reading the PID file and sending signal 0 to the process.
 */
export function isDaemonRunning(session?: string): boolean {
  const pidFile = getPidFile(session);
  try {
    const raw = fs.readFileSync(pidFile, 'utf-8').trim();
    const pid = parseInt(raw, 10);
    if (isNaN(pid)) return false;
    process.kill(pid, 0); // throws if process doesn't exist
    return true;
  } catch {
    return false;
  }
}

/**
 * Remove stale socket, PID, and stream files for the given session.
 */
export function cleanupSocket(session?: string): void {
  const sockPath = getSocketPath(session);
  const pidFile = getPidFile(session);
  const streamFile = getStreamPortFile(session);

  for (const f of [sockPath, pidFile, streamFile]) {
    if (typeof f === 'string') {
      try {
        fs.unlinkSync(f);
      } catch {
        // ignore — file may not exist
      }
    }
  }
}

/**
 * Return connection info for the daemon of a given session.
 */
export function getConnectionInfo(session?: string): { type: 'unix'; path: string } | { type: 'tcp'; port: number } {
  const sockPath = getSocketPath(session);
  if (typeof sockPath === 'number') {
    return { type: 'tcp', port: sockPath };
  }
  return { type: 'unix', path: sockPath };
}

// ---------------------------------------------------------------------------
// HTTP method detection (security — reject browser requests)
// ---------------------------------------------------------------------------

const HTTP_METHODS = ['GET ', 'POST ', 'PUT ', 'DELETE ', 'PATCH ', 'HEAD ', 'OPTIONS ', 'CONNECT '];

function looksLikeHTTP(data: Buffer): boolean {
  const head = data.subarray(0, 8).toString('ascii');
  return HTTP_METHODS.some((m) => head.startsWith(m));
}

// ---------------------------------------------------------------------------
// Actions that don't require a running browser
// ---------------------------------------------------------------------------

const NO_BROWSER_ACTIONS = new Set(['launch', 'close', 'schema', 'profile_list']);

// ---------------------------------------------------------------------------
// Environment-driven launch options
// ---------------------------------------------------------------------------

function buildAutoLaunchOptions(): Record<string, unknown> {
  const opts: Record<string, unknown> = {};

  if (process.env.CLOAK_AGENT_HEADED) {
    opts.headless = false;
  }
  if (process.env.CLOAK_AGENT_EXECUTABLE_PATH) {
    opts.executablePath = process.env.CLOAK_AGENT_EXECUTABLE_PATH;
  }
  if (process.env.CLOAK_AGENT_PROXY) {
    const proxyObj: Record<string, string> = { server: process.env.CLOAK_AGENT_PROXY };
    if (process.env.CLOAK_AGENT_PROXY_BYPASS) {
      proxyObj.bypass = process.env.CLOAK_AGENT_PROXY_BYPASS;
    }
    opts.proxy = proxyObj;
  }
  if (process.env.CLOAK_AGENT_ARGS) {
    opts.args = process.env.CLOAK_AGENT_ARGS.split(',').map((a) => a.trim()).filter(Boolean);
  }
  if (process.env.CLOAK_AGENT_USER_AGENT) {
    opts.userAgent = process.env.CLOAK_AGENT_USER_AGENT;
  }
  if (process.env.CLOAK_AGENT_PROFILE) {
    opts.profile = process.env.CLOAK_AGENT_PROFILE;
  }
  if (process.env.CLOAK_AGENT_STATE) {
    opts.storageState = process.env.CLOAK_AGENT_STATE;
  }
  if (process.env.CLOAK_AGENT_IGNORE_HTTPS_ERRORS) {
    opts.ignoreHTTPSErrors = true;
  }

  return opts;
}

// ---------------------------------------------------------------------------
// startDaemon
// ---------------------------------------------------------------------------

export interface DaemonOptions {
  session?: string;
}

export async function startDaemon(options?: DaemonOptions): Promise<void> {
  const session = options?.session ?? currentSession;

  // Ensure socket directory exists
  const socketDir = getSocketDir();
  fs.mkdirSync(socketDir, { recursive: true });

  // Clean stale socket/pid/stream files
  cleanupSocket(session);

  // Create browser manager
  const browserManager = new BrowserManager();

  // Create TCP/Unix server
  const server = net.createServer((socket: net.Socket) => {
    let buffer = '';

    socket.on('data', async (chunk: Buffer) => {
      // Security: reject raw HTTP requests
      if (buffer.length === 0 && looksLikeHTTP(chunk)) {
        socket.write(
          serializeResponse(
            errorResponse('_http', 'HTTP requests are not supported. Send newline-delimited JSON.'),
          ) + '\n',
        );
        socket.destroy();
        return;
      }

      buffer += chunk.toString('utf-8');

      // Process complete newline-delimited JSON messages
      let newlineIdx: number;
      while ((newlineIdx = buffer.indexOf('\n')) !== -1) {
        const line = buffer.slice(0, newlineIdx).trim();
        buffer = buffer.slice(newlineIdx + 1);

        if (line.length === 0) continue;

        // Parse the command
        const parsed = parseCommand(line);
        if (!parsed.ok) {
          const fail = parsed as import('./protocol.js').ParseFailure;
          const resp = errorResponse('unknown', fail.error);
          socket.write(serializeResponse(resp) + '\n');
          continue;
        }

        const success = parsed as import('./protocol.js').ParseSuccess;
        const command = success.command;

        // Auto-launch browser if needed
        if (!browserManager.isLaunched() && !NO_BROWSER_ACTIONS.has(command.action)) {
          try {
            const launchOpts = buildAutoLaunchOptions();
            await executeCommand(browserManager, {
              id: command.id,
              action: 'launch' as const,
              ...launchOpts,
            } as any);
          } catch (err) {
            const msg = err instanceof Error ? err.message : String(err);
            const resp = errorResponse(command.id, `Auto-launch failed: ${msg}`);
            socket.write(serializeResponse(resp) + '\n');
            continue;
          }
        }

        // Schema actions can be handled without a browser
        // (executeCommand handles this internally)

        try {
          const result = await executeCommand(browserManager, command);
          const resp = serializeResponse(result);

          // For close action: send response, then shut down
          if (command.action === 'close') {
            socket.write(resp + '\n', () => {
              gracefulShutdown(server, browserManager, session);
            });
            continue;
          }

          socket.write(resp + '\n');
        } catch (err) {
          const msg = err instanceof Error ? err.message : String(err);
          const resp = errorResponse(command.id, msg);
          socket.write(resp + '\n');
        }
      }
    });

    socket.on('error', () => {
      // Client disconnected unexpectedly — nothing to do
    });
  });

  // Write PID file
  const pidFile = getPidFile(session);
  fs.writeFileSync(pidFile, String(process.pid), 'utf-8');

  // Listen
  const sockPath = getSocketPath(session);
  await new Promise<void>((resolve, reject) => {
    server.on('error', reject);
    if (typeof sockPath === 'number') {
      // Windows TCP
      server.listen(sockPath, '127.0.0.1', () => resolve());
    } else {
      // Unix socket
      server.listen(sockPath, () => resolve());
    }
  });

  // Signal handlers for graceful shutdown
  const onSignal = () => {
    gracefulShutdown(server, browserManager, session);
  };
  process.on('SIGINT', onSignal);
  process.on('SIGTERM', onSignal);
  process.on('SIGHUP', onSignal);

  // Catch-all handlers
  process.on('uncaughtException', (err) => {
    console.error('[cloak-agent] uncaughtException:', err);
    cleanupSocket(session);
    process.exit(1);
  });

  process.on('unhandledRejection', (reason) => {
    console.error('[cloak-agent] unhandledRejection:', reason);
    cleanupSocket(session);
    process.exit(1);
  });
}

// ---------------------------------------------------------------------------
// Graceful shutdown
// ---------------------------------------------------------------------------

let shutdownInProgress = false;

async function gracefulShutdown(
  server: net.Server,
  browserManager: BrowserManager,
  session: string,
): Promise<void> {
  if (shutdownInProgress) return;
  shutdownInProgress = true;

  try {
    // Close browser if running
    if (browserManager.isLaunched()) {
      try {
        await browserManager.close();
      } catch {
        // best-effort
      }
    }
  } finally {
    // Close server
    server.close();

    // Clean up files
    cleanupSocket(session);

    process.exit(0);
  }
}

// ---------------------------------------------------------------------------
// Auto-start when run directly
// ---------------------------------------------------------------------------

if (process.argv[1]?.endsWith('daemon.js') || process.env.CLOAK_AGENT_DAEMON === '1') {
  startDaemon().catch((err) => {
    console.error('[cloak-agent] daemon failed to start:', err);
    cleanupSocket();
    process.exit(1);
  });
}
