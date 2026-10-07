import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { Monitors } from '@/components/monitors';
import { AuthError } from '@/lib/auth/api';

const { list, create, update, session, current } = vi.hoisted(() => {
  const current = { status: 'authenticated', user: { id: 'owner' } };
  return {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    current,
    session: { check: vi.fn(), snapshot: () => current },
  };
});
vi.mock('@/components/auth-provider', () => ({ useAuth: () => ({ session }) }));
vi.mock('@/lib/monitors/api', async (original) => ({
  ...(await original<typeof import('@/lib/monitors/api')>()),
  MonitorAPI: class {
    list = list;
    create = create;
    update = update;
  },
}));
const item = {
  id: 'one',
  url: 'https://example.com',
  interval_seconds: 300,
  created_at: '2026-10-07T10:00:00Z',
  updated_at: '2026-10-07T10:00:00Z',
};
beforeEach(() => {
  list.mockReset().mockResolvedValue([]);
  create.mockReset().mockResolvedValue(item);
  update.mockReset().mockResolvedValue({
    ...item,
    url: 'https://updated.example.com',
    interval_seconds: 90,
  });
  session.check.mockReset().mockResolvedValue(undefined);
  current.status = 'authenticated';
  current.user.id = 'owner';
});
it('loads empty state and displays a created monitor immediately', async () => {
  render(<Monitors userID="owner" />);
  expect(screen.getByText('Загружаем сайты…')).toBeInTheDocument();
  await screen.findByText('Пока нет сайтов');
  fireEvent.click(screen.getByRole('button', { name: 'Добавить сайт' }));
  fireEvent.change(screen.getByLabelText('URL сайта'), {
    target: { value: item.url },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
  await screen.findByText(item.url);
  expect(screen.getByText('Интервал: 5 мин')).toBeInTheDocument();
  expect(screen.getByText('Сайт добавлен.')).toBeInTheDocument();
  expect(screen.queryByRole('form')).not.toBeInTheDocument();
  expect(session.check).toHaveBeenCalledOnce();
});
it('loads saved monitors', async () => {
  list.mockResolvedValue([item]);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  expect(screen.queryByText('Пока нет сайтов')).not.toBeInTheDocument();
});
it('offers retry after a failed list request', async () => {
  list
    .mockRejectedValueOnce(new AuthError('network'))
    .mockResolvedValueOnce([item]);
  render(<Monitors userID="owner" />);
  await screen.findByRole('alert');
  fireEvent.click(screen.getByRole('button', { name: 'Повторить загрузку' }));
  await screen.findByText(item.url);
  expect(screen.queryByRole('alert')).not.toBeInTheDocument();
});
it('checks session and retries GET once on expired access', async () => {
  list
    .mockRejectedValueOnce(new AuthError('http', 401))
    .mockResolvedValueOnce([item]);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  expect(session.check).toHaveBeenCalledOnce();
  expect(list).toHaveBeenCalledTimes(2);
});
it('does not send POST when the session no longer belongs to the screen user', async () => {
  render(<Monitors userID="owner" />);
  await screen.findByText('Пока нет сайтов');
  fireEvent.click(screen.getByRole('button', { name: 'Добавить сайт' }));
  fireEvent.change(screen.getByLabelText('URL сайта'), {
    target: { value: item.url },
  });
  current.user.id = 'other';
  fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
  await screen.findByRole('alert');
  expect(create).not.toHaveBeenCalled();
});
it('ignores old responses after switching users', async () => {
  let finish!: (items: (typeof item)[]) => void;
  list
    .mockImplementationOnce(
      () =>
        new Promise((resolve) => {
          finish = resolve;
        }),
    )
    .mockResolvedValueOnce([]);
  const view = render(<Monitors key="owner" userID="owner" />);
  view.rerender(<Monitors key="other" userID="other" />);
  await screen.findByText('Пока нет сайтов');
  await act(async () => {
    finish([item]);
  });
  expect(screen.queryByText(item.url)).not.toBeInTheDocument();
});
it('blocks list refresh during creation so a late GET cannot remove the new card', async () => {
  let finish!: (value: typeof item) => void;
  create.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  render(<Monitors userID="owner" />);
  await screen.findByText('Пока нет сайтов');
  fireEvent.click(screen.getByRole('button', { name: 'Добавить сайт' }));
  fireEvent.change(screen.getByLabelText('URL сайта'), {
    target: { value: item.url },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
  await waitFor(() => expect(create).toHaveBeenCalledOnce());
  expect(
    screen.getByRole('button', { name: 'Обновить список сайтов' }),
  ).toBeDisabled();
  await act(async () => {
    finish(item);
  });
  await screen.findByText(item.url);
});

it('edits in place, preserves ordering and restores focus to its trigger', async () => {
  list.mockResolvedValue([
    item,
    { ...item, id: 'two', url: 'https://other.example.com' },
  ]);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  const button = screen.getByRole('button', {
    name: `Редактировать ${item.url}`,
  });
  fireEvent.click(button);
  expect(screen.getByRole('button', { name: 'Добавить сайт' })).toBeDisabled();
  expect(
    screen.getByRole('button', {
      name: 'Редактировать https://other.example.com',
    }),
  ).toBeDisabled();
  fireEvent.change(screen.getByLabelText('URL сайта'), {
    target: { value: 'https://updated.example.com' },
  });
  fireEvent.change(screen.getByLabelText('Единица измерения'), {
    target: { value: 'seconds' },
  });
  fireEvent.change(screen.getByLabelText('Интервал опроса'), {
    target: { value: '90' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
  await screen.findByText('Изменения сохранены.');
  expect(update).toHaveBeenCalledExactlyOnceWith(item.id, {
    url: 'https://updated.example.com',
    interval_seconds: 90,
  });
  expect(create).not.toHaveBeenCalled();
  expect(screen.getAllByRole('listitem')[0]).toHaveTextContent(
    'https://updated.example.com',
  );
  expect(screen.getAllByRole('listitem')[1]).toHaveTextContent(
    'https://other.example.com',
  );
  expect(screen.queryByRole('form')).not.toBeInTheDocument();
  expect(button).toHaveFocus();
});
it('cancels changes and starts a fresh draft when reopening', async () => {
  list.mockResolvedValue([item]);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  fireEvent.click(
    screen.getByRole('button', { name: `Редактировать ${item.url}` }),
  );
  fireEvent.change(screen.getByLabelText('URL сайта'), {
    target: { value: 'https://draft.example.com' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
  expect(update).not.toHaveBeenCalled();
  fireEvent.click(
    screen.getByRole('button', { name: `Редактировать ${item.url}` }),
  );
  expect(screen.getByLabelText('URL сайта')).toHaveValue(item.url);
});
it('blocks other actions and duplicate submissions while saving', async () => {
  let finish!: (value: typeof item) => void;
  update.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  list.mockResolvedValue([item]);
  const requestState = vi.fn();
  render(<Monitors userID="owner" onRequestState={requestState} />);
  await screen.findByText(item.url);
  fireEvent.click(
    screen.getByRole('button', { name: `Редактировать ${item.url}` }),
  );
  fireEvent.submit(screen.getByRole('form'));
  fireEvent.submit(screen.getByRole('form'));
  await waitFor(() => expect(update).toHaveBeenCalledOnce());
  expect(screen.getByRole('button', { name: 'Сохраняем…' })).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Отмена' })).toBeDisabled();
  expect(
    screen.getByRole('button', { name: 'Обновить список сайтов' }),
  ).toBeDisabled();
  expect(requestState).toHaveBeenCalledWith('pending');
  await act(async () => {
    finish(item);
  });
  await screen.findByText('Изменения сохранены.');
  expect(requestState).toHaveBeenLastCalledWith('idle');
});
it.each([
  new AuthError('http', 409),
  new AuthError('network'),
  new AuthError('http', 404),
])('preserves draft and old card on a failed update', async (error) => {
  list.mockResolvedValue([item]);
  update.mockRejectedValue(error);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  fireEvent.click(
    screen.getByRole('button', { name: `Редактировать ${item.url}` }),
  );
  fireEvent.change(screen.getByLabelText('URL сайта'), {
    target: { value: 'https://draft.example.com' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
  await screen.findByRole('alert');
  expect(screen.getByText(item.url)).toBeInTheDocument();
  expect(screen.getByLabelText('URL сайта')).toHaveValue(
    'https://draft.example.com',
  );
  expect(update).toHaveBeenCalledOnce();
  if (error.status === 404 || error.kind === 'network') {
    const buttons = screen.getAllByRole('button', {
      name: 'Обновить список сайтов',
    });
    fireEvent.click(buttons[buttons.length - 1]);
    await waitFor(() => expect(list).toHaveBeenCalledTimes(2));
    expect(screen.getByLabelText('URL сайта')).toHaveValue(
      'https://draft.example.com',
    );
  }
});
it('does not update after the session changes owner', async () => {
  list.mockResolvedValue([item]);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  fireEvent.click(
    screen.getByRole('button', { name: `Редактировать ${item.url}` }),
  );
  current.user.id = 'other';
  fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
  await screen.findByRole('alert');
  expect(update).not.toHaveBeenCalled();
});
