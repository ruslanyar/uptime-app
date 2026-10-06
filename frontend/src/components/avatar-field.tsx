'use client';
import { useEffect, useRef, useState } from 'react';
import Image from 'next/image';
import { Trash2 } from 'lucide-react';
import { Avatar } from './avatar';
import { Button } from '@/components/ui/button';

export function AvatarField({
  url,
  file,
  removed,
  disabled,
  onChange,
  onRemove,
}: {
  url?: string;
  file?: File;
  removed: boolean;
  disabled: boolean;
  onChange: (file?: File) => void;
  onRemove: () => void;
}) {
  const input = useRef<HTMLInputElement>(null);
  const [preview, setPreview] = useState('');
  const [error, setError] = useState('');
  const [dragging, setDragging] = useState(false);
  useEffect(() => {
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => setPreview(String(reader.result));
    reader.readAsDataURL(file);
    return () => {
      reader.onload = null;
      reader.abort();
    };
  }, [file]);
  function select(files: FileList | File[]) {
    if (disabled || files.length === 0) return;
    const selected = files[0];
    if (files.length !== 1) {
      setError('Выберите одно изображение.');
    } else if (
      !['image/jpeg', 'image/png', 'image/webp', 'image/gif'].includes(
        selected.type,
      )
    ) {
      setError('Выберите изображение JPG, PNG, WebP или GIF.');
    } else if (selected.size === 0 || selected.size > 500 * 1024) {
      setError('Размер аватара должен быть от 1 байта до 500 КБ.');
    } else {
      setError('');
      onChange(selected);
    }
  }
  return (
    <div className="space-y-2">
      <div
        aria-label="Поле аватара"
        onDragOver={(event) => {
          event.preventDefault();
          if (!disabled) setDragging(true);
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(event) => {
          event.preventDefault();
          setDragging(false);
          select(event.dataTransfer.files);
        }}
        className={`flex flex-wrap items-center gap-4 rounded-lg border border-dashed p-4 ${dragging ? 'border-primary bg-primary/5' : 'border-border'}`}
      >
        {file && preview ? (
          <Image
            src={preview}
            alt="Предпросмотр аватара"
            width={80}
            height={80}
            className="size-20 rounded-full object-cover"
          />
        ) : (
          <Avatar url={removed ? undefined : url} />
        )}
        <div className="min-w-0 flex-1 space-y-2">
          <p className="text-sm">Перетащите изображение сюда</p>
          <div className="flex flex-wrap items-center gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={disabled}
              onClick={() => input.current?.click()}
            >
              Выбрать файл
            </Button>
            {file && (
              <Button
                type="button"
                variant="outline"
                disabled={disabled}
                onClick={() => {
                  onChange(undefined);
                  setError('');
                }}
              >
                Отменить выбор
              </Button>
            )}
            {url && !removed && (
              <Button
                type="button"
                variant="outline"
                disabled={disabled}
                size="icon"
                className="size-11"
                aria-label="Удалить аватар"
                title="Удалить аватар"
                onClick={() => {
                  onRemove();
                  setError('');
                }}
              >
                <Trash2 aria-hidden="true" />
              </Button>
            )}
          </div>
          {file && (
            <p className="text-xs wrap-anywhere text-muted-foreground">
              {file.name}
            </p>
          )}
        </div>
        <input
          ref={input}
          type="file"
          accept="image/jpeg,image/png,image/webp,image/gif"
          aria-label="Выбрать аватар"
          aria-describedby="avatar-hint avatar-error"
          disabled={disabled}
          className="sr-only"
          onChange={(event) => {
            if (event.target.files) select(event.target.files);
            event.target.value = '';
          }}
        />
      </div>
      <p id="avatar-hint" className="text-xs text-muted-foreground">
        JPG, PNG, WebP или GIF, не более 500 КБ.
      </p>
      {removed && (
        <p className="text-xs text-muted-foreground">
          Аватар будет удалён после сохранения профиля.
        </p>
      )}
      <p
        id="avatar-error"
        role={error ? 'alert' : undefined}
        className="text-xs text-destructive"
      >
        {error}
      </p>
    </div>
  );
}
