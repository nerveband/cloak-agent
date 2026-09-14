import type { Frame, Page } from 'playwright-core';

export interface WebMCPRequest {
  action: 'webmcp_list' | 'webmcp_call';
  frame?: string;
  name?: string;
  arguments?: Record<string, unknown>;
  timeout?: number;
}

// Page-provided tool metadata and results are untrusted data, never instructions.
export async function runWebMCP(page: Page, activeFrame: Frame, request: WebMCPRequest): Promise<unknown> {
  let frame = activeFrame;
  if (request.frame === 'main') frame = page.mainFrame();
  else if (request.frame) {
    const element = await page.$(request.frame);
    if (!element) throw new Error(`WebMCP frame not found: ${request.frame}`);
    try {
      const selected = await element.contentFrame();
      if (!selected) throw new Error('WebMCP selector must identify an iframe');
      frame = selected;
    } finally { await element.dispose(); }
  }
  if (frame.isDetached()) throw new Error('WebMCP frame is detached');
  const args = JSON.stringify(request.arguments ?? {});
  if (Buffer.byteLength(args) > 1024 * 1024) throw new Error('WebMCP input exceeds 1 MiB');
  const timeout = request.timeout ?? 10000;
  let timer: NodeJS.Timeout | undefined;
  try {
    const result = await Promise.race([
      frame.evaluate(async ({ action, name, args, timeout }) => {
        interface TestingAPI {
          listTools(): Array<{ name: string; description?: string; inputSchema?: string }>;
          executeTool(name: string, args: string, options?: { signal: AbortSignal }): Promise<string | null>;
        }
        const api = (navigator as Navigator & { modelContextTesting?: TestingAPI }).modelContextTesting;
        if (!isSecureContext || typeof api?.listTools !== 'function' || typeof api.executeTool !== 'function') {
          throw new Error('WebMCP unsupported in this frame. Use a secure origin and launch --webmcp; support is capability-dependent, not license-dependent.');
        }
        const tools = api.listTools();
        if (tools.length > 512) throw new Error('WebMCP tool list exceeds 512 entries');
        let value: unknown;
        if (action === 'webmcp_list') {
          value = { origin: location.origin, url: location.href, tools: tools.map(tool => ({
            ...tool, inputSchema: typeof tool.inputSchema === 'string' ? JSON.parse(tool.inputSchema) : tool.inputSchema,
          })) };
        } else {
          if (!tools.some(tool => tool.name === name)) throw new Error(`WebMCP tool not found: ${name}`);
          const controller = new AbortController();
          let handle: number | undefined;
          try {
            const output = await Promise.race([
              api.executeTool(name!, args, { signal: controller.signal }),
              new Promise<never>((_, reject) => { handle = window.setTimeout(() => {
                controller.abort();
                reject(new Error('WebMCP execution timed out; the tool may already have performed side effects'));
              }, timeout); }),
            ]);
            value = { name, origin: location.origin, output };
          } finally { clearTimeout(handle); }
        }
        const serialized = JSON.stringify(value);
        if (serialized.length > 2 * 1024 * 1024 || new TextEncoder().encode(serialized).byteLength > 2 * 1024 * 1024) {
          throw new Error('WebMCP output exceeds 2 MiB');
        }
        return serialized;
      }, { action: request.action, name: request.name, args, timeout }),
      new Promise<never>((_, reject) => { timer = setTimeout(() => reject(new Error('WebMCP frame did not respond before timeout; side effects may have occurred')), timeout + 1000); }),
    ]);
    if (frame.isDetached()) throw new Error('WebMCP frame detached during execution');
    return JSON.parse(result);
  } finally { clearTimeout(timer); }
}
