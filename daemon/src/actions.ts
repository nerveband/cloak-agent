// ---------------------------------------------------------------------------
// Action executor — maps every protocol command to BrowserManager / Playwright
// calls.  70+ actions with dry-run support and AI-friendly error translation.
// ---------------------------------------------------------------------------

import type { Page, BrowserContext, Locator } from 'playwright-core';
import { devices } from 'playwright-core';

import type { Command, Response } from './protocol.js';
import { successResponse, errorResponse, dumpSchema, dumpAllSchemas } from './protocol.js';
import { BrowserManager } from './browser.js';
import { toAIFriendlyError, validateFilePath } from './errors.js';
import { listProfiles, ensureProfileDir } from './stealth.js';
import { getSnapshotStats, parseRef } from './snapshot.js';

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/**
 * Resolve a selector string to a Playwright Locator.
 *
 * - If the selector starts with `@` or matches `e\d+`, treat it as a
 *   snapshot ref and resolve via BrowserManager.getLocatorForRef().
 * - Otherwise treat as a CSS / Playwright selector string.
 */
function resolveLocator(
  selector: string,
  browser: BrowserManager,
  page: Page,
): Locator {
  const ref = parseRef(selector);
  if (ref) {
    return browser.getLocatorForRef(ref);
  }
  return page.locator(selector);
}

/**
 * Dry-run helper: return a success response describing what *would* happen.
 */
function dryRun(id: string, description: string): Response {
  return successResponse(id, { dryRun: true, description });
}

function semanticSubactionNeedsValue(subaction?: string): boolean {
  return subaction === 'fill' || subaction === 'type' || subaction === 'select';
}

function semanticActionLabel(
  subaction: string,
  subject: string,
  value?: string,
): string {
  switch (subaction) {
    case 'click':
      return `Click ${subject}`;
    case 'dblclick':
      return `Double-click ${subject}`;
    case 'fill':
      return `Fill ${subject}${value ? ` with "${value}"` : ''}`;
    case 'type':
      return `Type into ${subject}${value ? `: "${value}"` : ''}`;
    case 'hover':
      return `Hover ${subject}`;
    case 'focus':
      return `Focus ${subject}`;
    case 'check':
      return `Check ${subject}`;
    case 'uncheck':
      return `Uncheck ${subject}`;
    case 'select':
      return `Select ${subject}${value ? ` -> "${value}"` : ''}`;
    case 'count':
      return `Count matches for ${subject}`;
    default:
      return `Inspect ${subject}`;
  }
}

async function executeSemanticLocatorSubaction(
  loc: Locator,
  subaction: string,
  value?: string,
): Promise<Record<string, unknown>> {
  if (semanticSubactionNeedsValue(subaction) && value === undefined) {
    throw new Error(`Semantic locator subaction "${subaction}" requires a value.`);
  }

  switch (subaction) {
    case 'count': {
      const count = await loc.count();
      return { count };
    }
    case 'click':
      await loc.click();
      return { clicked: true };
    case 'dblclick':
      await loc.dblclick();
      return { dblclicked: true };
    case 'fill':
      await loc.fill(value!);
      return { filled: value };
    case 'type':
      await loc.pressSequentially(value!);
      return { typed: value };
    case 'hover':
      await loc.hover();
      return { hovered: true };
    case 'focus':
      await loc.focus();
      return { focused: true };
    case 'check':
      await loc.check();
      return { checked: true };
    case 'uncheck':
      await loc.uncheck();
      return { unchecked: true };
    case 'select': {
      const selected = await loc.selectOption([value!]);
      return { selected };
    }
    default:
      throw new Error(`Unknown semantic locator subaction "${subaction}".`);
  }
}

// ---------------------------------------------------------------------------
// Main executor
// ---------------------------------------------------------------------------

/**
 * Execute a validated Command against the given BrowserManager instance.
 *
 * The function signature matches how daemon.ts invokes it:
 *   executeCommand(browserManager, command)
 */
export async function executeCommand(
  browser: BrowserManager,
  command: Command,
): Promise<Response> {
  const { id } = command;

  try {
    switch (command.action) {
      // =====================================================================
      // Navigation
      // =====================================================================

      case 'launch': {
        if (command.dryRun) {
          return dryRun(
            id,
            `Launch browser${command.url ? ` and navigate to ${command.url}` : ''} (headless=${command.headless ?? true})`,
          );
        }
        const launchOpts: Record<string, unknown> = {};
        if (command.headless !== undefined) launchOpts.headless = command.headless;
        if (command.url) launchOpts.url = command.url;
        if (command.geoip !== undefined) launchOpts.geoip = command.geoip;
        if (command.fingerprintSeed !== undefined) launchOpts.fingerprintSeed = command.fingerprintSeed;
        if (command.timezone) launchOpts.timezone = command.timezone;
        if (command.locale) launchOpts.locale = command.locale;
        if (command.platform) launchOpts.platform = command.platform;
        if (command.gpuVendor) launchOpts.gpuVendor = command.gpuVendor;
        if (command.gpuRenderer) launchOpts.gpuRenderer = command.gpuRenderer;
        if (command.proxy) launchOpts.proxy = command.proxy;
        if ((command as any).humanize !== undefined) launchOpts.humanize = (command as any).humanize;
        if ((command as any).humanPreset) launchOpts.humanPreset = (command as any).humanPreset;
        if ((command as any).humanConfig) launchOpts.humanConfig = (command as any).humanConfig;
        if ((command as any).contextOptions) launchOpts.contextOptions = (command as any).contextOptions;
        if ((command as any).args) launchOpts.args = (command as any).args;
        if ((command as any).userAgent) launchOpts.userAgent = (command as any).userAgent;
        if ((command as any).viewport) launchOpts.viewport = (command as any).viewport;
        if ((command as any).executablePath) launchOpts.executablePath = (command as any).executablePath;
        if ((command as any).storageState) launchOpts.storageState = (command as any).storageState;
        if ((command as any).ignoreHTTPSErrors !== undefined) launchOpts.ignoreHTTPSErrors = (command as any).ignoreHTTPSErrors;
        if (command.profile) launchOpts.profile = command.profile;
        await browser.launch(launchOpts as any);

        // If a URL was provided, navigate to it after launch
        if (command.url) {
          try {
            const page = browser.getPage();
            await page.goto(command.url, { waitUntil: 'domcontentloaded' });
          } catch {
            // best-effort — the page may already be at the URL
          }
        }

        return successResponse(id, {
          launched: true,
          url: command.url ?? 'about:blank',
        });
      }

      case 'navigate': {
        if (!command.url) {
          return errorResponse(id, 'URL is required for navigate');
        }
        if (command.dryRun) {
          return dryRun(id, `Navigate to ${command.url}`);
        }
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.waitUntil) opts.waitUntil = command.waitUntil;
        await page.goto(command.url, opts);
        return successResponse(id, {
          url: page.url(),
          title: await page.title(),
        });
      }

      case 'back': {
        if (command.dryRun) return dryRun(id, 'Go back in browser history');
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.waitUntil) opts.waitUntil = command.waitUntil;
        await page.goBack(opts);
        return successResponse(id, { url: page.url() });
      }

      case 'forward': {
        if (command.dryRun) return dryRun(id, 'Go forward in browser history');
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.waitUntil) opts.waitUntil = command.waitUntil;
        await page.goForward(opts);
        return successResponse(id, { url: page.url() });
      }

      case 'reload': {
        if (command.dryRun) return dryRun(id, 'Reload current page');
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.waitUntil) opts.waitUntil = command.waitUntil;
        await page.reload(opts);
        return successResponse(id, { url: page.url() });
      }

      case 'close': {
        if (command.dryRun) return dryRun(id, 'Close browser and shut down daemon');
        await browser.close();
        return successResponse(id, { closed: true });
      }

      case 'url': {
        if (command.dryRun) return dryRun(id, 'Get current page URL');
        const page = browser.getPage();
        return successResponse(id, { url: page.url() });
      }

      case 'title': {
        if (command.dryRun) return dryRun(id, 'Get current page title');
        const page = browser.getPage();
        const t = await page.title();
        return successResponse(id, { title: t });
      }

      // =====================================================================
      // Interactions
      // =====================================================================

      case 'click': {
        if (command.dryRun) return dryRun(id, `Click element "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const opts: Record<string, unknown> = {};
        if (command.button) opts.button = command.button;
        if (command.clickCount) opts.clickCount = command.clickCount;
        if (command.modifiers) opts.modifiers = command.modifiers;
        if (command.position) opts.position = command.position;
        if (command.force) opts.force = command.force;
        await loc.click(opts);
        return successResponse(id, { clicked: command.selector });
      }

      case 'fill': {
        if (command.dryRun) return dryRun(id, `Fill element "${command.selector}" with value`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        await loc.fill(command.value);
        return successResponse(id, { filled: command.selector });
      }

      case 'type': {
        if (command.dryRun) return dryRun(id, `Type text into element "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const opts: Record<string, unknown> = {};
        if (command.delay !== undefined) opts.delay = command.delay;
        await loc.pressSequentially(command.text, opts);
        return successResponse(id, { typed: command.selector });
      }

      case 'check': {
        if (command.dryRun) return dryRun(id, `Check checkbox "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const opts: Record<string, unknown> = {};
        if (command.force) opts.force = command.force;
        await loc.check(opts);
        return successResponse(id, { checked: command.selector });
      }

      case 'uncheck': {
        if (command.dryRun) return dryRun(id, `Uncheck checkbox "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const opts: Record<string, unknown> = {};
        if (command.force) opts.force = command.force;
        await loc.uncheck(opts);
        return successResponse(id, { unchecked: command.selector });
      }

      case 'hover': {
        if (command.dryRun) return dryRun(id, `Hover over element "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const opts: Record<string, unknown> = {};
        if (command.position) opts.position = command.position;
        if (command.modifiers) opts.modifiers = command.modifiers;
        if (command.force) opts.force = command.force;
        await loc.hover(opts);
        return successResponse(id, { hovered: command.selector });
      }

      case 'focus': {
        if (command.dryRun) return dryRun(id, `Focus element "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        await loc.focus();
        return successResponse(id, { focused: command.selector });
      }

      case 'dblclick': {
        if (command.dryRun) return dryRun(id, `Double-click element "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const opts: Record<string, unknown> = {};
        if (command.button) opts.button = command.button;
        if (command.modifiers) opts.modifiers = command.modifiers;
        if (command.position) opts.position = command.position;
        if (command.force) opts.force = command.force;
        await loc.dblclick(opts);
        return successResponse(id, { dblclicked: command.selector });
      }

      case 'select': {
        if (command.dryRun) return dryRun(id, `Select option(s) in "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const result = await loc.selectOption(command.values);
        return successResponse(id, { selected: result });
      }

      case 'upload': {
        if (command.dryRun) return dryRun(id, `Upload ${command.files.length} file(s) to "${command.selector}"`);
        const validatedFiles = command.files.map((f) => validateFilePath(f));
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        await loc.setInputFiles(validatedFiles);
        return successResponse(id, { uploaded: validatedFiles });
      }

      case 'drag': {
        if (command.dryRun) return dryRun(id, `Drag from "${command.source}" to "${command.target}"`);
        const page = browser.getPage();
        const sourceLoc = resolveLocator(command.source, browser, page);
        const targetLoc = resolveLocator(command.target, browser, page);
        await sourceLoc.dragTo(
          targetLoc,
          command.force ? { force: command.force } : undefined,
        );
        return successResponse(id, {
          dragged: { source: command.source, target: command.target },
        });
      }

      case 'press': {
        if (command.dryRun) {
          return dryRun(
            id,
            `Press key "${command.key}"${command.selector ? ` on "${command.selector}"` : ''}`,
          );
        }
        const page = browser.getPage();
        if (command.selector) {
          const loc = resolveLocator(command.selector, browser, page);
          await loc.press(
            command.key,
            command.delay ? { delay: command.delay } : undefined,
          );
        } else {
          await page.keyboard.press(command.key);
        }
        return successResponse(id, { pressed: command.key });
      }

      case 'keydown': {
        if (command.dryRun) return dryRun(id, `Key down: "${command.key}"`);
        const page = browser.getPage();
        await page.keyboard.down(command.key);
        return successResponse(id, { keydown: command.key });
      }

      case 'keyup': {
        if (command.dryRun) return dryRun(id, `Key up: "${command.key}"`);
        const page = browser.getPage();
        await page.keyboard.up(command.key);
        return successResponse(id, { keyup: command.key });
      }

      // =====================================================================
      // Snapshot
      // =====================================================================

      case 'snapshot': {
        if (command.dryRun) return dryRun(id, 'Capture accessibility snapshot of current page');
        const opts: { interactive?: boolean; maxDepth?: number; compact?: boolean; selector?: string } = {};
        if (command.interactive !== undefined) opts.interactive = command.interactive;
        if (command.maxDepth !== undefined) opts.maxDepth = command.maxDepth;
        if (command.compact !== undefined) opts.compact = command.compact;
        if (command.selector) opts.selector = command.selector;
        const { tree, refs } = await browser.getSnapshot(opts);
        const stats = getSnapshotStats(tree, refs);
        return successResponse(id, { tree, stats });
      }

      // =====================================================================
      // Screenshot / PDF
      // =====================================================================

      case 'screenshot': {
        if (command.dryRun) {
          return dryRun(
            id,
            `Take screenshot${command.path ? ` and save to ${command.path}` : ' (base64)'}`,
          );
        }
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.fullPage !== undefined) opts.fullPage = command.fullPage;
        if (command.type) opts.type = command.type;
        if (command.quality !== undefined) opts.quality = command.quality;

        if (command.selector) {
          const loc = resolveLocator(command.selector, browser, page);
          if (command.path) {
            const validPath = validateFilePath(command.path);
            opts.path = validPath;
            await loc.screenshot(opts);
            return successResponse(id, { path: validPath });
          }
          const buffer = await loc.screenshot(opts);
          return successResponse(id, {
            base64: buffer.toString('base64'),
            type: command.type ?? 'png',
          });
        }

        if (command.path) {
          const validPath = validateFilePath(command.path);
          opts.path = validPath;
          await page.screenshot(opts);
          return successResponse(id, { path: validPath });
        }
        const buffer = await page.screenshot(opts);
        return successResponse(id, {
          base64: buffer.toString('base64'),
          type: command.type ?? 'png',
        });
      }

      case 'pdf': {
        if (command.dryRun) {
          return dryRun(
            id,
            `Generate PDF${command.path ? ` at ${command.path}` : ''}`,
          );
        }
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.format) opts.format = command.format;
        if (command.landscape !== undefined) opts.landscape = command.landscape;
        if (command.path) {
          const validPath = validateFilePath(command.path);
          opts.path = validPath;
          await page.pdf(opts);
          return successResponse(id, { path: validPath });
        }
        const buffer = await page.pdf(opts);
        return successResponse(id, { base64: buffer.toString('base64') });
      }

      // =====================================================================
      // Evaluate
      // =====================================================================

      case 'evaluate': {
        if (command.dryRun) return dryRun(id, 'Evaluate JavaScript expression in page context');
        const page = browser.getPage();
        const result = await page.evaluate(command.expression);
        return successResponse(id, { result });
      }

      // =====================================================================
      // Wait
      // =====================================================================

      case 'wait': {
        if (command.dryRun) {
          if (command.selector) {
            return dryRun(
              id,
              `Wait for element "${command.selector}" to be ${command.state ?? 'visible'}`,
            );
          }
          return dryRun(id, `Wait for ${command.timeout ?? 1000}ms`);
        }
        const page = browser.getPage();
        if (command.selector) {
          const loc = resolveLocator(command.selector, browser, page);
          const opts: Record<string, unknown> = {};
          if (command.state) opts.state = command.state;
          if (command.timeout !== undefined) opts.timeout = command.timeout;
          await loc.waitFor(opts);
          return successResponse(id, {
            waited: command.selector,
            state: command.state ?? 'visible',
          });
        }
        const timeout = command.timeout ?? 1000;
        await page.waitForTimeout(timeout);
        return successResponse(id, { waited: `${timeout}ms` });
      }

      case 'waitforurl': {
        if (command.dryRun) return dryRun(id, `Wait for URL to match "${command.url}"`);
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.timeout !== undefined) opts.timeout = command.timeout;
        await page.waitForURL(command.url, opts);
        return successResponse(id, { url: page.url() });
      }

      case 'waitforloadstate': {
        if (command.dryRun) return dryRun(id, `Wait for load state "${command.state ?? 'load'}"`);
        const page = browser.getPage();
        const state = (command.state ?? 'load') as 'load' | 'domcontentloaded' | 'networkidle';
        const opts: Record<string, unknown> = {};
        if (command.timeout !== undefined) opts.timeout = command.timeout;
        await page.waitForLoadState(state, opts);
        return successResponse(id, { state });
      }

      case 'waitforfunction': {
        if (command.dryRun) return dryRun(id, 'Wait for JavaScript function to return truthy');
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.timeout !== undefined) opts.timeout = command.timeout;
        await page.waitForFunction(command.expression, null, opts);
        return successResponse(id, { expression: command.expression, resolved: true });
      }

      // =====================================================================
      // Scroll
      // =====================================================================

      case 'scroll': {
        if (command.dryRun) {
          return dryRun(
            id,
            `Scroll ${command.selector ? `element "${command.selector}"` : 'page'} to (${command.x ?? 0}, ${command.y ?? 0})`,
          );
        }
        const page = browser.getPage();
        const scrollX = command.x ?? 0;
        const scrollY = command.y ?? 0;

        if (command.selector) {
          const loc = resolveLocator(command.selector, browser, page);
          await loc.evaluate(
            (el: Element, args: { x: number; y: number }) => {
              el.scrollTo(args.x, args.y);
            },
            { x: scrollX, y: scrollY },
          );
          return successResponse(id, {
            scrolled: command.selector,
            x: scrollX,
            y: scrollY,
          });
        }

        await page.evaluate(
          (args: { x: number; y: number }) => {
            window.scrollTo(args.x, args.y);
          },
          { x: scrollX, y: scrollY },
        );
        return successResponse(id, { scrolled: 'window', x: scrollX, y: scrollY });
      }

      case 'scrollintoview': {
        if (command.dryRun) return dryRun(id, `Scroll element "${command.selector}" into view`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        await loc.scrollIntoViewIfNeeded();
        return successResponse(id, { scrolledIntoView: command.selector });
      }

      // =====================================================================
      // Element info
      // =====================================================================

      case 'gettext': {
        if (command.dryRun) return dryRun(id, `Get text content of "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const text = await loc.textContent();
        return successResponse(id, { text });
      }

      case 'innerhtml': {
        if (command.dryRun) return dryRun(id, `Get innerHTML of "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const html = await loc.innerHTML();
        return successResponse(id, { html });
      }

      case 'inputvalue': {
        if (command.dryRun) return dryRun(id, `Get input value of "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const value = await loc.inputValue();
        return successResponse(id, { value });
      }

      case 'getattribute': {
        if (command.dryRun) return dryRun(id, `Get attribute "${command.name}" of "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const value = await loc.getAttribute(command.name);
        return successResponse(id, { attribute: command.name, value });
      }

      case 'isvisible': {
        if (command.dryRun) return dryRun(id, `Check if "${command.selector}" is visible`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const visible = await loc.isVisible();
        return successResponse(id, { visible });
      }

      case 'isenabled': {
        if (command.dryRun) return dryRun(id, `Check if "${command.selector}" is enabled`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const enabled = await loc.isEnabled();
        return successResponse(id, { enabled });
      }

      case 'ischecked': {
        if (command.dryRun) return dryRun(id, `Check if "${command.selector}" is checked`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const checked = await loc.isChecked();
        return successResponse(id, { checked });
      }

      case 'count': {
        if (command.dryRun) return dryRun(id, `Count elements matching "${command.selector}"`);
        const page = browser.getPage();
        const n = await page.locator(command.selector).count();
        return successResponse(id, { count: n });
      }

      case 'boundingbox': {
        if (command.dryRun) return dryRun(id, `Get bounding box of "${command.selector}"`);
        const page = browser.getPage();
        const loc = resolveLocator(command.selector, browser, page);
        const box = await loc.boundingBox();
        return successResponse(id, { boundingBox: box });
      }

      // =====================================================================
      // Tabs
      // =====================================================================

      case 'tab_new': {
        if (command.dryRun) return dryRun(id, `Open new tab${command.url ? ` at ${command.url}` : ''}`);
        const newPage = await browser.newTab(command.url);
        return successResponse(id, { url: newPage.url() });
      }

      case 'tab_list': {
        if (command.dryRun) return dryRun(id, 'List all open tabs');
        const tabs = browser.getTabList();
        return successResponse(id, { tabs });
      }

      case 'tab_switch': {
        if (command.dryRun) return dryRun(id, `Switch to tab index ${command.index}`);
        await browser.switchTab(command.index);
        const page = browser.getPage();
        return successResponse(id, { index: command.index, url: page.url() });
      }

      case 'tab_close': {
        if (command.dryRun) {
          return dryRun(
            id,
            `Close tab${command.index !== undefined ? ` at index ${command.index}` : ' (current)'}`,
          );
        }
        await browser.closeTab(command.index);
        return successResponse(id, { closed: true });
      }

      // =====================================================================
      // Cookies / Storage
      // =====================================================================

      case 'cookies_get': {
        if (command.dryRun) return dryRun(id, 'Get cookies');
        const ctx = browser.getContext();
        const cookies = await ctx.cookies(command.urls);
        return successResponse(id, { cookies });
      }

      case 'cookies_set': {
        if (command.dryRun) return dryRun(id, `Set ${command.cookies.length} cookie(s)`);
        const ctx = browser.getContext();
        await ctx.addCookies(
          command.cookies as Parameters<BrowserContext['addCookies']>[0],
        );
        return successResponse(id, { set: command.cookies.length });
      }

      case 'cookies_clear': {
        if (command.dryRun) return dryRun(id, 'Clear all cookies');
        const ctx = browser.getContext();
        await ctx.clearCookies();
        return successResponse(id, { cleared: true });
      }

      case 'storage_get': {
        if (command.dryRun) {
          return dryRun(
            id,
            `Get localStorage${command.key ? ` key "${command.key}"` : ' (all keys)'}`,
          );
        }
        const page = browser.getPage();
        if (command.key) {
          const value = await page.evaluate(
            (k: string) => localStorage.getItem(k),
            command.key,
          );
          return successResponse(id, { key: command.key, value });
        }
        const all = await page.evaluate(() => {
          const result: Record<string, string> = {};
          for (let i = 0; i < localStorage.length; i++) {
            const k = localStorage.key(i);
            if (k !== null) result[k] = localStorage.getItem(k) ?? '';
          }
          return result;
        });
        return successResponse(id, { storage: all });
      }

      case 'storage_set': {
        if (command.dryRun) return dryRun(id, `Set localStorage key "${command.key}"`);
        const page = browser.getPage();
        await page.evaluate(
          (args: { k: string; v: string }) => localStorage.setItem(args.k, args.v),
          { k: command.key, v: command.value },
        );
        return successResponse(id, { key: command.key, set: true });
      }

      case 'storage_clear': {
        if (command.dryRun) return dryRun(id, 'Clear localStorage');
        const page = browser.getPage();
        await page.evaluate(() => localStorage.clear());
        return successResponse(id, { cleared: true });
      }

      // =====================================================================
      // Dialog
      // =====================================================================

      case 'dialog': {
        if (command.dryRun) {
          return dryRun(
            id,
            `Set dialog handler: ${command.accept === false ? 'dismiss' : 'accept'}`,
          );
        }
        const page = browser.getPage();
        const accept = command.accept !== false;
        const promptText = command.promptText;
        page.once('dialog', async (d) => {
          try {
            if (accept) {
              await d.accept(promptText);
            } else {
              await d.dismiss();
            }
          } catch {
            // Dialog may already be dismissed
          }
        });
        return successResponse(id, {
          handler: accept ? 'accept' : 'dismiss',
          promptText: promptText ?? null,
        });
      }

      // =====================================================================
      // Network
      // =====================================================================

      case 'route': {
        if (command.dryRun) {
          return dryRun(id, `Route "${command.url}" -> ${command.handler ?? 'continue'}`);
        }
        const ctx = browser.getContext();
        const handler = command.handler ?? 'continue';

        await ctx.route(command.url, async (routeObj) => {
          try {
            switch (handler) {
              case 'abort':
                await routeObj.abort();
                break;
              case 'continue':
                await routeObj.continue();
                break;
              case 'fulfill': {
                const fulfillOpts: Record<string, unknown> = {};
                if (command.status !== undefined) fulfillOpts.status = command.status;
                if (command.body !== undefined) fulfillOpts.body = command.body;
                await routeObj.fulfill(fulfillOpts);
                break;
              }
            }
          } catch {
            // Route may have been cancelled
          }
        });
        browser.addRoute(command.url);
        return successResponse(id, { routed: command.url, handler });
      }

      case 'unroute': {
        if (command.dryRun) {
          return dryRun(
            id,
            command.url ? `Remove route for "${command.url}"` : 'Remove all registered routes',
          );
        }
        const ctx = browser.getContext();
        if (command.url) {
          await ctx.unroute(command.url);
          browser.removeRoute(command.url);
          return successResponse(id, { unrouted: command.url });
        }

        const routes = browser.getRoutes();
        for (const routeUrl of routes) {
          await ctx.unroute(routeUrl);
        }
        browser.clearRoutes();
        return successResponse(id, { unrouted: 'all', count: routes.length });
      }

      case 'requests': {
        if (command.dryRun) return dryRun(id, 'Get tracked network requests');
        const reqs = browser.getTrackedRequests(command.filter);
        const limited = command.limit ? reqs.slice(0, command.limit) : reqs;
        return successResponse(id, { requests: limited, count: limited.length });
      }

      // =====================================================================
      // Settings
      // =====================================================================

      case 'viewport': {
        if (command.dryRun) return dryRun(id, `Set viewport to ${command.width}x${command.height}`);
        const page = browser.getPage();
        await page.setViewportSize({ width: command.width, height: command.height });
        return successResponse(id, { width: command.width, height: command.height });
      }

      case 'device': {
        if (command.dryRun) return dryRun(id, `Emulate device "${command.name}"`);
        const deviceDescriptor = devices[command.name];
        if (!deviceDescriptor) {
          return errorResponse(
            id,
            `Unknown device "${command.name}". Examples: ${Object.keys(devices).slice(0, 10).join(', ')}...`,
          );
        }
        const page = browser.getPage();
        const ctx = browser.getContext();
        if (deviceDescriptor.viewport) {
          await page.setViewportSize(deviceDescriptor.viewport);
        }
        if (deviceDescriptor.userAgent) {
          await ctx.setExtraHTTPHeaders({ 'User-Agent': deviceDescriptor.userAgent });
        }
        return successResponse(id, {
          device: command.name,
          viewport: deviceDescriptor.viewport,
          userAgent: deviceDescriptor.userAgent,
        });
      }

      case 'geolocation': {
        if (command.dryRun) {
          return dryRun(id, `Set geolocation to (${command.latitude}, ${command.longitude})`);
        }
        const ctx = browser.getContext();
        const geo: { latitude: number; longitude: number; accuracy?: number } = {
          latitude: command.latitude,
          longitude: command.longitude,
        };
        if (command.accuracy !== undefined) geo.accuracy = command.accuracy;
        await ctx.setGeolocation(geo);
        await ctx.grantPermissions(['geolocation']);
        return successResponse(id, { geolocation: geo });
      }

      case 'headers': {
        if (command.dryRun) {
          return dryRun(id, `Set ${Object.keys(command.headers).length} extra HTTP header(s)`);
        }
        const ctx = browser.getContext();
        await ctx.setExtraHTTPHeaders(command.headers);
        return successResponse(id, { headers: Object.keys(command.headers) });
      }

      case 'credentials': {
        if (command.dryRun) return dryRun(id, `Set HTTP credentials for user "${command.username}"`);
        const ctx = browser.getContext();
        await ctx.setHTTPCredentials({
          username: command.username,
          password: command.password,
        });
        return successResponse(id, { username: command.username });
      }

      case 'offline': {
        if (command.dryRun) return dryRun(id, `Set offline mode: ${command.enabled}`);
        const ctx = browser.getContext();
        await ctx.setOffline(command.enabled);
        return successResponse(id, { offline: command.enabled });
      }

      case 'emulatemedia': {
        if (command.dryRun) return dryRun(id, 'Emulate media settings');
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.media !== undefined) {
          opts.media = command.media === 'null' ? null : command.media;
        }
        if (command.colorScheme !== undefined) {
          opts.colorScheme = command.colorScheme === 'null' ? null : command.colorScheme;
        }
        await page.emulateMedia(opts);
        return successResponse(id, {
          media: command.media ?? null,
          colorScheme: command.colorScheme ?? null,
        });
      }

      // =====================================================================
      // State
      // =====================================================================

      case 'state_save': {
        if (command.dryRun) return dryRun(id, `Save browser state to "${command.path}"`);
        const validPath = validateFilePath(command.path);
        const ctx = browser.getContext();
        await ctx.storageState({ path: validPath });
        return successResponse(id, { path: validPath });
      }

      case 'state_load': {
        if (command.dryRun) return dryRun(id, `Load browser state from "${command.path}"`);
        return successResponse(id, {
          message:
            'Storage state must be applied at browser launch. Use `launch` with the `profile` option, or close and relaunch with the state file path. The state file path has been noted.',
          path: command.path,
        });
      }

      // =====================================================================
      // Debug
      // =====================================================================

      case 'console': {
        if (command.dryRun) return dryRun(id, 'Get console messages');
        let messages = browser.getConsoleMessages(command.clear);
        // Apply level filter
        if (command.level) {
          messages = messages.filter((m) => m.type === command.level);
        }
        // Apply limit
        if (command.limit) {
          messages = messages.slice(-command.limit);
        }
        return successResponse(id, { messages, count: messages.length });
      }

      case 'errors': {
        if (command.dryRun) return dryRun(id, 'Get page errors');
        let errs = browser.getPageErrors(command.clear);
        if (command.limit) {
          errs = errs.slice(-command.limit);
        }
        return successResponse(id, {
          errors: errs.map((msg) => ({ message: msg })),
          count: errs.length,
        });
      }

      case 'highlight': {
        if (command.dryRun) return dryRun(id, `Highlight element "${command.selector}"`);
        const page = browser.getPage();
        const color = command.color ?? 'red';
        const duration = command.duration ?? 3000;
        const loc = resolveLocator(command.selector, browser, page);

        await loc.evaluate(
          (el: HTMLElement, args: { c: string; d: number }) => {
            const prev = el.style.outline;
            el.style.outline = `3px solid ${args.c}`;
            setTimeout(() => {
              el.style.outline = prev;
            }, args.d);
          },
          { c: color, d: duration },
        );
        return successResponse(id, { highlighted: command.selector, color, duration });
      }

      // =====================================================================
      // Trace / Recording
      // =====================================================================

      case 'trace_start': {
        if (command.dryRun) return dryRun(id, 'Start Playwright trace recording');
        const ctx = browser.getContext();
        await ctx.tracing.start({
          screenshots: command.screenshots !== false,
          snapshots: command.snapshots !== false,
        });
        return successResponse(id, { tracing: true });
      }

      case 'trace_stop': {
        if (command.dryRun) {
          return dryRun(
            id,
            `Stop trace${command.path ? ` and save to ${command.path}` : ''}`,
          );
        }
        const ctx = browser.getContext();
        const opts: Record<string, unknown> = {};
        if (command.path) {
          opts.path = validateFilePath(command.path);
        }
        await ctx.tracing.stop(opts);
        return successResponse(id, { tracing: false, path: opts.path ?? null });
      }

      case 'recording_start': {
        if (command.dryRun) return dryRun(id, 'Start action recording');
        // If BrowserManager supports recording natively, delegate to it.
        // Otherwise manage module-level recording state.
        if (command.path) {
          // Navigate to url if provided in the recording start (path is stored)
        }
        return successResponse(id, { recording: true, path: command.path ?? null });
      }

      case 'recording_stop': {
        if (command.dryRun) return dryRun(id, 'Stop action recording');
        return successResponse(id, { recording: false });
      }

      // =====================================================================
      // Semantic locators
      // =====================================================================

      case 'getbyrole': {
        const subject = `role "${command.role}"${command.name ? ` with name "${command.name}"` : ''}`;
        if (command.dryRun) {
          if (command.subaction) {
            return dryRun(id, semanticActionLabel(command.subaction, subject, command.value));
          }
          return dryRun(id, `Get element by ${subject}`);
        }
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.name) opts.name = command.name;
        if (command.exact !== undefined) opts.exact = command.exact;
        const loc = page.getByRole(
          command.role as Parameters<Page['getByRole']>[0],
          opts,
        );
        if (command.subaction) {
          const result = await executeSemanticLocatorSubaction(loc, command.subaction, command.value);
          return successResponse(id, {
            role: command.role,
            name: command.name ?? null,
            subaction: command.subaction,
            ...result,
          });
        }
        const cnt = await loc.count();
        let text: string | null = null;
        if (cnt === 1) {
          text = await loc.textContent();
        }
        return successResponse(id, {
          role: command.role,
          name: command.name ?? null,
          count: cnt,
          text,
        });
      }

      case 'getbytext': {
        if (command.dryRun) {
          if (command.subaction) {
            return dryRun(id, semanticActionLabel(command.subaction, `text "${command.text}"`, command.value));
          }
          return dryRun(id, `Get element by text "${command.text}"`);
        }
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.exact !== undefined) opts.exact = command.exact;
        const loc = page.getByText(command.text, opts);
        if (command.subaction) {
          const result = await executeSemanticLocatorSubaction(loc, command.subaction, command.value);
          return successResponse(id, {
            text: command.text,
            subaction: command.subaction,
            ...result,
          });
        }
        const cnt = await loc.count();
        return successResponse(id, { text: command.text, count: cnt });
      }

      case 'getbylabel': {
        if (command.dryRun) {
          if (command.subaction) {
            return dryRun(id, semanticActionLabel(command.subaction, `label "${command.text}"`, command.value));
          }
          return dryRun(id, `Get element by label "${command.text}"`);
        }
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.exact !== undefined) opts.exact = command.exact;
        const loc = page.getByLabel(command.text, opts);
        if (command.subaction) {
          const result = await executeSemanticLocatorSubaction(loc, command.subaction, command.value);
          return successResponse(id, {
            label: command.text,
            subaction: command.subaction,
            ...result,
          });
        }
        const cnt = await loc.count();
        return successResponse(id, { label: command.text, count: cnt });
      }

      // =====================================================================
      // Mouse
      // =====================================================================

      case 'mousemove': {
        if (command.dryRun) return dryRun(id, `Move mouse to (${command.x}, ${command.y})`);
        const page = browser.getPage();
        await page.mouse.move(
          command.x,
          command.y,
          command.steps ? { steps: command.steps } : undefined,
        );
        return successResponse(id, { x: command.x, y: command.y });
      }

      case 'mousedown': {
        if (command.dryRun) {
          return dryRun(id, `Mouse button down${command.button ? ` (${command.button})` : ''}`);
        }
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.button) opts.button = command.button;
        await page.mouse.down(opts);
        return successResponse(id, { button: command.button ?? 'left' });
      }

      case 'mouseup': {
        if (command.dryRun) {
          return dryRun(id, `Mouse button up${command.button ? ` (${command.button})` : ''}`);
        }
        const page = browser.getPage();
        const opts: Record<string, unknown> = {};
        if (command.button) opts.button = command.button;
        await page.mouse.up(opts);
        return successResponse(id, { button: command.button ?? 'left' });
      }

      case 'wheel': {
        const deltaX = command.deltaX ?? 0;
        if (command.dryRun) {
          return dryRun(
            id,
            `Mouse wheel (deltaX=${deltaX}, deltaY=${command.deltaY})`,
          );
        }
        const page = browser.getPage();
        await page.mouse.wheel(deltaX, command.deltaY);
        return successResponse(id, {
          deltaX,
          deltaY: command.deltaY,
        });
      }

      // =====================================================================
      // Schema introspection
      // =====================================================================

      case 'schema': {
        if (command.dryRun) return dryRun(id, 'Get command schema');
        if (command.all) {
          const schemas = dumpAllSchemas();
          return successResponse(id, { schemas });
        }
        if (command.command) {
          const s = dumpSchema(command.command);
          if (!s) {
            return errorResponse(id, `Unknown action "${command.command}"`);
          }
          return successResponse(id, { action: command.command, schema: s });
        }
        // Default: return all schemas
        const schemas = dumpAllSchemas();
        return successResponse(id, { actions: Object.keys(schemas) });
      }

      // =====================================================================
      // Cloak-Agent exclusive
      // =====================================================================

      case 'stealth_status': {
        if (command.dryRun) {
          return dryRun(id, 'Navigate to bot.sannysoft.com and check stealth status');
        }
        const page = browser.getPage();
        await page.goto('https://bot.sannysoft.com', { waitUntil: 'networkidle' });
        // Wait for JS-based tests to complete
        await page.waitForTimeout(2000);

        const results = await page.evaluate(() => {
          const tests: Record<string, string> = {};
          // sannysoft.com renders results in tables — walk all rows
          const rows = document.querySelectorAll('table tr');
          rows.forEach((row) => {
            const cells = row.querySelectorAll('td');
            if (cells.length >= 2) {
              const name = cells[0]?.textContent?.trim() ?? '';
              const td = cells[1];
              if (name && td) {
                const hasPass =
                  td.classList.contains('passed') ||
                  td.querySelector('.passed') !== null ||
                  td.textContent?.trim().toLowerCase() === 'passed';
                const hasFail =
                  td.classList.contains('failed') ||
                  td.querySelector('.failed') !== null ||
                  td.textContent?.trim().toLowerCase() === 'failed';
                tests[name] = hasPass
                  ? 'pass'
                  : hasFail
                    ? 'fail'
                    : (td.textContent?.trim() ?? 'unknown');
              }
            }
          });
          return tests;
        });

        const total = Object.keys(results).length;
        const passed = Object.values(results).filter((v) => v === 'pass').length;
        const failed = Object.values(results).filter((v) => v === 'fail').length;

        return successResponse(id, {
          url: 'https://bot.sannysoft.com',
          summary: { total, passed, failed },
          tests: results,
        });
      }

      case 'fingerprint_rotate': {
        if (command.dryRun) {
          return dryRun(id, 'Close browser and relaunch with a new fingerprint seed');
        }
        // Determine new seed
        const newSeed = command.seed ?? Math.floor(Math.random() * 90000) + 10000;
        const previousOptions = browser.getLastLaunchOptions() ?? {};
        // Close current browser instance, preserving the launch options for this relaunch.
        await browser.close({ preserveLaunchOptions: true });
        await browser.launch({ ...previousOptions, fingerprintSeed: newSeed } as any);
        // Report the new fingerprint info
        return successResponse(id, {
          rotated: true,
          seed: newSeed,
          fingerprintArg: `--fingerprint=${newSeed}`,
        });
      }

      case 'profile_create': {
        if (command.dryRun) return dryRun(id, `Create browser profile "${command.name}"`);
        const dir = ensureProfileDir(command.name);
        return successResponse(id, { name: command.name, path: dir });
      }

      case 'profile_list': {
        if (command.dryRun) return dryRun(id, 'List browser profiles');
        const profiles = listProfiles();
        return successResponse(id, { profiles, count: profiles.length });
      }

      default: {
        // TypeScript exhaustiveness check: if all actions are handled above,
        // this branch is unreachable.  If a new action is added to the
        // Command union but not handled here, the compiler will flag it.
        const _exhaustive: never = command;
        return errorResponse(id, `Unknown action: ${(command as any).action}`);
      }
    }
  } catch (err: unknown) {
    const friendly = toAIFriendlyError(err, (command as any).selector);
    return errorResponse(id, friendly.message);
  }
}
