import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { MonitorForm } from '@/components/monitor-form';
import { AuthError } from '@/lib/auth/api';

it('focuses the URL and cancels without submitting', () => {
  const cancel = vi.fn();
  const create = vi.fn();
  render(<MonitorForm onSubmit={create} onCancel={cancel} />);
  expect(screen.getByLabelText('URL сайта')).toHaveFocus();
  fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
  expect(cancel).toHaveBeenCalledOnce();
  expect(create).not.toHaveBeenCalled();
});
it('shows field errors without making a request', () => {
  const create = vi.fn();
  render(<MonitorForm onSubmit={create} onCancel={vi.fn()} />);
  fireEvent.change(screen.getByLabelText('Интервал опроса'), {
    target: { value: '0' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
  expect(screen.getByLabelText('URL сайта')).toHaveAttribute(
    'aria-invalid',
    'true',
  );
  expect(screen.getByLabelText('Интервал опроса')).toHaveAttribute(
    'aria-invalid',
    'true',
  );
  expect(create).not.toHaveBeenCalled();
});
it.each([
  ['seconds', '90', 90],
  ['minutes', '1.5', 90],
  ['hours', '2', 7200],
])(
  'submits %s as seconds and prevents double submission',
  async (unit, value, seconds) => {
    let finish!: () => void;
    const create = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          finish = resolve;
        }),
    );
    render(<MonitorForm onSubmit={create} onCancel={vi.fn()} />);
    fireEvent.change(screen.getByLabelText('URL сайта'), {
      target: { value: ' https://example.com ' },
    });
    fireEvent.change(screen.getByLabelText('Единица измерения'), {
      target: { value: unit },
    });
    fireEvent.change(screen.getByLabelText('Интервал опроса'), {
      target: { value },
    });
    fireEvent.submit(screen.getByRole('form', { name: 'Новый сайт' }));
    fireEvent.submit(screen.getByRole('form', { name: 'Новый сайт' }));
    expect(create).toHaveBeenCalledExactlyOnceWith({
      url: 'https://example.com',
      interval_seconds: seconds,
    });
    expect(screen.getByRole('button', { name: 'Создаём…' })).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Отмена' })).toBeDisabled();
    finish();
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Создать' })).toBeEnabled(),
    );
  },
);
it.each([
  new AuthError('http', 409),
  new AuthError('network'),
  new AuthError('http', 500),
])('preserves inputs after an error', async (error) => {
  const create = vi.fn().mockRejectedValue(error);
  render(<MonitorForm onSubmit={create} onCancel={vi.fn()} />);
  fireEvent.change(screen.getByLabelText('URL сайта'), {
    target: { value: 'https://example.com' },
  });
  fireEvent.click(screen.getByRole('button', { name: 'Создать' }));
  await screen.findByRole('alert');
  expect(screen.getByLabelText('URL сайта')).toHaveValue('https://example.com');
  expect(screen.getByLabelText('Интервал опроса')).toHaveValue(5);
  expect(screen.getByRole('button', { name: 'Создать' })).toBeEnabled();
});

it.each([
  [7200, 2, 'hours'],
  [300, 5, 'minutes'],
  [90, 90, 'seconds'],
])('prefills interval %s without losing precision', (seconds, amount, unit) => {
  const submit = vi.fn();
  const cancel = vi.fn();
  render(
    <MonitorForm
      initialValues={{
        url: 'https://example.com',
        interval_seconds: Number(seconds),
      }}
      onSubmit={submit}
      onCancel={cancel}
    />,
  );
  expect(
    screen.getByRole('form', { name: 'Редактирование сайта' }),
  ).toBeInTheDocument();
  expect(screen.getByLabelText('URL сайта')).toHaveValue('https://example.com');
  expect(screen.getByLabelText('Интервал опроса')).toHaveValue(amount);
  expect(screen.getByLabelText('Единица измерения')).toHaveValue(unit);
  fireEvent.click(screen.getByRole('button', { name: 'Отмена' }));
  expect(submit).not.toHaveBeenCalled();
  expect(cancel).toHaveBeenCalledOnce();
});
it.each([
  new AuthError('http', 404),
  new AuthError('network'),
  new AuthError('protocol'),
])(
  'keeps an edit draft and offers reload after an uncertain result',
  async (error) => {
    const reload = vi.fn();
    render(
      <MonitorForm
        initialValues={{ url: 'https://example.com', interval_seconds: 90 }}
        onSubmit={vi.fn().mockRejectedValue(error)}
        onCancel={vi.fn()}
        onReload={reload}
      />,
    );
    fireEvent.change(screen.getByLabelText('URL сайта'), {
      target: { value: 'https://draft.example.com' },
    });
    fireEvent.click(screen.getByRole('button', { name: 'Сохранить' }));
    await screen.findByRole('alert');
    expect(screen.getByLabelText('URL сайта')).toHaveValue(
      'https://draft.example.com',
    );
    expect(screen.getByLabelText('Интервал опроса')).toHaveValue(90);
    fireEvent.click(
      screen.getByRole('button', { name: 'Обновить список сайтов' }),
    );
    expect(reload).toHaveBeenCalledOnce();
  },
);
