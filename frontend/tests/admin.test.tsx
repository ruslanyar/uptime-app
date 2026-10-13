import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { AdminUsers } from '@/components/admin-users';
import { AdminAPI } from '@/lib/admin/api';
import { AuthError, type User } from '@/lib/auth/api';

const { auth } = vi.hoisted(() => {
  const auth = {
    status: 'authenticated',
    user: {
      id: 'admin-id',
      name: 'Админ',
      email: 'admin@example.com',
      role: 'admin',
    } as User,
    session: { snapshot: vi.fn(), check: vi.fn() },
  };
  auth.session.snapshot.mockImplementation(() => auth);
  return { auth };
});
vi.mock('@/components/auth-provider', () => ({ useAuth: () => auth }));
vi.mock('next/navigation', () => ({
  redirect: (path: string) => {
    throw new Error(`redirect:${path}`);
  },
}));

const target = {
  id: 'target-id',
  name: 'Пользователь',
  email: 'target@example.com',
  role: 'user' as const,
};
beforeEach(() => {
  vi.restoreAllMocks();
  auth.status = 'authenticated';
  auth.user = {
    id: 'admin-id',
    name: 'Админ',
    email: 'admin@example.com',
    role: 'admin',
  };
  auth.session.check.mockReset();
});

it('does not load the directory for an ordinary user', () => {
  auth.user = { ...auth.user, role: 'user' };
  const list = vi.spyOn(AdminAPI.prototype, 'list');
  render(<AdminUsers />);
  expect(screen.getByText('Доступ к админке запрещён.')).toBeVisible();
  expect(list).not.toHaveBeenCalled();
});

it('protects admins, confirms deletion, focuses cancel and blocks repeat deletes until reconciliation', async () => {
  const list = vi.spyOn(AdminAPI.prototype, 'list').mockResolvedValue({
    users: [auth.user, target],
    page: 1,
    page_size: 25,
    total: 2,
  });
  const remove = vi
    .spyOn(AdminAPI.prototype, 'delete')
    .mockRejectedValue(new AuthError('network'));
  render(<AdminUsers />);
  const open = await screen.findByRole('button', {
    name: 'Удалить target@example.com',
  });
  expect(
    screen.queryByRole('button', { name: 'Удалить admin@example.com' }),
  ).not.toBeInTheDocument();
  fireEvent.click(open);
  expect(screen.getByRole('button', { name: 'Отмена' })).toHaveFocus();
  fireEvent.keyDown(screen.getByRole('button', { name: 'Отмена' }), {
    key: 'Escape',
  });
  expect(open).toHaveFocus();
  expect(remove).not.toHaveBeenCalled();
  fireEvent.click(open);
  fireEvent.click(screen.getByRole('button', { name: 'Удалить пользователя' }));
  await screen.findByText(
    'Не удалось подтвердить удаление. Обновите список перед повторной попыткой.',
  );
  expect(
    screen.getByRole('button', { name: 'Удалить пользователя' }),
  ).toBeDisabled();
  fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
  expect(open).toBeDisabled();
  expect(remove).toHaveBeenCalledTimes(1);
  list.mockResolvedValue({
    users: [auth.user],
    page: 1,
    page_size: 25,
    total: 1,
  });
  fireEvent.click(screen.getByRole('button', { name: 'Обновить список' }));
  await waitFor(() =>
    expect(screen.queryByText(target.email)).not.toBeInTheDocument(),
  );
});

it('returns to the preceding page after deleting the last row', async () => {
  const first = { users: [auth.user], page: 1, page_size: 25, total: 26 };
  const list = vi
    .spyOn(AdminAPI.prototype, 'list')
    .mockImplementation(async (page) =>
      page === 1
        ? first
        : { users: [target], page: 2, page_size: 25, total: 26 },
    );
  vi.spyOn(AdminAPI.prototype, 'delete').mockImplementation(async () => {
    list.mockImplementation(async (page) => ({
      users: page === 1 ? [auth.user] : [],
      page,
      page_size: 25,
      total: 25,
    }));
  });
  render(<AdminUsers />);
  await screen.findByText('Всего пользователей: 26');
  fireEvent.click(screen.getByRole('button', { name: 'Далее' }));
  fireEvent.click(
    await screen.findByRole('button', { name: 'Удалить target@example.com' }),
  );
  fireEvent.click(screen.getByRole('button', { name: 'Удалить пользователя' }));
  await screen.findByText('Страница 1 из 1');
  expect(list.mock.calls.map(([page]) => page)).toEqual([1, 2, 2, 1]);
  await waitFor(() =>
    expect(
      screen.getByRole('button', { name: 'Обновить список' }),
    ).toHaveFocus(),
  );
});

it('discards a pending response when the current user changes', async () => {
  let finish!: (value: Awaited<ReturnType<AdminAPI['list']>>) => void;
  vi.spyOn(AdminAPI.prototype, 'list').mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const { rerender } = render(<AdminUsers />);
  auth.user = { ...target };
  rerender(<AdminUsers />);
  finish({ users: [target], page: 1, page_size: 25, total: 1 });
  await waitFor(() =>
    expect(screen.getByText('Доступ к админке запрещён.')).toBeVisible(),
  );
  expect(screen.queryByText(target.email)).not.toBeInTheDocument();
});

it('uses cookies and CSRF and rejects unexpected role data', async () => {
  const request = vi.fn().mockResolvedValue(
    new Response(
      JSON.stringify({
        users: [{ ...target, role: 'owner' }],
        total: 1,
        page: 1,
        page_size: 25,
      }),
    ),
  );
  const api = new AdminAPI(() => 'http://localhost:8080', request);
  await expect(api.list(1)).rejects.toMatchObject({ kind: 'protocol' });
  request.mockResolvedValue(new Response(null, { status: 204 }));
  await api.delete(target.id);
  expect(request.mock.calls[1][1]).toMatchObject({
    method: 'DELETE',
    credentials: 'include',
    cache: 'no-store',
    headers: { 'X-CSRF-Protection': '1' },
  });
});
