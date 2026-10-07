'use client';
import { useEffect, useRef, useState } from 'react';
import { Plus } from 'lucide-react';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Button } from '@/components/ui/button';
import { Card } from '@/components/ui/card';
import { Alert } from '@/components/ui/alert';
import { monitorErrorMessage } from '@/lib/monitors/api';
import {
  validateMonitor,
  type IntervalUnit,
  type MonitorInput,
  type MonitorErrors,
} from '@/lib/monitors/validation';

export function MonitorForm({
  onCreate,
  onCancel,
  disabled = false,
}: {
  onCreate: (input: MonitorInput) => Promise<void>;
  onCancel: () => void;
  disabled?: boolean;
}) {
  const [url, setURL] = useState('');
  const [amount, setAmount] = useState('5');
  const [unit, setUnit] = useState<IntervalUnit>('minutes');
  const [errors, setErrors] = useState<MonitorErrors>({});
  const [message, setMessage] = useState('');
  const [pending, setPending] = useState(false);
  const submitting = useRef(false);
  const urlInput = useRef<HTMLInputElement>(null);
  useEffect(() => {
    urlInput.current?.focus();
  }, []);

  async function submit(event: React.SubmitEvent<HTMLFormElement>) {
    event.preventDefault();
    if (submitting.current || disabled) return;
    const result = validateMonitor(url, amount, unit);
    setErrors(result.errors);
    setMessage('');
    if (!result.input) return;
    submitting.current = true;
    setPending(true);
    try {
      await onCreate(result.input);
    } catch (error) {
      setMessage(monitorErrorMessage(error, true));
    } finally {
      submitting.current = false;
      setPending(false);
    }
  }

  return (
    <Card className="mt-6 border-primary/25 px-6 sm:px-8">
      <div>
        <h3 id="monitor-form-title" className="text-xl font-medium">
          Новый сайт
        </h3>
        <p className="mt-2 text-sm text-muted-foreground">
          Укажите адрес сайта и интервал опроса.
        </p>
      </div>
      <form aria-labelledby="monitor-form-title" noValidate onSubmit={submit}>
        <fieldset disabled={pending || disabled} className="space-y-5">
          <div className="space-y-2">
            <Label htmlFor="monitor-url">URL сайта</Label>
            <Input
              id="monitor-url"
              type="url"
              placeholder="https://example.com"
              value={url}
              onChange={(e) => {
                setURL(e.target.value);
                setErrors((previous) => ({ ...previous, url: undefined }));
              }}
              aria-invalid={!!errors.url}
              aria-describedby={errors.url ? 'monitor-url-error' : undefined}
              ref={urlInput}
            />
            {errors.url && (
              <p id="monitor-url-error" className="text-sm text-destructive">
                {errors.url}
              </p>
            )}
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div className="space-y-2">
              <Label htmlFor="monitor-interval">Интервал опроса</Label>
              <Input
                id="monitor-interval"
                type="number"
                step="any"
                min="0"
                value={amount}
                onChange={(e) => {
                  setAmount(e.target.value);
                  setErrors((previous) => ({
                    ...previous,
                    interval: undefined,
                  }));
                }}
                aria-invalid={!!errors.interval}
                aria-describedby={
                  errors.interval
                    ? 'monitor-interval-help monitor-interval-error'
                    : 'monitor-interval-help'
                }
              />
            </div>
            <div className="space-y-2">
              <Label htmlFor="monitor-unit">Единица измерения</Label>
              <select
                id="monitor-unit"
                value={unit}
                onChange={(e) => {
                  setUnit(e.target.value as IntervalUnit);
                  setErrors((previous) => ({
                    ...previous,
                    interval: undefined,
                  }));
                }}
                className="h-11 w-full rounded-md border border-input bg-card px-3 text-sm outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/50 disabled:opacity-50"
              >
                <option value="seconds">Секунды</option>
                <option value="minutes">Минуты</option>
                <option value="hours">Часы</option>
              </select>
            </div>
          </div>
          <p
            id="monitor-interval-help"
            className="text-xs text-muted-foreground"
          >
            От 1 минуты до 24 часов.
          </p>
          {errors.interval && (
            <p id="monitor-interval-error" className="text-sm text-destructive">
              {errors.interval}
            </p>
          )}
          {message && (
            <Alert className="border-destructive/30 bg-destructive/5 text-destructive">
              {message}
            </Alert>
          )}
          <div className="flex flex-wrap gap-3">
            <Button type="submit">
              <Plus aria-hidden="true" className="size-4" />
              {pending ? 'Создаём…' : 'Создать'}
            </Button>
            <Button type="button" variant="outline" onClick={onCancel}>
              Отмена
            </Button>
          </div>
        </fieldset>
      </form>
    </Card>
  );
}
