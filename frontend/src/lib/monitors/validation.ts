export type IntervalUnit = 'seconds' | 'minutes' | 'hours';
export type MonitorInput = { url: string; interval_seconds: number };
export type MonitorErrors = { url?: string; interval?: string };
const multipliers: Record<IntervalUnit, number> = {
  seconds: 1,
  minutes: 60,
  hours: 3600,
};

export function validateMonitor(
  rawURL: string,
  amount: string,
  unit: IntervalUnit,
): { input?: MonitorInput; errors: MonitorErrors } {
  const errors: MonitorErrors = {};
  const url = rawURL.trim();
  try {
    const parsed = new URL(url);
    const authority = url.match(/^https?:\/\/([^/?#]*)/i)?.[1];
    if (
      !['http:', 'https:'].includes(parsed.protocol) ||
      !parsed.hostname ||
      !authority ||
      authority.includes('@') ||
      authority.endsWith(':') ||
      url.includes('#') ||
      /\s/.test(url) ||
      /%(?![a-f\d]{2})/i.test(url) ||
      new TextEncoder().encode(url).length > 2048 ||
      (parsed.port !== '' && Number(parsed.port) < 1)
    )
      throw new Error();
  } catch {
    errors.url =
      'Введите полный URL с http:// или https:// без логина и фрагмента.';
  }
  const seconds = Number(amount) * multipliers[unit];
  if (
    amount.trim() === '' ||
    !Number.isSafeInteger(seconds) ||
    seconds < 60 ||
    seconds > 86400
  )
    errors.interval =
      'Интервал должен быть от 1 минуты до 24 часов и составлять целое число секунд.';
  return {
    errors,
    ...(Object.keys(errors).length === 0
      ? { input: { url, interval_seconds: seconds } }
      : {}),
  };
}

export function formatInterval(seconds: number): string {
  if (seconds % 3600 === 0) return `${seconds / 3600} ч`;
  if (seconds % 60 === 0) return `${seconds / 60} мин`;
  return `${seconds} сек`;
}
