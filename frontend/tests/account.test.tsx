import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Account } from '@/components/account';
import { AuthError } from '@/lib/auth/api';
import type { SessionState } from '@/lib/auth/session';

const { auth, mutate } = vi.hoisted(() => {
  const mutate = vi.fn();
  return {
    mutate,
    auth: {
      status: 'authenticated',
      user: { id: '1', name: 'Анна', email: 'anna@example.com' },
      session: { mutate, check: vi.fn() },
    } as SessionState & {
      session: {
        mutate: ReturnType<typeof vi.fn>;
        check: ReturnType<typeof vi.fn>;
      };
    },
  };
});
vi.mock('@/components/auth-provider', () => ({ useAuth: () => auth }));
vi.mock('next/navigation', () => ({ redirect: vi.fn() }));
beforeEach(() => {
  mutate.mockReset();
  auth.status = 'authenticated';
  auth.user = { id: '1', name: 'Анна', email: 'anna@example.com' };
  auth.error = undefined;
});
function edit() {
  render(<Account />);
  fireEvent.click(
    screen.getByRole('button', { name: 'Редактировать профиль' }),
  );
  return screen.getByLabelText('Имя');
}
it('prefills the name and cancels without saving', () => {
  expect(edit()).toHaveValue('Анна');
  fireEvent.change(screen.getByLabelText('Имя'), {
    target: { value: 'Other' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
  expect(screen.queryByLabelText('Имя')).not.toBeInTheDocument();
  expect(mutate).not.toHaveBeenCalled();
});
it.each(['   ', 'я', 'я'.repeat(51)])(
  'validates invalid names before submission',
  (name) => {
    const input = edit();
    fireEvent.change(input, { target: { value: name } });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    expect(input).toHaveAttribute('aria-invalid', 'true');
    expect(mutate).not.toHaveBeenCalled();
  },
);
it('trims Unicode names and disables saving while pending', async () => {
  let finish!: () => void;
  mutate.mockImplementation(() => {
    auth.status = 'loading';
    auth.user = undefined;
    return new Promise<void>((resolve) => {
      finish = () => {
        auth.status = 'authenticated';
        auth.user = {
          id: '1',
          name: '👩'.repeat(50),
          email: 'anna@example.com',
        };
        resolve();
      };
    });
  });
  const input = edit();
  fireEvent.change(input, {
    target: { value: '  ' + '👩'.repeat(50) + '  ' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
  expect(screen.getByRole('button', { name: 'Сохраняем…' })).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Выйти' })).toBeDisabled();
  expect(mutate).toHaveBeenCalledExactlyOnceWith('profile', {
    name: '👩'.repeat(50),
  });
  finish();
  await waitFor(() =>
    expect(screen.getByRole('status')).toHaveTextContent('Имя сохранено.'),
  );
  expect(screen.getByRole('heading')).toHaveTextContent(
    'Здравствуйте, ' + '👩'.repeat(50),
  );
});
it('preserves the draft after a server error and allows retry', async () => {
  mutate.mockRejectedValueOnce(new AuthError('http', 500));
  const input = edit();
  fireEvent.change(input, { target: { value: 'Новое имя' } });
  fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
  await waitFor(() => expect(screen.getByRole('alert')).toBeVisible());
  expect(screen.getByLabelText('Имя')).toHaveValue('Новое имя');
  expect(screen.getByRole('button', { name: 'Сохранить' })).toBeEnabled();
});
