import { describe, it, expect } from 'vitest';
import { getPortForSession } from '../src/daemon.js';

describe('daemon transport', () => {
  it('matches the Go session-port test vectors', () => {
    const vectors: Array<[string, number]> = [
      ['default', 63400],
      ['test-session', 51523],
      ['alpha', 52947],
      ['', 58288],
    ];

    for (const [session, expected] of vectors) {
      expect(getPortForSession(session), session).toBe(expected);
    }
  });
});
