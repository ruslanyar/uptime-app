import Link from 'next/link';
import { Eyebrow, PageHeading } from '@/components/page-shell';
import { AuthForm } from '@/components/auth-form';

export default function Register() {
  return (
    <>
      <Eyebrow>ВАШ АККАУНТ</Eyebrow>
      <PageHeading>Создать аккаунт</PageHeading>
      <p className="text-sm leading-relaxed text-muted-foreground">
        Начните с личного аккаунта в Uptime.
      </p>
      <AuthForm
        register
        footer={
          <p className="mt-7 text-center text-sm leading-relaxed text-muted-foreground">
            Уже есть аккаунт?{' '}
            <Link
              className="rounded-sm text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring"
              href="/login"
            >
              Войти
            </Link>
          </p>
        }
      />
    </>
  );
}
