// BrowserManager — lifecycle, snapshots, tabs, and state for CloakBrowser + Playwright.

import type { Browser, BrowserContext, Page, Frame } from 'playwright-core';
import { chromium } from 'playwright-core';
import { launch as cloakLaunch, ensureBinary } from 'cloakbrowser';
import { buildStealthArgs, ensureProfileDir } from './stealth.js';
import { getEnhancedSnapshot, type RefData, type SnapshotOptions } from './snapshot.js';
import { toAIFriendlyError } from './errors.js';
import type { StealthOptions } from './stealth.js';

// ── Launch options ──────────────────────────────────────────────────────────

export interface BrowserLaunchOptions extends StealthOptions {
  headless?: boolean;
  profile?: string;
  viewport?: { width: number; height: number };
  userAgent?: string;
  proxy?: string | { server: string; bypass?: string; username?: string; password?: string };
  args?: string[];
  executablePath?: string;
  storageState?: string;
  ignoreHTTPSErrors?: boolean;
  locale?: string;
  timezone?: string;
}

// ── BrowserManager ──────────────────────────────────────────────────────────

export class BrowserManager {
  private browser: Browser | null = null;
  private contexts: BrowserContext[] = [];
  private pages: Page[] = [];
  private activePageIndex: number = 0;
  private activeFrame: Frame | null = null;
  private refMap: Record<string, RefData> = {};
  private lastSnapshot: string = '';
  private consoleMessages: Array<{ type: string; text: string }> = [];
  private pageErrors: string[] = [];
  private trackedRequests: Array<{ url: string; method: string; status?: number }> = [];
  private routes: Map<string, true> = new Map();
  private isPersistentContext: boolean = false;
  private dialogHandler: ((dialog: any) => void) | null = null;

  // ── Query state ─────────────────────────────────────────────────────────

  isLaunched(): boolean {
    return this.browser !== null || (this.isPersistentContext && this.contexts.length > 0);
  }

  resolveRef(ref: string): RefData | null {
    return this.refMap[ref] ?? null;
  }

  getRefMap(): Record<string, RefData> {
    return { ...this.refMap };
  }

  // ── Launch ──────────────────────────────────────────────────────────────

  async launch(options: BrowserLaunchOptions = {}): Promise<void> {
    const executablePath = options.executablePath ?? await ensureBinary();
    const stealthArgs = [...buildStealthArgs(options), ...(options.args ?? [])];
    const viewport = options.viewport ?? { width: 1920, height: 947 };

    if (options.profile) {
      // Persistent context path — uses chromium.launchPersistentContext directly
      const userDataDir = ensureProfileDir(options.profile);
      this.isPersistentContext = true;

      const launchOptions: Record<string, unknown> = {
        executablePath,
        args: stealthArgs,
        headless: options.headless ?? true,
        viewport,
        userAgent: options.userAgent,
        proxy: typeof options.proxy === 'string' ? { server: options.proxy } : options.proxy,
        ignoreHTTPSErrors: options.ignoreHTTPSErrors ?? false,
        locale: options.locale,
        timezoneId: options.timezone,
      };

      const context = await chromium.launchPersistentContext(userDataDir, launchOptions as any);

      this.contexts.push(context);
      const page = context.pages()[0] ?? await context.newPage();
      this.pages.push(page);
      this.setupPageListeners(page);
    } else {
      // Standard path — use cloakbrowser's launch()
      const browser = await cloakLaunch({
        headless: options.headless ?? true,
        args: stealthArgs,
        executablePath,
        locale: options.locale,
        timezone: options.timezone,
        userAgent: options.userAgent,
        proxy: options.proxy as any,
      } as any);
      this.browser = browser;

      const context = await browser.newContext({
        viewport,
        userAgent: options.userAgent,
        proxy: typeof options.proxy === 'string' ? { server: options.proxy } : options.proxy,
        storageState: options.storageState,
        ignoreHTTPSErrors: options.ignoreHTTPSErrors ?? false,
        locale: options.locale,
      });
      this.contexts.push(context);

      const page = await context.newPage();
      this.pages.push(page);
      this.setupPageListeners(page);
    }
  }

  // ── Page listeners ──────────────────────────────────────────────────────

  private setupPageListeners(page: Page): void {
    page.on('console', (msg) => {
      this.consoleMessages.push({ type: msg.type(), text: msg.text() });
    });

    page.on('pageerror', (error) => {
      this.pageErrors.push(error.message);
    });

    page.on('request', (request) => {
      const entry = { url: request.url(), method: request.method() } as {
        url: string;
        method: string;
        status?: number;
      };
      this.trackedRequests.push(entry);
    });

    page.on('response', (response) => {
      const url = response.url();
      // Update the most recent matching request with status
      for (let i = this.trackedRequests.length - 1; i >= 0; i--) {
        if (this.trackedRequests[i].url === url && this.trackedRequests[i].status === undefined) {
          this.trackedRequests[i].status = response.status();
          break;
        }
      }
    });
  }

  // ── Active page / frame ─────────────────────────────────────────────────

  getPage(): Page {
    const page = this.pages[this.activePageIndex];
    if (!page) {
      throw new Error('No browser page available. Call launch() first.');
    }
    return page;
  }

  getContext(): BrowserContext {
    const context = this.contexts[this.contexts.length - 1];
    if (!context) {
      throw new Error('No browser context available. Call launch() first.');
    }
    return context;
  }

  getFrame(): Frame {
    if (this.activeFrame) {
      return this.activeFrame;
    }
    return this.getPage().mainFrame();
  }

  setActiveFrame(frame: Frame | null): void {
    this.activeFrame = frame;
  }

  // ── Snapshot ────────────────────────────────────────────────────────────

  async getSnapshot(
    options: SnapshotOptions = {},
  ): Promise<{ tree: string; refs: Record<string, RefData> }> {
    const page = this.getPage();
    const result = await getEnhancedSnapshot(page, options);
    this.refMap = result.refs;
    this.lastSnapshot = result.tree;
    return result;
  }

  // ── Locator for ref ─────────────────────────────────────────────────────

  getLocatorForRef(ref: string) {
    const data = this.resolveRef(ref);
    if (!data) {
      throw new Error(`Unknown ref "${ref}". Run 'snapshot' to get updated refs.`);
    }

    const page = this.getPage();
    let locator = page.getByRole(data.role as any, data.name ? { name: data.name, exact: true } : undefined);

    if (data.nth !== undefined) {
      locator = locator.nth(data.nth - 1); // nth in refMap is 1-based, Playwright .nth() is 0-based
    }

    return locator;
  }

  // ── Tab management ──────────────────────────────────────────────────────

  async newTab(url?: string): Promise<Page> {
    const context = this.contexts[this.contexts.length - 1];
    if (!context) {
      throw new Error('No browser context available. Call launch() first.');
    }
    const page = await context.newPage();
    this.pages.push(page);
    this.activePageIndex = this.pages.length - 1;
    this.setupPageListeners(page);

    if (url) {
      await page.goto(url);
    }

    return page;
  }

  getTabList(): Array<{ index: number; url: string; title: string }> {
    return this.pages.map((page, index) => ({
      index,
      url: page.url(),
      title: page.url(), // title() is async, use url as sync fallback
    }));
  }

  async switchTab(index: number): Promise<void> {
    if (index < 0 || index >= this.pages.length) {
      throw new Error(`Tab index ${index} out of range (0-${this.pages.length - 1}).`);
    }
    this.activePageIndex = index;
    this.activeFrame = null;
    await this.pages[index].bringToFront();
  }

  async closeTab(index?: number): Promise<void> {
    const idx = index ?? this.activePageIndex;
    if (idx < 0 || idx >= this.pages.length) {
      throw new Error(`Tab index ${idx} out of range (0-${this.pages.length - 1}).`);
    }

    const page = this.pages[idx];
    await page.close();
    this.pages.splice(idx, 1);

    // Adjust activePageIndex
    if (this.pages.length === 0) {
      this.activePageIndex = 0;
    } else if (this.activePageIndex >= this.pages.length) {
      this.activePageIndex = this.pages.length - 1;
    }
    this.activeFrame = null;
  }

  // ── Diagnostics ─────────────────────────────────────────────────────────

  getConsoleMessages(clear?: boolean): Array<{ type: string; text: string }> {
    const messages = [...this.consoleMessages];
    if (clear) {
      this.consoleMessages = [];
    }
    return messages;
  }

  getPageErrors(clear?: boolean): string[] {
    const errors = [...this.pageErrors];
    if (clear) {
      this.pageErrors = [];
    }
    return errors;
  }

  getTrackedRequests(
    filter?: string,
    clear?: boolean,
  ): Array<{ url: string; method: string; status?: number }> {
    let requests = [...this.trackedRequests];
    if (filter) {
      requests = requests.filter((r) => r.url.includes(filter));
    }
    if (clear) {
      this.trackedRequests = [];
    }
    return requests;
  }

  addRoute(url: string): void {
    this.routes.set(url, true);
  }

  removeRoute(url: string): void {
    this.routes.delete(url);
  }

  getRoutes(): string[] {
    return [...this.routes.keys()];
  }

  clearRoutes(): void {
    this.routes = new Map();
  }

  // ── Teardown ────────────────────────────────────────────────────────────

  async close(): Promise<void> {
    for (const context of this.contexts) {
      try {
        await context.close();
      } catch {
        // ignore — context may already be closed
      }
    }

    if (this.browser) {
      try {
        await this.browser.close();
      } catch {
        // ignore
      }
    }

    // Reset all state
    this.browser = null;
    this.contexts = [];
    this.pages = [];
    this.activePageIndex = 0;
    this.activeFrame = null;
    this.refMap = {};
    this.lastSnapshot = '';
    this.consoleMessages = [];
    this.pageErrors = [];
    this.trackedRequests = [];
    this.routes = new Map();
    this.isPersistentContext = false;
    this.dialogHandler = null;
  }
}
