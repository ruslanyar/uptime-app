import type { Credentials } from './api';

export type FieldErrors = Partial<Record<keyof Credentials, string>>;
/** Returns normalized values even on validation failure; preserves password whitespace. */
export function validate(
  values: Credentials,
  register: boolean,
): { values: Credentials; errors: FieldErrors } {
  const normalized = {
    ...values,
    email: values.email.trim().toLowerCase(),
    ...(register ? { name: values.name?.trim() ?? '' } : {}),
  };
  const errors: FieldErrors = {};
  // Match backend limits: Unicode code points for name/password, UTF-8 bytes for email.
  if (
    register &&
    (Array.from(normalized.name ?? '').length < 2 ||
      Array.from(normalized.name ?? '').length > 50)
  )
    errors.name = 'Введите имя длиной от 2 до 50 символов.';
  if (
    new TextEncoder().encode(normalized.email).length > 254 ||
    !/^[^\s@]+@[^\s@]+$/.test(normalized.email)
  )
    errors.email = 'Введите корректный email (до 254 байт).';
  const length = Array.from(values.password).length;
  if (length < 15 || length > 128)
    errors.password = 'Пароль должен содержать от 15 до 128 символов.';
  return { values: normalized, errors };
}
