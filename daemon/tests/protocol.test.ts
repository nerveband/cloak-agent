import { describe, it, expect } from 'vitest';
import {
  parseCommand,
  successResponse,
  errorResponse,
  serializeResponse,
  dumpSchema,
  dumpAllSchemas,
} from '../src/protocol.js';

describe('parseCommand', () => {
  it('parses a valid navigate command', () => {
    const result = parseCommand(
      JSON.stringify({ id: '1', action: 'navigate', url: 'https://example.com' })
    );
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.command.action).toBe('navigate');
      expect((result.command as any).url).toBe('https://example.com');
    }
  });

  it('rejects invalid JSON', () => {
    const result = parseCommand('not json at all{{{');
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toBe('Invalid JSON');
    }
  });

  it('rejects unknown action', () => {
    const result = parseCommand(JSON.stringify({ id: '1', action: 'explode' }));
    expect(result.ok).toBe(false);
    if (!result.ok) {
      expect(result.error).toBeTruthy();
    }
  });

  it('parses launch with stealth options (geoip, fingerprintSeed, proxy)', () => {
    const result = parseCommand(
      JSON.stringify({
        id: '2',
        action: 'launch',
        geoip: true,
        fingerprintSeed: 42,
        proxy: { server: 'http://proxy:8080', username: 'user', password: 'pass' },
        platform: 'linux',
        timezone: 'America/New_York',
        locale: 'en-US',
        gpuVendor: 'NVIDIA',
        gpuRenderer: 'GeForce RTX 3090',
        profile: 'default',
        humanize: true,
        humanPreset: 'careful',
        humanConfig: { clickDelay: 120 },
        contextOptions: { permissions: ['geolocation'] },
      })
    );
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.command.action).toBe('launch');
      const cmd = result.command as any;
      expect(cmd.geoip).toBe(true);
      expect(cmd.fingerprintSeed).toBe(42);
      expect(cmd.proxy.server).toBe('http://proxy:8080');
      expect(cmd.platform).toBe('linux');
      expect(cmd.profile).toBe('default');
      expect(cmd.humanize).toBe(true);
      expect(cmd.humanPreset).toBe('careful');
      expect(cmd.humanConfig).toEqual({ clickDelay: 120 });
      expect(cmd.contextOptions).toEqual({ permissions: ['geolocation'] });
    }
  });

  it('parses launch with proxy as string', () => {
    const result = parseCommand(
      JSON.stringify({ id: '3', action: 'launch', proxy: 'http://proxy:3128' })
    );
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect((result.command as any).proxy).toBe('http://proxy:3128');
    }
  });

  it('parses launch with advanced browser options', () => {
    const result = parseCommand(
      JSON.stringify({
        id: '3b',
        action: 'launch',
        userAgent: 'CustomAgent/1.0',
        viewport: { width: 1440, height: 900 },
        args: ['--disable-gpu'],
        executablePath: '/tmp/cloakbrowser',
        storageState: 'state.json',
        ignoreHTTPSErrors: true,
      })
    );
    expect(result.ok).toBe(true);
    if (result.ok) {
      const cmd = result.command as any;
      expect(cmd.userAgent).toBe('CustomAgent/1.0');
      expect(cmd.viewport).toEqual({ width: 1440, height: 900 });
      expect(cmd.args).toEqual(['--disable-gpu']);
      expect(cmd.executablePath).toBe('/tmp/cloakbrowser');
      expect(cmd.storageState).toBe('state.json');
      expect(cmd.ignoreHTTPSErrors).toBe(true);
    }
  });

  it('parses dialog accept and dismiss payloads', () => {
    const accept = parseCommand(
      JSON.stringify({ id: '3c', action: 'dialog', accept: true, promptText: 'hello' })
    );
    expect(accept.ok).toBe(true);

    const dismiss = parseCommand(
      JSON.stringify({ id: '3d', action: 'dialog', accept: false })
    );
    expect(dismiss.ok).toBe(true);
  });

  it('parses route and unroute payloads', () => {
    const route = parseCommand(
      JSON.stringify({
        id: '3e',
        action: 'route',
        url: 'https://example.com',
        handler: 'fulfill',
        body: '{}',
        status: 201,
      })
    );
    expect(route.ok).toBe(true);

    const unroute = parseCommand(
      JSON.stringify({ id: '3f', action: 'unroute' })
    );
    expect(unroute.ok).toBe(true);
  });

  it('parses console clear and semantic locator subactions', () => {
    const consoleResult = parseCommand(
      JSON.stringify({ id: '3g', action: 'console', clear: true })
    );
    expect(consoleResult.ok).toBe(true);

    const labelFill = parseCommand(
      JSON.stringify({
        id: '3h',
        action: 'getbylabel',
        text: 'Email',
        subaction: 'fill',
        value: 'user@test.com',
      })
    );
    expect(labelFill.ok).toBe(true);
  });

  it('parses stealth_status', () => {
    const result = parseCommand(JSON.stringify({ id: '4', action: 'stealth_status' }));
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.command.action).toBe('stealth_status');
    }
  });

  it('parses fingerprint_rotate with seed', () => {
    const result = parseCommand(
      JSON.stringify({ id: '5', action: 'fingerprint_rotate', seed: 12345 })
    );
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.command.action).toBe('fingerprint_rotate');
      expect((result.command as any).seed).toBe(12345);
    }
  });

  it('parses fingerprint_rotate without seed', () => {
    const result = parseCommand(
      JSON.stringify({ id: '5b', action: 'fingerprint_rotate' })
    );
    expect(result.ok).toBe(true);
  });

  it('parses profile_create', () => {
    const result = parseCommand(
      JSON.stringify({ id: '6', action: 'profile_create', name: 'my-profile' })
    );
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.command.action).toBe('profile_create');
      expect((result.command as any).name).toBe('my-profile');
    }
  });

  it('parses profile_list', () => {
    const result = parseCommand(JSON.stringify({ id: '6b', action: 'profile_list' }));
    expect(result.ok).toBe(true);
  });

  it('parses snapshot with interactive flag', () => {
    const result = parseCommand(
      JSON.stringify({
        id: '7',
        action: 'snapshot',
        interactive: true,
        maxDepth: 5,
        compact: true,
        selector: '#main',
      })
    );
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.command.action).toBe('snapshot');
      const cmd = result.command as any;
      expect(cmd.interactive).toBe(true);
      expect(cmd.maxDepth).toBe(5);
      expect(cmd.compact).toBe(true);
      expect(cmd.selector).toBe('#main');
    }
  });

  it('accepts dryRun on any command', () => {
    const result = parseCommand(
      JSON.stringify({ id: '8', action: 'navigate', url: 'https://x.com', dryRun: true })
    );
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect((result.command as any).dryRun).toBe(true);
    }
  });

  it('parses schema introspection command', () => {
    const result = parseCommand(
      JSON.stringify({ id: '9', action: 'schema', command: 'navigate', all: false })
    );
    expect(result.ok).toBe(true);
    if (result.ok) {
      expect(result.command.action).toBe('schema');
    }
  });

  it('rejects navigate without required url', () => {
    const result = parseCommand(JSON.stringify({ id: '10', action: 'navigate' }));
    expect(result.ok).toBe(false);
  });
});

describe('dumpSchema', () => {
  it('returns correct shape for navigate', () => {
    const schema = dumpSchema('navigate');
    expect(schema).not.toBeNull();
    expect(schema!.action).toEqual({ type: 'string', required: true });
    expect(schema!.id).toEqual({ type: 'string', required: true });
    expect(schema!.url).toEqual({ type: 'string', required: true });
    expect(schema!.waitUntil).toEqual(
      expect.objectContaining({ type: 'enum', required: false })
    );
    expect(schema!.waitUntil.values).toEqual(
      expect.arrayContaining(['load', 'domcontentloaded', 'networkidle'])
    );
    expect(schema!.dryRun).toEqual({ type: 'boolean', required: false });
  });

  it('returns null for unknown action', () => {
    expect(dumpSchema('nonexistent_action')).toBeNull();
  });

  it('returns correct shape for launch with stealth fields', () => {
    const schema = dumpSchema('launch');
    expect(schema).not.toBeNull();
    expect(schema!.geoip).toEqual({ type: 'boolean', required: false });
    expect(schema!.fingerprintSeed).toEqual({ type: 'number', required: false });
    expect(schema!.proxy).toEqual({ type: 'union', required: false });
    expect(schema!.platform).toEqual(
      expect.objectContaining({ type: 'enum', required: false })
    );
    expect(schema!.userAgent).toEqual({ type: 'string', required: false });
    expect(schema!.viewport).toEqual({ type: 'object', required: false });
    expect(schema!.args).toEqual({ type: 'array', required: false });
    expect(schema!.executablePath).toEqual({ type: 'string', required: false });
    expect(schema!.storageState).toEqual({ type: 'string', required: false });
    expect(schema!.ignoreHTTPSErrors).toEqual({ type: 'boolean', required: false });
    expect(schema!.humanize).toEqual({ type: 'boolean', required: false });
    expect(schema!.humanPreset).toEqual(
      expect.objectContaining({ type: 'enum', required: false })
    );
    expect(schema!.humanConfig).toEqual({ type: 'record', required: false });
    expect(schema!.contextOptions).toEqual({ type: 'record', required: false });
  });
});

describe('dumpAllSchemas', () => {
  it('returns object with multiple keys', () => {
    const all = dumpAllSchemas();
    expect(typeof all).toBe('object');
    const keys = Object.keys(all);
    expect(keys.length).toBeGreaterThan(50);
    expect(keys).toContain('launch');
    expect(keys).toContain('navigate');
    expect(keys).toContain('stealth_status');
    expect(keys).toContain('fingerprint_rotate');
    expect(keys).toContain('profile_create');
    expect(keys).toContain('profile_list');
    expect(keys).toContain('snapshot');
    expect(keys).toContain('schema');
  });
});

describe('successResponse', () => {
  it('creates correct shape', () => {
    const resp = successResponse('abc', { title: 'Hello' });
    expect(resp).toEqual({
      id: 'abc',
      ok: true,
      data: { title: 'Hello' },
    });
  });
});

describe('errorResponse', () => {
  it('creates correct shape', () => {
    const resp = errorResponse('xyz', 'Something went wrong');
    expect(resp).toEqual({
      id: 'xyz',
      ok: false,
      error: 'Something went wrong',
    });
  });
});

describe('serializeResponse', () => {
  it('produces valid JSON for success', () => {
    const resp = successResponse('1', { value: 42 });
    const json = serializeResponse(resp);
    const parsed = JSON.parse(json);
    expect(parsed.id).toBe('1');
    expect(parsed.ok).toBe(true);
    expect(parsed.data.value).toBe(42);
  });

  it('produces valid JSON for error', () => {
    const resp = errorResponse('2', 'fail');
    const json = serializeResponse(resp);
    const parsed = JSON.parse(json);
    expect(parsed.id).toBe('2');
    expect(parsed.ok).toBe(false);
    expect(parsed.error).toBe('fail');
  });
});
