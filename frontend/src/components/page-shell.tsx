import Link from 'next/link';
import { Activity, ArrowUpRight } from 'lucide-react';
import { Card } from '@/components/ui/card';

export function Brand() {
  return (
    <Link
      href="/"
      aria-label="Uptime — главная"
      className="inline-flex shrink-0 items-center gap-3 rounded-sm text-xl font-semibold tracking-tight focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring"
    >
      <span className="grid size-9 place-items-center rounded-lg border border-primary/30 bg-primary/10 text-primary">
        <Activity className="size-5" aria-hidden="true" />
      </span>
      uptime<span className="text-primary">.</span>
    </Link>
  );
}

export function Surface({ children }: { children: React.ReactNode }) {
  return (
    <div className="relative isolate flex min-h-dvh flex-col overflow-x-clip">
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-x-0 top-0 -z-10 h-162.5 bg-[radial-gradient(ellipse_at_75%_0%,#a1bb2630,transparent_55%),radial-gradient(ellipse_at_0%_30%,#39852b20,transparent_50%)]"
      />
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 -z-10 bg-[linear-gradient(#d7f54204_1px,transparent_1px),linear-gradient(90deg,#d7f54204_1px,transparent_1px)] bg-size-[64px_64px] mask-[linear-gradient(black,transparent_80%)]"
      />
      {children}
      <footer className="mx-6 flex flex-wrap justify-between gap-3 border-t border-border/60 py-6 font-mono text-[10px] tracking-widest text-muted-foreground sm:mx-12">
        <span>UPTIME / МОНИТОРИНГ ДОСТУПНОСТИ</span>
        <span>
          ВСЕГДА НА СВЯЗИ <span className="text-primary">↗</span>
        </span>
      </footer>
    </div>
  );
}

export function Eyebrow({ children }: { children: React.ReactNode }) {
  return (
    <p className="mb-4 font-mono text-[11px] font-medium tracking-[0.18em] text-primary">
      {'// '}
      {children}
    </p>
  );
}

export function PageHeading({ children }: { children: React.ReactNode }) {
  return (
    <h1 className="mb-3 text-3xl font-medium tracking-tight wrap-anywhere sm:text-4xl">
      {children}
    </h1>
  );
}

export function PageShell({ children }: { children: React.ReactNode }) {
  return (
    <Surface>
      <header className="flex items-center justify-between border-b border-border/60 px-6 py-6 sm:px-12">
        <Brand />
        <span className="hidden font-mono text-[10px] tracking-widest text-muted-foreground sm:block">
          ВАШИ СЕРВИСЫ. ПОД КОНТРОЛЕМ.
        </span>
      </header>
      <main className="mx-auto grid w-full max-w-6xl flex-1 items-center gap-12 px-6 py-12 lg:grid-cols-2 lg:gap-20 lg:px-12 lg:py-20">
        <div className="hidden lg:block">
          <Eyebrow>НА СВЯЗИ С ВАШИМИ СЕРВИСАМИ</Eyebrow>
          <p className="max-w-lg text-6xl leading-[1.08] tracking-tight">
            Спокойствие начинается с{' '}
            <span className="bg-linear-to-r from-primary to-emerald-400 bg-clip-text text-transparent">
              контроля.
            </span>
          </p>
          <p className="mt-6 max-w-sm text-base leading-relaxed text-muted-foreground">
            Личное пространство для ваших сервисов. Всё важное — в одном месте.
          </p>
          <div
            aria-hidden="true"
            className="mt-12 flex items-center gap-4 text-primary"
          >
            <span className="h-px w-16 bg-linear-to-r from-primary to-transparent" />
            <ArrowUpRight className="size-5" />
            <span className="font-mono text-[10px] tracking-widest">
              UPTIME / YOUR CONTROL ROOM
            </span>
          </div>
        </div>
        <Card className="mx-auto w-full max-w-120 gap-0 border-border/80 bg-card/95 p-6 shadow-2xl shadow-black/30 sm:p-9">
          {children}
        </Card>
      </main>
    </Surface>
  );
}
