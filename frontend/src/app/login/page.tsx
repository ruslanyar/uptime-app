import Link from 'next/link';
import { Eyebrow, PageHeading } from '@/components/page-shell';
import { AuthForm } from '@/components/auth-form';

export default function Login() {
  return (
    <>
      <Eyebrow>Ваш аккаунт</Eyebrow>
      <PageHeading>Войти</PageHeading>
      <p className="text-sm leading-relaxed text-muted-foreground">
        Рады видеть вас снова в Uptime.
      </p>
      <AuthForm
        footer={
          <p className="mt-7 text-center text-sm leading-relaxed text-muted-foreground">
            Нет аккаунта?{' '}
            <Link
              className="rounded-sm text-primary underline-offset-4 hover:underline focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-ring"
              href="/register"
            >
              Зарегистрироваться
            </Link>
          </p>
        }
      />
    </>
  );
}
