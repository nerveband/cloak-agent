import type { Locator } from 'playwright-core';

export function semanticSubactionNeedsValue(subaction?: string): boolean {
  return subaction === 'fill' || subaction === 'type' || subaction === 'select';
}

export function semanticActionLabel(
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

export async function executeSemanticLocatorSubaction(
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
