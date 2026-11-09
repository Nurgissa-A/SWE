'use client';

import { useId, type InputHTMLAttributes } from 'react';

type TextInputProps = Omit<InputHTMLAttributes<HTMLInputElement>, 'id'> & {
  label: string;
  id?: string;
  error?: string;
  helpText?: string;
};

export function TextInput({ label, id, error, helpText, className = '', ...props }: TextInputProps) {
  const generatedId = useId();
  const inputId = id ?? generatedId;
  const descriptionId = `${inputId}-description`;

  return (
    <div className="grid gap-2">
      <label htmlFor={inputId} className="text-sm font-semibold text-text">
        {label}{props.required ? <span className="ml-1 text-error" aria-hidden="true">*</span> : null}
      </label>
      <input
        {...props}
        id={inputId}
        aria-invalid={Boolean(error)}
        aria-describedby={error || helpText ? descriptionId : undefined}
        className={`min-h-12 w-full rounded-input border bg-surface px-4 text-sm text-text placeholder:text-text-muted/70 ${error ? 'border-error' : 'border-border'} disabled:cursor-not-allowed disabled:bg-background disabled:text-text-muted ${className}`}
      />
      {error ? <p id={descriptionId} className="text-sm text-error" role="alert">{error}</p> : null}
      {!error && helpText ? <p id={descriptionId} className="text-sm text-text-muted">{helpText}</p> : null}
    </div>
  );
}
