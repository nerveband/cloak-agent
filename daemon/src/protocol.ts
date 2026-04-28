import { z } from 'zod';

// ---------------------------------------------------------------------------
// Base fields shared by every command
// ---------------------------------------------------------------------------
const base = {
  id: z.string(),
  dryRun: z.boolean().optional(),
};

// ---------------------------------------------------------------------------
// Reusable option enums / shapes
// ---------------------------------------------------------------------------
const waitUntilEnum = z.enum(['load', 'domcontentloaded', 'networkidle']).optional();
const modifiers = z.array(z.enum(['Alt', 'Control', 'Meta', 'Shift'])).optional();
const buttonEnum = z.enum(['left', 'right', 'middle']).optional();
const positionObj = z.object({ x: z.number(), y: z.number() }).optional();
const viewportObj = z.object({ width: z.number(), height: z.number() });
const looseRecord = z.record(z.unknown());
const semanticSubactionEnum = z.enum([
  'count',
  'click',
  'dblclick',
  'fill',
  'type',
  'hover',
  'focus',
  'check',
  'uncheck',
  'select',
]).optional();

const proxySchema = z.union([
  z.string(),
  z.object({
    server: z.string(),
    bypass: z.string().optional(),
    username: z.string().optional(),
    password: z.string().optional(),
  }),
]);

// ---------------------------------------------------------------------------
// Individual command schemas
// ---------------------------------------------------------------------------

// --- Core navigation ---
const launch = z.object({
  ...base,
  action: z.literal('launch'),
  headless: z.boolean().optional(),
  url: z.string().optional(),
  // CloakBrowser stealth options
  geoip: z.boolean().optional(),
  fingerprintSeed: z.number().optional(),
  timezone: z.string().optional(),
  locale: z.string().optional(),
  platform: z.enum(['windows', 'macos', 'linux']).optional(),
  gpuVendor: z.string().optional(),
  gpuRenderer: z.string().optional(),
  proxy: proxySchema.optional(),
  profile: z.string().optional(),
  humanize: z.boolean().optional(),
  humanPreset: z.enum(['default', 'careful']).optional(),
  humanConfig: looseRecord.optional(),
  userAgent: z.string().optional(),
  viewport: viewportObj.optional(),
  args: z.array(z.string()).optional(),
  executablePath: z.string().optional(),
  storageState: z.string().optional(),
  ignoreHTTPSErrors: z.boolean().optional(),
  contextOptions: looseRecord.optional(),
});

const navigate = z.object({
  ...base,
  action: z.literal('navigate'),
  url: z.string(),
  waitUntil: waitUntilEnum,
});

const back = z.object({ ...base, action: z.literal('back'), waitUntil: waitUntilEnum });
const forward = z.object({ ...base, action: z.literal('forward'), waitUntil: waitUntilEnum });
const reload = z.object({ ...base, action: z.literal('reload'), waitUntil: waitUntilEnum });
const close = z.object({ ...base, action: z.literal('close') });
const url = z.object({ ...base, action: z.literal('url') });
const title = z.object({ ...base, action: z.literal('title') });

// --- Interactions ---
const click = z.object({
  ...base,
  action: z.literal('click'),
  selector: z.string(),
  button: buttonEnum,
  clickCount: z.number().optional(),
  modifiers,
  position: positionObj,
  force: z.boolean().optional(),
});

const fill = z.object({ ...base, action: z.literal('fill'), selector: z.string(), value: z.string() });
const type_ = z.object({ ...base, action: z.literal('type'), selector: z.string(), text: z.string(), delay: z.number().optional() });
const check = z.object({ ...base, action: z.literal('check'), selector: z.string(), force: z.boolean().optional() });
const uncheck = z.object({ ...base, action: z.literal('uncheck'), selector: z.string(), force: z.boolean().optional() });
const hover = z.object({ ...base, action: z.literal('hover'), selector: z.string(), position: positionObj, modifiers, force: z.boolean().optional() });
const focus = z.object({ ...base, action: z.literal('focus'), selector: z.string() });
const dblclick = z.object({ ...base, action: z.literal('dblclick'), selector: z.string(), button: buttonEnum, modifiers, position: positionObj, force: z.boolean().optional() });
const select = z.object({ ...base, action: z.literal('select'), selector: z.string(), values: z.array(z.string()) });
const upload = z.object({ ...base, action: z.literal('upload'), selector: z.string(), files: z.array(z.string()) });
const drag = z.object({ ...base, action: z.literal('drag'), source: z.string(), target: z.string(), force: z.boolean().optional() });
const press = z.object({ ...base, action: z.literal('press'), key: z.string(), selector: z.string().optional(), delay: z.number().optional() });
const keydown = z.object({ ...base, action: z.literal('keydown'), key: z.string() });
const keyup = z.object({ ...base, action: z.literal('keyup'), key: z.string() });

// --- Snapshot ---
const snapshot = z.object({
  ...base,
  action: z.literal('snapshot'),
  interactive: z.boolean().optional(),
  maxDepth: z.number().optional(),
  compact: z.boolean().optional(),
  selector: z.string().optional(),
});

// --- Screenshot / PDF ---
const screenshot = z.object({
  ...base,
  action: z.literal('screenshot'),
  selector: z.string().optional(),
  fullPage: z.boolean().optional(),
  path: z.string().optional(),
  quality: z.number().optional(),
  type: z.enum(['png', 'jpeg']).optional(),
});

const pdf = z.object({
  ...base,
  action: z.literal('pdf'),
  path: z.string().optional(),
  format: z.string().optional(),
  landscape: z.boolean().optional(),
});

// --- Evaluate ---
const evaluate = z.object({
  ...base,
  action: z.literal('evaluate'),
  expression: z.string(),
});

// --- Wait ---
const wait = z.object({ ...base, action: z.literal('wait'), timeout: z.number().optional(), selector: z.string().optional(), state: z.enum(['attached', 'detached', 'visible', 'hidden']).optional() });
const waitforurl = z.object({ ...base, action: z.literal('waitforurl'), url: z.string(), timeout: z.number().optional() });
const waitforloadstate = z.object({ ...base, action: z.literal('waitforloadstate'), state: z.enum(['load', 'domcontentloaded', 'networkidle']).optional(), timeout: z.number().optional() });
const waitforfunction = z.object({ ...base, action: z.literal('waitforfunction'), expression: z.string(), timeout: z.number().optional() });

// --- Scroll ---
const scroll = z.object({ ...base, action: z.literal('scroll'), x: z.number().optional(), y: z.number().optional(), selector: z.string().optional() });
const scrollintoview = z.object({ ...base, action: z.literal('scrollintoview'), selector: z.string() });

// --- Element info ---
const gettext = z.object({ ...base, action: z.literal('gettext'), selector: z.string() });
const innerhtml = z.object({ ...base, action: z.literal('innerhtml'), selector: z.string() });
const inputvalue = z.object({ ...base, action: z.literal('inputvalue'), selector: z.string() });
const getattribute = z.object({ ...base, action: z.literal('getattribute'), selector: z.string(), name: z.string() });
const isvisible = z.object({ ...base, action: z.literal('isvisible'), selector: z.string() });
const isenabled = z.object({ ...base, action: z.literal('isenabled'), selector: z.string() });
const ischecked = z.object({ ...base, action: z.literal('ischecked'), selector: z.string() });
const count = z.object({ ...base, action: z.literal('count'), selector: z.string() });
const boundingbox = z.object({ ...base, action: z.literal('boundingbox'), selector: z.string() });

// --- Tabs ---
const tab_new = z.object({ ...base, action: z.literal('tab_new'), url: z.string().optional() });
const tab_list = z.object({ ...base, action: z.literal('tab_list') });
const tab_switch = z.object({ ...base, action: z.literal('tab_switch'), index: z.number() });
const tab_close = z.object({ ...base, action: z.literal('tab_close'), index: z.number().optional() });

// --- Cookies / Storage ---
const cookies_get = z.object({ ...base, action: z.literal('cookies_get'), urls: z.array(z.string()).optional() });
const cookies_set = z.object({ ...base, action: z.literal('cookies_set'), cookies: z.array(z.object({ name: z.string(), value: z.string(), url: z.string().optional(), domain: z.string().optional(), path: z.string().optional() })) });
const cookies_clear = z.object({ ...base, action: z.literal('cookies_clear') });
const storage_get = z.object({ ...base, action: z.literal('storage_get'), key: z.string().optional() });
const storage_set = z.object({ ...base, action: z.literal('storage_set'), key: z.string(), value: z.string() });
const storage_clear = z.object({ ...base, action: z.literal('storage_clear') });

// --- Dialog ---
const dialog = z.object({ ...base, action: z.literal('dialog'), accept: z.boolean().optional(), promptText: z.string().optional() });

// --- Network ---
const route = z.object({ ...base, action: z.literal('route'), url: z.string(), handler: z.enum(['abort', 'continue', 'fulfill']).optional(), body: z.string().optional(), status: z.number().optional() });
const unroute = z.object({ ...base, action: z.literal('unroute'), url: z.string().optional() });
const requests = z.object({ ...base, action: z.literal('requests'), filter: z.string().optional(), limit: z.number().optional() });

// --- Settings ---
const viewport = z.object({ ...base, action: z.literal('viewport'), width: z.number(), height: z.number() });
const device = z.object({ ...base, action: z.literal('device'), name: z.string() });
const geolocation = z.object({ ...base, action: z.literal('geolocation'), latitude: z.number(), longitude: z.number(), accuracy: z.number().optional() });
const headers = z.object({ ...base, action: z.literal('headers'), headers: z.record(z.string()) });
const credentials = z.object({ ...base, action: z.literal('credentials'), username: z.string(), password: z.string() });
const offline = z.object({ ...base, action: z.literal('offline'), enabled: z.boolean() });
const emulatemedia = z.object({ ...base, action: z.literal('emulatemedia'), media: z.enum(['screen', 'print', 'null']).optional(), colorScheme: z.enum(['light', 'dark', 'no-preference', 'null']).optional() });

// --- State ---
const state_save = z.object({ ...base, action: z.literal('state_save'), path: z.string() });
const state_load = z.object({ ...base, action: z.literal('state_load'), path: z.string() });

// --- Debug ---
const console_ = z.object({ ...base, action: z.literal('console'), limit: z.number().optional(), level: z.string().optional(), clear: z.boolean().optional() });
const errors = z.object({ ...base, action: z.literal('errors'), limit: z.number().optional(), clear: z.boolean().optional() });
const highlight = z.object({ ...base, action: z.literal('highlight'), selector: z.string(), color: z.string().optional(), duration: z.number().optional() });

// --- Trace / Recording ---
const trace_start = z.object({ ...base, action: z.literal('trace_start'), path: z.string().optional(), screenshots: z.boolean().optional(), snapshots: z.boolean().optional() });
const trace_stop = z.object({ ...base, action: z.literal('trace_stop'), path: z.string().optional() });
const recording_start = z.object({ ...base, action: z.literal('recording_start'), path: z.string().optional() });
const recording_stop = z.object({ ...base, action: z.literal('recording_stop') });

// --- Semantic locators ---
const getbyrole = z.object({ ...base, action: z.literal('getbyrole'), role: z.string(), name: z.string().optional(), exact: z.boolean().optional(), subaction: semanticSubactionEnum, value: z.string().optional() });
const getbytext = z.object({ ...base, action: z.literal('getbytext'), text: z.string(), exact: z.boolean().optional(), subaction: semanticSubactionEnum, value: z.string().optional() });
const getbylabel = z.object({ ...base, action: z.literal('getbylabel'), text: z.string(), exact: z.boolean().optional(), subaction: semanticSubactionEnum, value: z.string().optional() });

// --- Mouse ---
const mousemove = z.object({ ...base, action: z.literal('mousemove'), x: z.number(), y: z.number(), steps: z.number().optional() });
const mousedown = z.object({ ...base, action: z.literal('mousedown'), button: buttonEnum });
const mouseup = z.object({ ...base, action: z.literal('mouseup'), button: buttonEnum });
const wheel = z.object({ ...base, action: z.literal('wheel'), deltaX: z.number().optional(), deltaY: z.number() });

// --- Schema introspection ---
const schema = z.object({ ...base, action: z.literal('schema'), command: z.string().optional(), all: z.boolean().optional() });

// --- Cloak-Agent exclusive ---
const stealth_status = z.object({ ...base, action: z.literal('stealth_status') });
const fingerprint_rotate = z.object({ ...base, action: z.literal('fingerprint_rotate'), seed: z.number().optional() });
const profile_create = z.object({ ...base, action: z.literal('profile_create'), name: z.string() });
const profile_list = z.object({ ...base, action: z.literal('profile_list') });

// ---------------------------------------------------------------------------
// Discriminated union of ALL commands
// ---------------------------------------------------------------------------
const allSchemas = [
  // Core navigation
  launch, navigate, back, forward, reload, close, url, title,
  // Interactions
  click, fill, type_, check, uncheck, hover, focus, dblclick, select, upload, drag, press, keydown, keyup,
  // Snapshot
  snapshot,
  // Screenshot / PDF
  screenshot, pdf,
  // Evaluate
  evaluate,
  // Wait
  wait, waitforurl, waitforloadstate, waitforfunction,
  // Scroll
  scroll, scrollintoview,
  // Element info
  gettext, innerhtml, inputvalue, getattribute, isvisible, isenabled, ischecked, count, boundingbox,
  // Tabs
  tab_new, tab_list, tab_switch, tab_close,
  // Cookies / Storage
  cookies_get, cookies_set, cookies_clear, storage_get, storage_set, storage_clear,
  // Dialog
  dialog,
  // Network
  route, unroute, requests,
  // Settings
  viewport, device, geolocation, headers, credentials, offline, emulatemedia,
  // State
  state_save, state_load,
  // Debug
  console_, errors, highlight,
  // Trace / Recording
  trace_start, trace_stop, recording_start, recording_stop,
  // Semantic locators
  getbyrole, getbytext, getbylabel,
  // Mouse
  mousemove, mousedown, mouseup, wheel,
  // Schema introspection
  schema,
  // Cloak-Agent exclusive
  stealth_status, fingerprint_rotate, profile_create, profile_list,
] as const;

export const CommandSchema = z.discriminatedUnion('action', [...allSchemas]);

// ---------------------------------------------------------------------------
// Lookup map: action name -> individual Zod schema
// ---------------------------------------------------------------------------
const schemaMap: Record<string, z.ZodObject<any>> = {};
for (const s of allSchemas) {
  const actionLiteral = (s.shape as any).action;
  if (actionLiteral && '_def' in actionLiteral && actionLiteral._def.value) {
    schemaMap[actionLiteral._def.value] = s as z.ZodObject<any>;
  }
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------
export type Command = z.infer<typeof CommandSchema>;

export interface SuccessResponse {
  id: string;
  ok: true;
  data: unknown;
}

export interface ErrorResponse {
  id: string;
  ok: false;
  error: string;
}

export type Response = SuccessResponse | ErrorResponse;

export interface ParseSuccess {
  ok: true;
  command: Command;
}

export interface ParseFailure {
  ok: false;
  error: string;
}

export type ParseResult = ParseSuccess | ParseFailure;

// ---------------------------------------------------------------------------
// Functions
// ---------------------------------------------------------------------------

/**
 * Parse a raw JSON string into a validated Command.
 */
export function parseCommand(input: string): ParseResult {
  let parsed: unknown;
  try {
    parsed = JSON.parse(input);
  } catch {
    return { ok: false, error: 'Invalid JSON' };
  }

  const result = CommandSchema.safeParse(parsed);
  if (result.success) {
    return { ok: true, command: result.data };
  }

  // Build a human-friendly error string from Zod issues
  const messages = result.error.issues.map((i) => {
    const path = i.path.length > 0 ? i.path.join('.') + ': ' : '';
    return path + i.message;
  });
  return { ok: false, error: messages.join('; ') };
}

/**
 * Build a success response envelope.
 */
export function successResponse(id: string, data: unknown): SuccessResponse {
  return { id, ok: true, data };
}

/**
 * Build an error response envelope.
 */
export function errorResponse(id: string, error: string): ErrorResponse {
  return { id, ok: false, error };
}

/**
 * Serialize a Response to a JSON string (one line, for socket transport).
 */
export function serializeResponse(response: Response): string {
  return JSON.stringify(response);
}

// ---------------------------------------------------------------------------
// Schema introspection helpers
// ---------------------------------------------------------------------------

/**
 * Simplify a single ZodType into a { type, required?, values? } descriptor.
 */
function describeType(zodType: z.ZodTypeAny): { type: string; values?: string[] } {
  // Unwrap optionals / defaults
  if (zodType instanceof z.ZodOptional || zodType instanceof z.ZodDefault) {
    return describeType((zodType as any)._def.innerType);
  }
  if (zodType instanceof z.ZodNullable) {
    return describeType((zodType as any)._def.innerType);
  }

  if (zodType instanceof z.ZodString) return { type: 'string' };
  if (zodType instanceof z.ZodNumber) return { type: 'number' };
  if (zodType instanceof z.ZodBoolean) return { type: 'boolean' };
  if (zodType instanceof z.ZodLiteral) return { type: 'string' }; // action literals
  if (zodType instanceof z.ZodEnum) {
    return { type: 'enum', values: (zodType as any)._def.values as string[] };
  }
  if (zodType instanceof z.ZodArray) return { type: 'array' };
  if (zodType instanceof z.ZodObject) return { type: 'object' };
  if (zodType instanceof z.ZodRecord) return { type: 'record' };
  if (zodType instanceof z.ZodUnion || zodType instanceof z.ZodDiscriminatedUnion) return { type: 'union' };

  return { type: 'unknown' };
}

function isOptional(zodType: z.ZodTypeAny): boolean {
  if (zodType instanceof z.ZodOptional || zodType instanceof z.ZodDefault) return true;
  return false;
}

/**
 * Return a simplified JSON representation of a single action's schema shape.
 * Returns null if the action is unknown.
 */
export function dumpSchema(action: string): Record<string, any> | null {
  const s = schemaMap[action];
  if (!s) return null;

  const shape = s.shape as Record<string, z.ZodTypeAny>;
  const result: Record<string, any> = {};

  for (const [key, zodType] of Object.entries(shape)) {
    const desc = describeType(zodType);
    const entry: Record<string, any> = {
      type: desc.type,
      required: !isOptional(zodType),
    };
    if (desc.values) {
      entry.values = desc.values;
    }
    result[key] = entry;
  }

  return result;
}

/**
 * Return simplified schema representations for ALL actions.
 */
export function dumpAllSchemas(): Record<string, Record<string, any>> {
  const result: Record<string, Record<string, any>> = {};
  for (const action of Object.keys(schemaMap)) {
    const d = dumpSchema(action);
    if (d) result[action] = d;
  }
  return result;
}
