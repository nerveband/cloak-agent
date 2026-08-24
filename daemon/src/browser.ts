// BrowserManager — lifecycle, snapshots, tabs, and state for CloakBrowser + Playwright.

import type { Browser, BrowserContext, BrowserContextOptions, CDPSession, Page, Frame, Dialog } from 'playwright-core';
import { launchContext, launchPersistentContext } from 'cloakbrowser';
import { buildStealthArgs, ensureProfileDir } from './stealth.js';
import { getEnhancedSnapshot, type RefData, type SnapshotOptions } from './snapshot.js';
import { toAIFriendlyError } from './errors.js';
import type { StealthOptions } from './stealth.js';
import { prepareCATrust, type PreparedCATrust } from './ca-trust.js';

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
  browserVersion?: string;
  releaseChannel?: 'stable' | 'preview';
  extensionPaths?: string[];
  humanize?: boolean;
  humanPreset?: 'default' | 'careful';
  humanConfig?: Record<string, unknown>;
  contextOptions?: BrowserContextOptions;
  caCert?: string;
  clearCaCert?: boolean;
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

export interface RuntimeStatus {
  launched: boolean;
  activePageIndex: number | null;
  activePage?: {
    url: string;
    title: string;
  };
  tabs: Array<{ index: number; url: string; title: string }>;
  contextCount: number;
}

export interface TrackedRequest {
  id: string;
  url: string;
  method: string;
  resourceType: string;
  requestHeaders: Record<string, string>;
  postData?: string | null;
  status?: number;
  responseHeaders?: Record<string, string>;
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
  private trackedRequests: TrackedRequest[] = [];
  private nextRequestId = 1;
  private routes: Map<string, true> = new Map();
  private isPersistentContext: boolean = false;
  private dialogHandler: ((dialog: any) => void) | null = null;
  private lastLaunchOptions: BrowserLaunchOptions | null = null;
  private screencastSession: CDPSession | null = null;
  private profilerSession: CDPSession | null = null;
  private pendingDialog: Dialog | null = null;
  private tabIds = new Map<Page, string>();
  private tabLabels = new Map<string, Page>();
  private nextTabId = 1;
  private preparedCATrust: PreparedCATrust | null = null;

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
    if (options.caCert && options.clearCaCert) {
      throw new Error('Cannot use --ca-cert with --no-ca-cert');
    }
    if (options.caCert && options.profile) {
      throw new Error('--ca-cert cannot be combined with --profile because isolated CA trust changes the profile NSS environment');
    }
    if (options.caCert && options.ignoreHTTPSErrors) {
      throw new Error('--ca-cert cannot be combined with --ignore-https-errors');
    }
    if (this.isLaunched()) {
      await this.close({ preserveLaunchOptions: true });
    }

    let preparedCATrust: PreparedCATrust | null = null;
    try {
      preparedCATrust = options.caCert ? await prepareCATrust(options.caCert) : null;
      const args = buildStealthArgs(options);
      const viewport = options.viewport ?? { width: 1920, height: 947 };
      const contextOptions: BrowserContextOptions = {
        ...(options.contextOptions ?? {}),
        ...(options.storageState ? { storageState: options.storageState } : {}),
        ...(options.ignoreHTTPSErrors !== undefined ? { ignoreHTTPSErrors: options.ignoreHTTPSErrors } : {}),
      };
      const launchEnv = preparedCATrust
        ? {
            ...process.env,
            HOME: preparedCATrust.homeDir,
            XDG_DATA_HOME: `${preparedCATrust.homeDir}/.local/share`,
          }
        : undefined;

      const baseOptions: Record<string, unknown> = {
        headless: options.headless ?? true,
        args,
        locale: options.locale,
        timezone: options.timezone,
        userAgent: options.userAgent,
        viewport,
        proxy: options.proxy,
        geoip: options.geoip,
        browserVersion: options.browserVersion,
        releaseChannel: options.releaseChannel,
        extensionPaths: options.extensionPaths,
        humanize: options.humanize,
        humanPreset: options.humanPreset,
        humanConfig: options.humanConfig,
        contextOptions,
        launchOptions: options.executablePath || launchEnv
          ? { executablePath: options.executablePath, env: launchEnv }
          : undefined,
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
        this.registerPage(page);
      } else {
        const context = await launchContext(baseOptions as any);
        this.contexts.push(context);
        const page = await context.newPage();
        this.registerPage(page);
      }

      for (const context of this.contexts) {
        context.on('page', (page) => this.registerPage(page));
      }
      this.preparedCATrust = preparedCATrust;
      this.lastLaunchOptions = { ...options, clearCaCert: undefined };
    } catch (error) {
      await preparedCATrust?.cleanup();
      throw error;
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
      const entry: TrackedRequest = {
        id: `r${this.nextRequestId++}`,
        url: request.url(),
        method: request.method(),
        resourceType: request.resourceType(),
        requestHeaders: request.headers(),
        postData: request.postData(),
      };
      this.trackedRequests.push(entry);
    });

    page.on('response', (response) => {
      const url = response.url();
      // Update the most recent matching request with status
      for (let i = this.trackedRequests.length - 1; i >= 0; i--) {
        if (this.trackedRequests[i].url === url && this.trackedRequests[i].status === undefined) {
          this.trackedRequests[i].status = response.status();
          this.trackedRequests[i].responseHeaders = response.headers();
          break;
        }
      }
    });

    page.on('dialog', async (dialog) => {
      if (dialog.type() === 'alert' || dialog.type() === 'beforeunload') {
        await dialog.accept().catch(() => {});
        return;
      }
      this.pendingDialog = dialog;
    });
  }

  private registerPage(page: Page): void {
    if (this.pages.includes(page)) return;
    this.pages.push(page);
    this.tabIds.set(page, `t${this.nextTabId++}`);
    this.setupPageListeners(page);
  }

  // ── Active page / frame ─────────────────────────────────────────────────

  getPage(): Page {
    this.pruneClosedPages();
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

  async newTab(url?: string, label?: string): Promise<Page> {
    const context = this.contexts[this.contexts.length - 1];
    if (!context) {
      throw new Error('No browser context available. Call launch() first.');
    }
    if (label && this.tabLabels.has(label)) {
      throw new Error(`Tab label "${label}" is already in use.`);
    }
    const page = await context.newPage();
    this.registerPage(page);
    this.activePageIndex = this.pages.length - 1;
    if (label) {
      this.tabLabels.set(label, page);
    }

    if (url) {
      await page.goto(url);
    }

    return page;
  }

  getTabList(): Array<{ index: number; tabId: string; label?: string; url: string; title: string }> {
    this.pruneClosedPages();
    return this.pages.map((page, index) => ({
      index,
      tabId: this.tabIds.get(page) ?? `t${index + 1}`,
      label: [...this.tabLabels.entries()].find(([, labelledPage]) => labelledPage === page)?.[0],
      url: page.url(),
      title: page.url(), // title() is async, use url as sync fallback
    }));
  }

  async switchTab(index: number): Promise<void> {
    this.pruneClosedPages();
    if (index < 0 || index >= this.pages.length) {
      throw new Error(`Tab index ${index} out of range (0-${this.pages.length - 1}).`);
    }
    this.activePageIndex = index;
    this.activeFrame = null;
    await this.pages[index].bringToFront();
  }

  async switchTabTarget(target: string): Promise<void> {
    this.pruneClosedPages();
    const page = target.startsWith('t')
      ? this.pages.find((candidate) => this.tabIds.get(candidate) === target)
      : this.tabLabels.get(target);
    if (!page || page.isClosed()) throw new Error(`Unknown tab "${target}". Run 'tab' to list tabs.`);
    this.activePageIndex = this.pages.indexOf(page);
    this.activeFrame = null;
    await page.bringToFront();
  }

  async closeTab(index?: number): Promise<void> {
    this.pruneClosedPages();
    const idx = index ?? this.activePageIndex;
    if (idx < 0 || idx >= this.pages.length) {
      throw new Error(`Tab index ${idx} out of range (0-${this.pages.length - 1}).`);
    }

    const page = this.pages[idx];
    await page.close();
    this.pages.splice(idx, 1);
    this.tabIds.delete(page);
    for (const [label, labelledPage] of this.tabLabels) {
      if (labelledPage === page) this.tabLabels.delete(label);
    }

    // Adjust activePageIndex
    if (this.pages.length === 0) {
      this.activePageIndex = 0;
    } else if (this.activePageIndex >= this.pages.length) {
      this.activePageIndex = this.pages.length - 1;
    }
    this.activeFrame = null;
  }

  async closeTabTarget(target: string): Promise<void> {
    const page = target.startsWith('t')
      ? this.pages.find((candidate) => this.tabIds.get(candidate) === target)
      : this.tabLabels.get(target);
    if (!page) throw new Error(`Unknown tab "${target}". Run 'tab' to list tabs.`);
    await this.closeTab(this.pages.indexOf(page));
  }

  getDialogStatus(): { open: boolean; type?: string; message?: string } {
    return this.pendingDialog
      ? { open: true, type: this.pendingDialog.type(), message: this.pendingDialog.message() }
      : { open: false };
  }

  async handleDialog(accept: boolean, promptText?: string): Promise<void> {
    const dialog = this.pendingDialog;
    if (!dialog) throw new Error('No dialog is currently open.');
    this.pendingDialog = null;
    if (accept) await dialog.accept(promptText);
    else await dialog.dismiss();
  }

  async getRuntimeStatus(): Promise<RuntimeStatus> {
    this.pruneClosedPages();
    const tabs = await Promise.all(this.pages.map(async (page, index) => ({
      index,
      url: page.url(),
      title: await page.title().catch(() => page.url()),
    })));
    const activePage = this.pages[this.activePageIndex];

    return {
      launched: this.isLaunched(),
      activePageIndex: activePage ? this.activePageIndex : null,
      activePage: activePage
        ? {
            url: activePage.url(),
            title: await activePage.title().catch(() => activePage.url()),
          }
        : undefined,
      tabs,
      contextCount: this.contexts.length,
    };
  }

  async sendCDP(method: string, params: Record<string, unknown> = {}): Promise<unknown> {
    const session = await this.createInputSession();
    return session.send(method as any, params as any);
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
  ): TrackedRequest[] {
    let requests = [...this.trackedRequests];
    if (filter) {
      requests = requests.filter((r) => r.url.includes(filter));
    }
    if (clear) {
      this.trackedRequests = [];
    }
    return requests;
  }

  getRequest(id: string): TrackedRequest | undefined {
    return this.trackedRequests.find((request) => request.id === id);
  }

  async startProfiler(): Promise<void> {
    if (this.profilerSession) throw new Error('Profiler is already running.');
    const session = await this.createInputSession();
    await session.send('Profiler.enable');
    await session.send('Profiler.start');
    this.profilerSession = session;
  }

  async stopProfiler(): Promise<unknown> {
    const session = this.profilerSession;
    if (!session) throw new Error('Profiler is not running.');
    this.profilerSession = null;
    const result = await session.send('Profiler.stop');
    await session.send('Profiler.disable').catch(() => {});
    await session.detach().catch(() => {});
    return result.profile;
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

  private pruneClosedPages(): void {
    const activePage = this.pages[this.activePageIndex];
    this.pages = this.pages.filter((page) => !page.isClosed());
    if (activePage && !activePage.isClosed()) {
      this.activePageIndex = this.pages.indexOf(activePage);
    }
    if (this.activePageIndex < 0) {
      this.activePageIndex = 0;
    }
    if (this.activePageIndex >= this.pages.length) {
      this.activePageIndex = Math.max(0, this.pages.length - 1);
    }
    if (this.pages.length === 0) {
      this.activeFrame = null;
    }
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
    await this.preparedCATrust?.cleanup();
    this.preparedCATrust = null;

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
    this.nextRequestId = 1;
    this.routes = new Map();
    this.isPersistentContext = false;
    this.dialogHandler = null;
    this.pendingDialog = null;
    this.tabIds = new Map();
    this.tabLabels = new Map();
    this.nextTabId = 1;
    if (!options.preserveLaunchOptions) {
      this.lastLaunchOptions = null;
    }
  }
}
