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
import { monitorErrorMessage } from '@/lib/monitors/api';

const { list, create, update, remove, session, current } = vi.hoisted(() => {
  const current: { status: string; user: { id: string }; error?: unknown } = {
    status: 'authenticated',
    user: { id: 'owner' },
  };
  return {
    list: vi.fn(),
    create: vi.fn(),
    update: vi.fn(),
    remove: vi.fn(),
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
    delete = remove;
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
  remove.mockReset().mockResolvedValue(undefined);
  session.check.mockReset().mockResolvedValue(undefined);
  current.error = undefined;
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

it('requires confirmation and cancels without deleting', async () => {
  list.mockResolvedValue([item]);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  const button = screen.getByRole('button', { name: `Удалить ${item.url}` });
  fireEvent.click(button);
  expect(
    screen.getByRole('group', { name: `Удалить сайт ${item.url}?` }),
  ).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Отмена' })).toHaveFocus();
  expect(screen.getByRole('button', { name: 'Добавить сайт' })).toBeDisabled();
  expect(
    screen.getByRole('button', { name: `Редактировать ${item.url}` }),
  ).toBeDisabled();
  expect(remove).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
  expect(button).toHaveFocus();
  expect(screen.queryByRole('group')).not.toBeInTheDocument();
  expect(remove).not.toHaveBeenCalled();
});
it('deletes only the confirmed item and preserves the other cards', async () => {
  list.mockResolvedValue([
    item,
    { ...item, id: 'two', url: 'https://other.example.com' },
  ]);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  fireEvent.click(screen.getByRole('button', { name: `Удалить ${item.url}` }));
  fireEvent.click(screen.getByRole('button', { name: 'Удалить сайт' }));
  await screen.findByText('Сайт удалён.');
  expect(remove).toHaveBeenCalledExactlyOnceWith(item.id);
  expect(screen.queryByText(item.url)).not.toBeInTheDocument();
  expect(screen.getByText('https://other.example.com')).toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Добавить сайт' })).toHaveFocus();
});
it('blocks duplicate deletion and other actions, then shows the empty state', async () => {
  list.mockResolvedValue([item]);
  let finish!: () => void;
  remove.mockImplementationOnce(
    () =>
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
  );
  const requestState = vi.fn();
  render(<Monitors userID="owner" onRequestState={requestState} />);
  await screen.findByText(item.url);
  fireEvent.click(screen.getByRole('button', { name: `Удалить ${item.url}` }));
  const confirm = screen.getByRole('button', {
    name: 'Удалить сайт',
  });
  fireEvent.click(confirm);
  fireEvent.click(confirm);
  await waitFor(() => expect(remove).toHaveBeenCalledOnce());
  expect(screen.getByRole('button', { name: 'Удаляем…' })).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Отмена' })).toBeDisabled();
  expect(
    screen.getByRole('button', { name: 'Обновить список сайтов' }),
  ).toBeDisabled();
  expect(requestState).toHaveBeenCalledWith('pending');
  await act(async () => {
    finish();
  });
  await screen.findByText('Пока нет сайтов');
  expect(screen.queryByRole('list')).not.toBeInTheDocument();
  expect(requestState).toHaveBeenLastCalledWith('idle');
});
it.each([
  new AuthError('http', 500),
  new AuthError('http', 404),
  new AuthError('network'),
  new AuthError('protocol'),
])('keeps the card and confirmation after deletion fails', async (error) => {
  list.mockResolvedValue([item]);
  remove.mockRejectedValue(error);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  fireEvent.click(screen.getByRole('button', { name: `Удалить ${item.url}` }));
  fireEvent.click(screen.getByRole('button', { name: 'Удалить сайт' }));
  await screen.findByText(monitorErrorMessage(error, 'delete'));
  expect(screen.getByText(item.url)).toBeInTheDocument();
  expect(screen.getByRole('group')).toBeInTheDocument();
  expect(remove).toHaveBeenCalledOnce();
  expect(screen.getByRole('button', { name: 'Удалить сайт' })).toBeEnabled();
});
it('reconciles a missing card after refreshing an uncertain deletion', async () => {
  list.mockResolvedValueOnce([item]).mockResolvedValueOnce([]);
  remove.mockRejectedValue(new AuthError('http', 404));
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  fireEvent.click(screen.getByRole('button', { name: `Удалить ${item.url}` }));
  fireEvent.click(screen.getByRole('button', { name: 'Удалить сайт' }));
  await screen.findByText('Сайт больше не найден. Обновите список сайтов.');
  const buttons = screen.getAllByRole('button', {
    name: 'Обновить список сайтов',
  });
  fireEvent.click(buttons[buttons.length - 1]);
  await screen.findByText('Пока нет сайтов');
  expect(screen.queryByRole('group')).not.toBeInTheDocument();
  expect(screen.getByRole('button', { name: 'Добавить сайт' })).toBeEnabled();
  expect(screen.getByRole('button', { name: 'Добавить сайт' })).toHaveFocus();
  expect(remove).toHaveBeenCalledOnce();
});
it('does not delete after the session changes owner', async () => {
  list.mockResolvedValue([item]);
  render(<Monitors userID="owner" />);
  await screen.findByText(item.url);
  fireEvent.click(screen.getByRole('button', { name: `Удалить ${item.url}` }));
  current.user.id = 'other';
  fireEvent.click(screen.getByRole('button', { name: 'Удалить сайт' }));
  await screen.findByText('Сессия истекла. Повторите вход.');
  expect(remove).not.toHaveBeenCalled();
});
it('retains the confirmation if the session check loses connection', async () => {
  list.mockResolvedValue([item]);
  session.check.mockImplementationOnce(async () => {
    current.status = 'error';
    current.error = new AuthError('network');
  });
  const requestState = vi.fn();
  render(<Monitors userID="owner" onRequestState={requestState} />);
  await screen.findByText(item.url);
  fireEvent.click(screen.getByRole('button', { name: `Удалить ${item.url}` }));
  fireEvent.click(screen.getByRole('button', { name: 'Удалить сайт' }));
  await screen.findByText(
    'Не удалось подтвердить удаление. Обновите список сайтов перед повторной попыткой.',
  );
  expect(remove).not.toHaveBeenCalled();
  expect(requestState).toHaveBeenLastCalledWith('failed');
  fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
  expect(requestState).toHaveBeenLastCalledWith('idle');
});
