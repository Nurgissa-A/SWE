'use client';

import type { FormEvent, FormHTMLAttributes, ReactNode } from 'react';

type FormProps = Omit<FormHTMLAttributes<HTMLFormElement>, 'onSubmit'> & {
  onSubmit: (event: FormEvent<HTMLFormElement>) => void | Promise<void>;
  children: ReactNode;
};

export function Form({ onSubmit, children, ...props }: FormProps) {
  return (
    <form
      {...props}
      noValidate
      onSubmit={(event) => {
        event.preventDefault();
        void onSubmit(event);
      }}
    >
      {children}
    </form>
  );
}
