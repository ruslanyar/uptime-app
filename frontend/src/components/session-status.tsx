'use client';
import { Button } from '@/components/ui/button';
import { Alert } from '@/components/ui/alert';
import { LoaderCircle } from 'lucide-react';
import { useAuth } from './auth-provider';
import { AuthError, errorMessage } from '@/lib/auth/api';

export function SessionStatus() {
  const { status, error, session } = useAuth();
  if (status === 'error')
    return (
      <Alert className="mt-5 border-destructive/30 bg-destructive/5 text-destructive">
        <p>{errorMessage(error)}</p>
        {!(
          error instanceof AuthError &&
          ['unsupported', 'config'].includes(error.kind)
        ) && (
          <Button
            variant="outline"
            className="mt-3"
            onClick={() => void session.check(true)}
          >
            Повторить проверку
          </Button>
        )}
      </Alert>
    );
  return (
    <p
      role="status"
      className="mt-6 flex items-center gap-3 text-sm text-muted-foreground"
    >
      <LoaderCircle
        aria-hidden="true"
        className="size-4 motion-safe:animate-spin"
      />
      Проверяем сессию…
    </p>
  );
}
