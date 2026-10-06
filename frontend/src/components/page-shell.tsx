import Link from 'next/link';

export function PageShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="shell">
      <header>
        <Link className="brand" href="/">
          <span aria-hidden="true" />
          uptime
        </Link>
      </header>
      <main>
        <section className="card">{children}</section>
      </main>
    </div>
  );
}
