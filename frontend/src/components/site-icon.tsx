'use client';
import { useEffect, useState } from 'react';
import { Globe } from 'lucide-react';
import { MonitorAPI } from '@/lib/monitors/api';

/** Remount after saving a monitor to retry discovery and discard the old icon. */
export function SiteIcon({ url }: { url: string }) {
  const [source, setSource] = useState<string>();
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    let objectURL: string | undefined;
    void new MonitorAPI().favicon(url, controller.signal).then((blob) => {
      if (!blob || controller.signal.aborted) return;
      objectURL = URL.createObjectURL(blob);
      setSource(objectURL);
    });
    return () => {
      controller.abort();
      if (objectURL) URL.revokeObjectURL(objectURL);
    };
  }, [url]);

  return (
    <div
      aria-hidden="true"
      className="relative grid size-10 shrink-0 place-items-center rounded-lg border border-border bg-background text-primary"
    >
      {!loaded && <Globe className="size-5" />}
      {source && (
        // Blob URLs already contain bounded API image responses; Next optimization is unnecessary.
        // eslint-disable-next-line @next/next/no-img-element
        <img
          src={source}
          alt=""
          width={20}
          height={20}
          className={`absolute size-5 object-contain ${loaded ? '' : 'invisible'}`}
          onLoad={() => setLoaded(true)}
          onError={() => {
            setLoaded(false);
            setSource(undefined);
          }}
        />
      )}
    </div>
  );
}
