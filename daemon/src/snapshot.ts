import { selectors, type Page, type Frame, type JSHandle } from 'playwright-core';

// ARIA snapshot engine — parses Playwright's accessibility tree and assigns
// @ref IDs to interactive elements so AI agents can target them with minimal tokens.

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export interface RefData {
  selector: string;
  role: string;
  name?: string;
  nth?: number;
}

export interface SnapshotOptions {
  selector?: string;
  interactive?: boolean;
  compact?: boolean;
  maxDepth?: number;
}

export interface SnapshotStats {
  lines: number;
  chars: number;
  tokens: number;
  refs: number;
  interactive: number;
}

// ---------------------------------------------------------------------------
// Ref counter
// ---------------------------------------------------------------------------

let refCounter = 0;

export function resetRefs(): void {
  refCounter = 0;
}

export function nextRef(): string {
  refCounter++;
  return `e${refCounter}`;
}

// ---------------------------------------------------------------------------
// Role classifications
// ---------------------------------------------------------------------------

export const INTERACTIVE_ROLES = new Set([
  'button', 'link', 'textbox', 'checkbox', 'radio', 'combobox', 'listbox',
  'menuitem', 'menuitemcheckbox', 'menuitemradio', 'option', 'searchbox',
  'slider', 'spinbutton', 'switch', 'tab', 'treeitem',
]);

export const CONTENT_ROLES = new Set([
  'heading', 'cell', 'gridcell', 'columnheader', 'rowheader', 'listitem',
  'article', 'region', 'main', 'navigation',
]);

export const STRUCTURAL_ROLES = new Set([
  'generic', 'group', 'list', 'table', 'row', 'rowgroup', 'grid',
  'treegrid', 'menu', 'menubar', 'toolbar', 'tablist', 'tree',
  'directory', 'document', 'application', 'presentation', 'none',
]);

// ---------------------------------------------------------------------------
// Selector builder
// ---------------------------------------------------------------------------

export function buildSelector(role: string, name?: string): string {
  if (name) {
    return `getByRole('${role}', { name: "${name}", exact: true })`;
  }
  return `getByRole('${role}')`;
}

// ---------------------------------------------------------------------------
// Role+name deduplication tracker
// ---------------------------------------------------------------------------

export interface RoleNameTracker {
  counts: Map<string, number>;
  refsByKey: Map<string, string[]>;
}

export function createRoleNameTracker(): RoleNameTracker {
  return {
    counts: new Map(),
    refsByKey: new Map(),
  };
}

function trackRoleName(tracker: RoleNameTracker, role: string, name: string | undefined, refId: string): number {
  const key = `${role}::${name ?? ''}`;
  const count = (tracker.counts.get(key) ?? 0) + 1;
  tracker.counts.set(key, count);

  const refs = tracker.refsByKey.get(key) ?? [];
  refs.push(refId);
  tracker.refsByKey.set(key, refs);

  return count;
}

// ---------------------------------------------------------------------------
// Remove nth from non-duplicates
// ---------------------------------------------------------------------------

export function removeNthFromNonDuplicates(
  refs: Record<string, RefData>,
  tracker: RoleNameTracker,
): void {
  for (const [key, count] of tracker.counts) {
    if (count === 1) {
      const refIds = tracker.refsByKey.get(key);
      if (refIds) {
        for (const refId of refIds) {
          if (refs[refId]) {
            delete refs[refId].nth;
          }
        }
      }
    }
  }
}

// ---------------------------------------------------------------------------
// Parse a single ARIA tree line
// ---------------------------------------------------------------------------

const LINE_RE = /^(\s*-\s*)(\w+)(?:\s+"([^"]*)")?(.*)$/;

interface ParsedLine {
  indent: string;
  role: string;
  name?: string;
  rest: string;
  depth: number;
}

function parseLine(line: string): ParsedLine | null {
  const m = line.match(LINE_RE);
  if (!m) return null;
  const indent = m[1];
  // Depth is determined by leading whitespace (each level = 2 spaces typically)
  const leadingSpaces = line.match(/^(\s*)/)?.[1].length ?? 0;
  const depth = Math.floor(leadingSpaces / 2);
  return {
    indent,
    role: m[2],
    name: m[3],       // may be undefined if no quoted string
    rest: m[4] ?? '',
    depth,
  };
}

// ---------------------------------------------------------------------------
// Main tree processor
// ---------------------------------------------------------------------------

export function processAriaTree(
  ariaTree: string,
  refs: Record<string, RefData>,
  options: SnapshotOptions = {},
): string {
  const lines = ariaTree.split('\n');
  const tracker = createRoleNameTracker();
  const outputLines: string[] = [];

  for (const line of lines) {
    if (!line.trim()) continue;

    const parsed = parseLine(line);
    if (!parsed) {
      // Non-matching lines (e.g. "- paragraph: Some text") — keep them in
      // full mode, skip in interactive mode
      if (!options.interactive) {
        if (options.maxDepth !== undefined) {
          const leadingSpaces = line.match(/^(\s*)/)?.[1].length ?? 0;
          const depth = Math.floor(leadingSpaces / 2);
          if (depth > options.maxDepth) continue;
        }
        outputLines.push(line);
      }
      continue;
    }

    const { role, name, rest, depth } = parsed;

    // maxDepth filter
    if (options.maxDepth !== undefined && depth > options.maxDepth) {
      continue;
    }

    const isInteractive = INTERACTIVE_ROLES.has(role);
    const isContent = CONTENT_ROLES.has(role);
    const isStructural = STRUCTURAL_ROLES.has(role);

    if (options.interactive) {
      // Interactive mode: only interactive elements, flattened, with refs
      if (!isInteractive) continue;

      const refId = nextRef();
      const nth = trackRoleName(tracker, role, name, refId);
      refs[refId] = {
        selector: buildSelector(role, name),
        role,
        ...(name !== undefined && { name }),
        nth,
      };

      // Flatten — no indentation
      const nameStr = name !== undefined ? ` "${name}"` : '';
      outputLines.push(`- ${role}${nameStr}${rest} [ref=${refId}]`);
    } else if (options.compact) {
      // Compact mode: skip unnamed structural, assign refs to interactive + named content
      if (isStructural && !name) continue;

      const shouldRef = isInteractive || (isContent && name !== undefined);
      if (shouldRef) {
        const refId = nextRef();
        const nth = trackRoleName(tracker, role, name, refId);
        refs[refId] = {
          selector: buildSelector(role, name),
          role,
          ...(name !== undefined && { name }),
          nth,
        };
        const nameStr = name !== undefined ? ` "${name}"` : '';
        outputLines.push(`${parsed.indent}${role}${nameStr}${rest} [ref=${refId}]`);
      } else {
        const nameStr = name !== undefined ? ` "${name}"` : '';
        outputLines.push(`${parsed.indent}${role}${nameStr}${rest}`);
      }
    } else {
      // Full mode: keep everything, assign refs to interactive + named content elements
      const shouldRef = isInteractive || (isContent && name !== undefined);
      if (shouldRef) {
        const refId = nextRef();
        const nth = trackRoleName(tracker, role, name, refId);
        refs[refId] = {
          selector: buildSelector(role, name),
          role,
          ...(name !== undefined && { name }),
          nth,
        };
        const nameStr = name !== undefined ? ` "${name}"` : '';
        outputLines.push(`${parsed.indent}${role}${nameStr}${rest} [ref=${refId}]`);
      } else {
        const nameStr = name !== undefined ? ` "${name}"` : '';
        outputLines.push(`${parsed.indent}${role}${nameStr}${rest}`);
      }
    }
  }

  removeNthFromNonDuplicates(refs, tracker);

  return outputLines.join('\n');
}

// ---------------------------------------------------------------------------
// Parse @ref argument
// ---------------------------------------------------------------------------

export function parseRef(arg: string): string | null {
  // @e1 → e1
  if (arg.startsWith('@')) {
    const rest = arg.slice(1);
    if (/^e\d+$/.test(rest)) return rest;
    return null;
  }
  // ref=e5 → e5
  if (arg.startsWith('ref=')) {
    const rest = arg.slice(4);
    if (/^e\d+$/.test(rest)) return rest;
    return null;
  }
  // bare e3 → e3
  if (/^e\d+$/.test(arg)) return arg;
  // anything else (CSS selectors, random text)
  return null;
}

// ---------------------------------------------------------------------------
// Snapshot stats
// ---------------------------------------------------------------------------

export function getSnapshotStats(
  tree: string,
  refs: Record<string, RefData>,
): SnapshotStats {
  const lines = tree.split('\n').filter((l) => l.length > 0);
  const chars = tree.length;
  const interactiveCount = Object.values(refs).filter((r) =>
    INTERACTIVE_ROLES.has(r.role),
  ).length;

  return {
    lines: lines.length,
    chars,
    tokens: Math.ceil(chars / 4),
    refs: Object.keys(refs).length,
    interactive: interactiveCount,
  };
}

// ---------------------------------------------------------------------------
// Enhanced snapshot (requires a Playwright Page)
// ---------------------------------------------------------------------------

// The selector engine owns refs in Playwright's isolated world. Page scripts
// cannot enumerate or rewrite its registry.
await selectors.register('cloakref', () => {
  const elements = new Map<string, Element>();
  const query = (root: Node, selector: string): Element | null => {
    if (selector.startsWith('bind:') && root.nodeType === 1) {
      for (const [id, element] of elements) {
        if (!element.isConnected) elements.delete(id);
      }
      elements.set(selector.slice(5), root as Element);
      return root as Element;
    }
    const element = elements.get(selector);
    return element?.isConnected && (root.nodeType === 9 ? element.ownerDocument === root : root.contains(element)) ? element : null;
  };
  return { query, queryAll(root: Node, selector: string) {
    const element = query(root, selector);
    return element ? [element] : [];
  } };
}, { contentScript: true });

const identities = new WeakMap<Page | Frame, { generation: number; handle: JSHandle<WeakMap<Element, string>> }>();

export async function getEnhancedSnapshot(
  page: Page | Frame,
  options: SnapshotOptions = {},
  generation = 0,
): Promise<{ tree: string; refs: Record<string, RefData> }> {
  let identity = identities.get(page);
  if (!identity || identity.generation !== generation) {
    await identity?.handle.dispose();
    identity = { generation, handle: await page.evaluateHandle(() => new WeakMap<Element, string>()) };
    identities.set(page, identity);
  }
  const scope = page.locator(options.selector ?? ':root');
  const ariaTree = await scope.ariaSnapshot();
  const temporary: Record<string, RefData> = {};
  let tree = processAriaTree(ariaTree, temporary, options);
  const refs: Record<string, RefData> = {};
  for (const [candidate, data] of Object.entries(temporary)) {
    let locator = scope.getByRole(data.role as any, data.name !== undefined ? { name: data.name, exact: true } : undefined);
    // A selected root can itself be interactive.
    locator = scope.and(page.getByRole(data.role as any, data.name !== undefined ? { name: data.name, exact: true } : undefined)).or(locator);
    if (data.nth !== undefined) locator = locator.nth(data.nth - 1);
    const ref = await locator.evaluate((element, { ids, proposed }) => {
      let id = ids.get(element);
      if (!id) {
        id = proposed;
        ids.set(element, id);
      }
      return id;
    }, { ids: identity.handle, proposed: candidate });
    await locator.locator(`cloakref=bind:${ref}`).count();
    refs[ref] = data;
    tree = tree.replace(`[ref=${candidate}]`, `[stable-ref=${ref}]`);
  }
  return { tree: tree.replaceAll('[stable-ref=', '[ref='), refs };
}
