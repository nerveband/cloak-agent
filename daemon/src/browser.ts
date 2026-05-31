// BrowserManager — lifecycle, snapshots, tabs, and state for CloakBrowser + Playwright.

import type { Browser, BrowserContext, BrowserContextOptions, CDPSession, Page, Frame } from 'playwright-core';
import { launchContext, launchPersistentContext } from 'cloakbrowser';
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
  geoip?: boolean;
  humanize?: boolean;
  humanPreset?: 'default' | 'careful';
  humanConfig?: Record<string, unknown>;
  contextOptions?: BrowserContextOptions;
}

export interface StreamMouseInput {
  action: 'move' | 'down' | 'up' | 'click';
  x: number;
  y: number;
  button?: 'left' | 'right' | 'middle';
}

export interface StreamKeyboardInput {
  action: 'keydown' | 'keyup' | 'press';
  key: string;
  modifiers?: string[];
}

export interface StreamTouchInput {
  action: 'start' | 'move' | 'end' | 'cancel';
  x: number;
  y: number;
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
  private lastLaunchOptions: BrowserLaunchOptions | null = null;
  private screencastSession: CDPSession | null = null;

  // ── Query state ─────────────────────────────────────────────────────────

  isLaunched(): boolean {
    return this.browser !== null || this.contexts.length > 0;
  }

  resolveRef(ref: string): RefData | null {
    return this.refMap[ref] ?? null;
  }

  getRefMap(): Record<string, RefData> {
    return { ...this.refMap };
  }

  getLastLaunchOptions(): BrowserLaunchOptions | null {
    return this.lastLaunchOptions ? { ...this.lastLaunchOptions } : null;
  }

  // ── Launch ──────────────────────────────────────────────────────────────

  async launch(options: BrowserLaunchOptions = {}): Promise<void> {
    if (this.isLaunched()) {
      await this.close({ preserveLaunchOptions: true });
    }

    const explicitStealthArgs = buildStealthArgs(options);
    const args = [...explicitStealthArgs, ...(options.args ?? [])];
    const viewport = options.viewport ?? { width: 1920, height: 947 };
    const contextOptions: BrowserContextOptions = {
      ...(options.contextOptions ?? {}),
      ...(options.storageState ? { storageState: options.storageState } : {}),
      ...(options.ignoreHTTPSErrors !== undefined ? { ignoreHTTPSErrors: options.ignoreHTTPSErrors } : {}),
    };

    const baseOptions: Record<string, unknown> = {
      headless: options.headless ?? true,
      args,
      locale: options.locale,
      timezone: options.timezone,
      userAgent: options.userAgent,
      viewport,
      proxy: options.proxy as any,
      geoip: options.geoip,
      humanize: options.humanize,
      humanPreset: options.humanPreset,
      humanConfig: options.humanConfig,
      contextOptions,
      launchOptions: options.executablePath ? { executablePath: options.executablePath } : undefined,
    };

    if (options.profile) {
      const userDataDir = ensureProfileDir(options.profile);
      this.isPersistentContext = true;

      const context = await launchPersistentContext({
        ...baseOptions,
        userDataDir,
      } as any);
      this.contexts.push(context);
      const page = context.pages()[0] ?? await context.newPage();
      this.pages.push(page);
      this.setupPageListeners(page);
    } else {
      const context = await launchContext(baseOptions as any);
      this.contexts.push(context);

      const page = await context.newPage();
      this.pages.push(page);
      this.setupPageListeners(page);
    }

    this.lastLaunchOptions = { ...options };
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

  // ── CDP streaming / remote input ────────────────────────────────────────

  async startScreencast(onFrame: (data: string) => void): Promise<void> {
    if (this.screencastSession) {
      return;
    }

    const page = this.getPage();
    const session = await page.context().newCDPSession(page);
    session.on('Page.screencastFrame', (params: { data: string; sessionId: number }) => {
      onFrame(params.data);
      session.send('Page.screencastFrameAck', { sessionId: params.sessionId }).catch(() => {});
    });

    await session.send('Page.startScreencast', {
      format: 'jpeg',
      quality: 60,
      maxWidth: page.viewportSize()?.width ?? 1920,
      maxHeight: page.viewportSize()?.height ?? 947,
    });
    this.screencastSession = session;
  }

  async stopScreencast(): Promise<void> {
    const session = this.screencastSession;
    if (!session) {
      return;
    }

    this.screencastSession = null;
    try {
      await session.send('Page.stopScreencast');
    } finally {
      await session.detach().catch(() => {});
    }
  }

  async injectMouseEvent(message: StreamMouseInput): Promise<void> {
    const session = await this.createInputSession();
    const button = message.button ?? 'left';
    const base = { x: message.x, y: message.y, button };

    switch (message.action) {
      case 'move':
        await session.send('Input.dispatchMouseEvent', { ...base, type: 'mouseMoved', button: 'none' });
        return;
      case 'down':
        await session.send('Input.dispatchMouseEvent', { ...base, type: 'mousePressed', clickCount: 1 });
        return;
      case 'up':
        await session.send('Input.dispatchMouseEvent', { ...base, type: 'mouseReleased', clickCount: 1 });
        return;
      case 'click':
        await session.send('Input.dispatchMouseEvent', { ...base, type: 'mousePressed', clickCount: 1 });
        await session.send('Input.dispatchMouseEvent', { ...base, type: 'mouseReleased', clickCount: 1 });
        return;
    }
  }

  async injectKeyboardEvent(message: StreamKeyboardInput): Promise<void> {
    const session = await this.createInputSession();
    const payload = { key: message.key, modifiers: this.encodeModifiers(message.modifiers) };

    switch (message.action) {
      case 'keydown':
        await session.send('Input.dispatchKeyEvent', { ...payload, type: 'keyDown' });
        return;
      case 'keyup':
        await session.send('Input.dispatchKeyEvent', { ...payload, type: 'keyUp' });
        return;
      case 'press':
        await session.send('Input.dispatchKeyEvent', { ...payload, type: 'keyDown' });
        await session.send('Input.dispatchKeyEvent', { ...payload, type: 'keyUp' });
        return;
    }
  }

  async injectTouchEvent(message: StreamTouchInput): Promise<void> {
    const session = await this.createInputSession();
    const typeByAction = {
      start: 'touchStart',
      move: 'touchMove',
      end: 'touchEnd',
      cancel: 'touchCancel',
    } as const;
    const touchPoints = message.action === 'end' || message.action === 'cancel'
      ? []
      : [{ x: message.x, y: message.y }];

    await session.send('Input.dispatchTouchEvent', {
      type: typeByAction[message.action],
      touchPoints,
    });
  }

  private async createInputSession(): Promise<CDPSession> {
    const page = this.getPage();
    return page.context().newCDPSession(page);
  }

  private encodeModifiers(modifiers: string[] = []): number {
    const bits: Record<string, number> = {
      Alt: 1,
      Control: 2,
      Meta: 4,
      Shift: 8,
    };
    return modifiers.reduce((mask, modifier) => mask | (bits[modifier] ?? 0), 0);
  }

  // ── Teardown ────────────────────────────────────────────────────────────

  async close(options: { preserveLaunchOptions?: boolean } = {}): Promise<void> {
    await this.stopScreencast();

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
    if (!options.preserveLaunchOptions) {
      this.lastLaunchOptions = null;
    }
  }
}
