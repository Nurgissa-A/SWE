// src/components/Form.tsx

type FormProps = {
  onSubmit: () => void;
  children: React.ReactNode;
};

export function Form({ onSubmit, children }: FormProps) {
  function handleSubmit(e: React.FormEvent) {
    e.preventDefault();
    onSubmit();
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      {children}
    </form>
  );
}