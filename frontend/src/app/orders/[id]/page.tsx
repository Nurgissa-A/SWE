import { OrderPage } from '@/components/orders/OrderPage';

type Props = { params: Promise<{ id: string }>; searchParams: Promise<{ new?: string }> };

export default async function OrderDetailsPage({ params, searchParams }: Props) {
  const { id } = await params;
  const query = await searchParams;
  return <OrderPage id={id} justBought={query.new === '1'} />;
}
