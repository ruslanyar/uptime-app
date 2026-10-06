import { StrictMode } from 'react';
import { render, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import { AuthProvider, useAuth } from '@/components/auth-provider';
import { AuthAPI } from '@/lib/auth/api';
import { BrowserCoordination } from '@/lib/auth/browser-coordination';

it('reconnects in Strict Mode and removes browser listeners on unmount', async () => {
  const open = vi
    .spyOn(BrowserCoordination.prototype, 'open')
    .mockImplementation(() => {});
  const close = vi
    .spyOn(BrowserCoordination.prototype, 'close')
    .mockImplementation(() => {});
  const call = vi
    .spyOn(AuthAPI.prototype, 'call')
    .mockResolvedValue({ id: '1', name: 'Анна', email: 'anna@example.com' });
  const windowAdd = vi.spyOn(window, 'addEventListener');
  const windowRemove = vi.spyOn(window, 'removeEventListener');
  const documentAdd = vi.spyOn(document, 'addEventListener');
  const documentRemove = vi.spyOn(document, 'removeEventListener');
  function Status() {
    return <p>{useAuth().status}</p>;
  }
  const view = render(
    <StrictMode>
      <AuthProvider>
        <Status />
      </AuthProvider>
    </StrictMode>,
  );
  await waitFor(() => expect(view.getByText('authenticated')).toBeVisible());
  expect(open).toHaveBeenCalledTimes(2);
  expect(close).toHaveBeenCalledTimes(1);
  expect(call).toHaveBeenCalledTimes(1);
  view.unmount();
  expect(close).toHaveBeenCalledTimes(2);
  for (const [event, handler] of windowAdd.mock.calls.filter(
    ([event]) => event === 'focus',
  )) {
    expect(windowRemove).toHaveBeenCalledWith(event, handler);
  }
  for (const [event, handler] of documentAdd.mock.calls.filter(
    ([event]) => event === 'visibilitychange',
  )) {
    expect(documentRemove).toHaveBeenCalledWith(event, handler);
  }
  window.dispatchEvent(new Event('focus'));
  document.dispatchEvent(new Event('visibilitychange'));
  expect(call).toHaveBeenCalledTimes(1);
});
