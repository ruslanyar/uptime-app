'use client';
import Image from 'next/image';
import { UserRound } from 'lucide-react';
import { apiOrigin } from '@/lib/auth/api';
import { cn } from '@/lib/utils';

export function Avatar({
  url,
  name,
  size = 'lg',
}: {
  url?: string;
  name?: string;
  size?: 'sm' | 'lg';
}) {
  const pixels = size === 'sm' ? 36 : 80;
  return (
    <span
      className={cn(
        'flex shrink-0 items-center justify-center overflow-hidden rounded-full border',
        size === 'sm'
          ? 'size-9 border-primary/25 bg-primary/10 text-primary uppercase'
          : 'size-20 bg-muted text-muted-foreground',
      )}
    >
      {url ? (
        <Image
          src={`${apiOrigin(process.env.NEXT_PUBLIC_API_URL)}${url}`}
          alt="Аватар"
          width={pixels}
          height={pixels}
          className={cn(
            'shrink-0 object-cover',
            size === 'sm' ? 'size-9 max-w-none' : 'size-full',
          )}
        />
      ) : name ? (
        <span aria-hidden="true">{Array.from(name.trim())[0]}</span>
      ) : (
        <UserRound className="size-8" aria-hidden="true" />
      )}
    </span>
  );
}
