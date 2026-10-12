import { act, fireEvent, render, waitFor } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { SiteIcon } from '@/components/site-icon';

const { favicon } = vi.hoisted(() => ({ favicon: vi.fn() }));
vi.mock('@/lib/monitors/api', () => ({
  MonitorAPI: class {
    favicon = favicon;
  },
}));
const createObjectURL = vi.fn();
const revokeObjectURL = vi.fn();
beforeEach(() => {
  favicon.mockReset().mockResolvedValue(null);
  createObjectURL.mockReset().mockReturnValue('blob:site-icon');
  revokeObjectURL.mockReset();
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL = createObjectURL;
      static revokeObjectURL = revokeObjectURL;
    },
  );
});
afterEach(() => vi.unstubAllGlobals());

it('keeps the globe until the favicon loads and releases the blob on unmount', async () => {
  favicon.mockResolvedValue(new Blob(['image'], { type: 'image/png' }));
  const view = render(<SiteIcon url="https://example.com" />);
  expect(view.container.querySelector('svg')).toBeInTheDocument();
  await waitFor(() => expect(view.container.querySelector('img')).toBeTruthy());
  const image = view.container.querySelector('img')!;
  expect(image).toHaveClass('invisible');
  fireEvent.load(image);
  expect(image).not.toHaveClass('invisible');
  expect(view.container.querySelector('svg')).not.toBeInTheDocument();
  view.unmount();
  expect(revokeObjectURL).toHaveBeenCalledWith('blob:site-icon');
  expect(favicon.mock.calls[0][1].aborted).toBe(true);
});

it('preserves the globe when discovery returns no image', async () => {
  const view = render(<SiteIcon url="https://example.com" />);
  await act(async () => {});
  expect(view.container.querySelector('svg')).toBeInTheDocument();
  expect(view.container.querySelector('img')).not.toBeInTheDocument();
});

it('restores the globe if the browser cannot decode the image', async () => {
  favicon.mockResolvedValue(new Blob(['broken'], { type: 'image/png' }));
  const view = render(<SiteIcon url="https://example.com" />);
  await waitFor(() => expect(view.container.querySelector('img')).toBeTruthy());
  fireEvent.error(view.container.querySelector('img')!);
  expect(view.container.querySelector('svg')).toBeInTheDocument();
  expect(view.container.querySelector('img')).not.toBeInTheDocument();
});

it('ignores late icon responses after editing and retries the new URL', async () => {
  let finish!: (blob: Blob) => void;
  favicon.mockImplementationOnce(
    () =>
      new Promise<Blob>((resolve) => {
        finish = resolve;
      }),
  );
  const view = render(<SiteIcon key="old" url="https://old.example.com" />);
  view.rerender(<SiteIcon key="new" url="https://new.example.com" />);
  await act(async () => {
    finish(new Blob(['old']));
  });
  expect(favicon).toHaveBeenCalledTimes(2);
  expect(favicon.mock.calls[1][0]).toBe('https://new.example.com');
  expect(createObjectURL).not.toHaveBeenCalled();
  expect(view.container.querySelector('svg')).toBeInTheDocument();
});
