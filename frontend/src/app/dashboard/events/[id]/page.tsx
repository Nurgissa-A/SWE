import { ManageEvent } from '@/components/organizer/ManageEvent';

type Props = { params: Promise<{ id: string }>; searchParams: Promise<{ tab?: string }> };

export default async function ManageEventPage({ params, searchParams }: Props) {
  const { id } = await params;
  const { tab } = await searchParams;
  return <ManageEvent id={id} initialTab={tab} />;
}
