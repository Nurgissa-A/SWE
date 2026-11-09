import { Spinner } from '@/components/ui/Spinner';

type LoadingScreenProps = { label?: string };

export function LoadingScreen({ label = 'Loading your workspace' }: LoadingScreenProps) {
  return (
    <main className="grid min-h-[50vh] place-items-center px-4">
      <Spinner label={label} />
    </main>
  );
}
