// Action executor — dispatches validated commands to BrowserManager.
// Stub: will be fully implemented by the action executor task.

import type { Command, Response } from './protocol.js';
import { successResponse, errorResponse, dumpSchema, dumpAllSchemas } from './protocol.js';
import type { BrowserManager } from './browser.js';

/**
 * Execute a validated command against the BrowserManager and return a Response.
 */
export async function executeCommand(
  browser: BrowserManager,
  command: Command,
): Promise<Response> {
  const { id, action } = command;

  // Schema introspection — no browser needed
  if (action === 'schema') {
    const cmd = command as Command & { action: 'schema' };
    if (cmd.all) {
      return successResponse(id, dumpAllSchemas());
    }
    if (cmd.command) {
      const schema = dumpSchema(cmd.command);
      if (!schema) {
        return errorResponse(id, `Unknown action: ${cmd.command}`);
      }
      return successResponse(id, schema);
    }
    return successResponse(id, dumpAllSchemas());
  }

  // Profile list — no browser needed
  if (action === 'profile_list') {
    const { listProfiles } = await import('./stealth.js');
    return successResponse(id, { profiles: listProfiles() });
  }

  // All other actions require implementation
  return errorResponse(id, `Action "${action}" is not yet implemented.`);
}
