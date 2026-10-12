/** Decode SVG in image mode and expose only PNG pixels, never an active SVG document. */
export async function rasterizeIcon(
  blob: Blob,
  signal?: AbortSignal,
): Promise<Blob | null> {
  if (signal?.aborted) return null;
  const source = URL.createObjectURL(blob);
  const image = new Image();
  const abort = () => image.removeAttribute('src');
  signal?.addEventListener('abort', abort, { once: true });
  try {
    // This URL stays off the DOM and is revoked before a caller receives pixels.
    image.src = source;
    await image.decode();
    if (signal?.aborted || !image.naturalWidth || !image.naturalHeight)
      return null;
    const canvas = document.createElement('canvas');
    canvas.width = 64;
    canvas.height = 64;
    const context = canvas.getContext('2d');
    if (!context) return null;
    const scale = 64 / Math.max(image.naturalWidth, image.naturalHeight);
    const width = image.naturalWidth * scale;
    const height = image.naturalHeight * scale;
    context.drawImage(
      image,
      (64 - width) / 2,
      (64 - height) / 2,
      width,
      height,
    );
    const result = await new Promise<Blob | null>((resolve) =>
      canvas.toBlob(resolve, 'image/png'),
    );
    return signal?.aborted ? null : result;
  } catch {
    return null;
  } finally {
    signal?.removeEventListener('abort', abort);
    URL.revokeObjectURL(source);
  }
}
