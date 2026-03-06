import path from 'node:path';

/**
 * Translate Playwright errors into actionable AI-friendly messages.
 */
export function toAIFriendlyError(error: unknown, selector?: string): Error {
  const msg = error instanceof Error ? error.message : String(error);

  // strict mode violation — selector matched multiple elements
  const strictMatch = msg.match(/strict mode violation.*resolved to (\d+) elements/i);
  if (strictMatch) {
    const count = strictMatch[1];
    return new Error(
      `Selector matched ${count} elements. Run 'snapshot' to get updated refs.`,
    );
  }

  // element intercepts pointer events (modal/overlay blocking)
  if (/intercepts pointer events/i.test(msg)) {
    return new Error(
      'Element blocked by another element (modal/overlay). Dismiss modals first.',
    );
  }

  // timeout exceeded
  if (/timeout/i.test(msg) && /exceeded/i.test(msg)) {
    return new Error("Action timed out. Run 'snapshot' to check page state.");
  }

  // "waiting for" + "to be visible" — element not found
  if (/waiting for/i.test(msg) && /to be visible/i.test(msg)) {
    return new Error(
      "Element not found. Run 'snapshot' to see current elements.",
    );
  }

  // element not visible (but NOT timeout — checked above)
  if (/not visible/i.test(msg)) {
    return new Error('Element not visible. Try scrolling into view.');
  }

  // passthrough — return as-is
  return error instanceof Error ? error : new Error(msg);
}

/**
 * Validate and normalize a file path.
 * Rejects path traversal (..) and control characters.
 */
export function validateFilePath(filePath: string): string {
  // Reject path traversal
  if (filePath.includes('..')) {
    throw new Error(
      `Invalid file path: path traversal ("..") is not allowed: ${filePath}`,
    );
  }

  // Reject control characters (ASCII < 0x20) except \n (0x0A) and \t (0x09)
  for (let i = 0; i < filePath.length; i++) {
    const code = filePath.charCodeAt(i);
    if (code < 0x20 && code !== 0x0a && code !== 0x09) {
      throw new Error(
        `Invalid file path: control character (0x${code.toString(16).padStart(2, '0')}) detected`,
      );
    }
  }

  return path.resolve(filePath);
}

/**
 * Strip control characters from input text.
 * Preserves newlines (0x0A) and tabs (0x09).
 */
export function sanitizeInput(text: string): string {
  let result = '';
  for (let i = 0; i < text.length; i++) {
    const code = text.charCodeAt(i);
    if (code < 0x20 && code !== 0x0a && code !== 0x09) {
      continue; // strip control character
    }
    result += text[i];
  }
  return result;
}

/**
 * Validate that a ref matches the expected format: e followed by digits.
 */
export function validateRef(ref: string): boolean {
  return /^e\d+$/.test(ref);
}
