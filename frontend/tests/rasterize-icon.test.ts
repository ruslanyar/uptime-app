import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { rasterizeIcon } from '@/lib/monitors/rasterize-icon';

const decode = vi.fn();
const revokeObjectURL = vi.fn();
let image: HTMLImageElement;

beforeEach(() => {
  decode.mockReset().mockResolvedValue(undefined);
  revokeObjectURL.mockReset();
  image = document.createElement('img');
  image.decode = decode;
  Object.defineProperties(image, {
    naturalWidth: { value: 32 },
    naturalHeight: { value: 16 },
  });
  vi.stubGlobal(
    'Image',
    class {
      constructor() {
        return image;
      }
    },
  );
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL = vi.fn().mockReturnValue('blob:private-svg');
      static revokeObjectURL = revokeObjectURL;
    },
  );
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

it('returns PNG pixels, preserving aspect ratio without attaching the SVG to the DOM', async () => {
  const drawImage = vi.fn();
  vi.spyOn(HTMLCanvasElement.prototype, 'getContext').mockReturnValue({
    drawImage,
  } as unknown as CanvasRenderingContext2D);
  const png = new Blob(['png'], { type: 'image/png' });
  vi.spyOn(HTMLCanvasElement.prototype, 'toBlob').mockImplementation(function (
    this: HTMLCanvasElement,
    callback,
    type,
  ) {
    expect(this.width).toBe(64);
    expect(this.height).toBe(64);
    expect(type).toBe('image/png');
    callback(png);
  });
  expect(
    await rasterizeIcon(new Blob(['svg'], { type: 'image/svg+xml' })),
  ).toBe(png);
  expect(drawImage).toHaveBeenCalledWith(image, 0, 16, 64, 32);
  expect(image.isConnected).toBe(false);
  expect(revokeObjectURL).toHaveBeenCalledWith('blob:private-svg');
});

it('releases the SVG URL on decode failure', async () => {
  decode.mockRejectedValue(new Error('invalid SVG'));
  expect(await rasterizeIcon(new Blob(['broken']))).toBeNull();
  expect(revokeObjectURL).toHaveBeenCalledWith('blob:private-svg');
});

it('discards decoding cancelled by a monitor edit and releases the source', async () => {
  const controller = new AbortController();
  let finish!: () => void;
  decode.mockImplementation(
    () =>
      new Promise<void>((resolve) => {
        finish = resolve;
      }),
  );
  const pending = rasterizeIcon(new Blob(['svg']), controller.signal);
  controller.abort();
  expect(image.hasAttribute('src')).toBe(false);
  finish();
  expect(await pending).toBeNull();
  expect(revokeObjectURL).toHaveBeenCalledWith('blob:private-svg');
});
