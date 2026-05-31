import type { BrowserLaunchOptions } from './browser.js';
import type { Command } from './protocol.js';

type LaunchCommand = Extract<Command, { action: 'launch' }>;

export function buildLaunchOptions(command: LaunchCommand): BrowserLaunchOptions {
  const options: BrowserLaunchOptions = {};

  if (command.headless !== undefined) options.headless = command.headless;
  if (command.geoip !== undefined) options.geoip = command.geoip;
  if (command.fingerprintSeed !== undefined) options.fingerprintSeed = command.fingerprintSeed;
  if (command.timezone) options.timezone = command.timezone;
  if (command.locale) options.locale = command.locale;
  if (command.platform) options.platform = command.platform;
  if (command.gpuVendor) options.gpuVendor = command.gpuVendor;
  if (command.gpuRenderer) options.gpuRenderer = command.gpuRenderer;
  if (command.proxy) options.proxy = command.proxy;
  if (command.humanize !== undefined) options.humanize = command.humanize;
  if (command.humanPreset) options.humanPreset = command.humanPreset;
  if (command.humanConfig) options.humanConfig = command.humanConfig;
  if (command.contextOptions) options.contextOptions = command.contextOptions;
  if (command.args) options.args = command.args;
  if (command.userAgent) options.userAgent = command.userAgent;
  if (command.viewport) options.viewport = command.viewport;
  if (command.executablePath) options.executablePath = command.executablePath;
  if (command.storageState) options.storageState = command.storageState;
  if (command.ignoreHTTPSErrors !== undefined) options.ignoreHTTPSErrors = command.ignoreHTTPSErrors;
  if (command.profile) options.profile = command.profile;

  return options;
}
