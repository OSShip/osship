'use client';

import { useEffect, useState } from 'react';
import { useParams, useRouter } from 'next/navigation';
import { api, fetchListing, formatPrice, getStoredUser, Listing } from '@/lib/api';
import { PayoutBreakdown } from '@/components';

export default function CheckoutPage() {
  const { listingId } = useParams<{ listingId: string }>();
  const router = useRouter();
  const [listing, setListing] = useState<Listing | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);
  const user = getStoredUser();

  useEffect(() => {
    fetchListing(listingId).then(setListing).catch(() => setError('Listing not found'));
  }, [listingId]);

  async function handleCheckout() {
    if (!user || !listing) return;
    setLoading(true);
    setError('');
    try {
      const enrollment = await api<{ id: string }>('/users/enrollments', {
        method: 'POST',
        body: JSON.stringify({ listing_id: listing.id }),
      });
      const fee = Math.round(listing.price_cents * 0.1);
      const checkout = await api<{ checkout_url: string }>('/payments/checkout', {
        method: 'POST',
        body: JSON.stringify({
          listing_id: listing.id,
          student_id: user.id,
          mentor_id: listing.mentor_id,
          enrollment_id: enrollment.id,
          amount_cents: listing.price_cents,
          success_url: `${window.location.origin}/dashboard/student?paid=1`,
          cancel_url: `${window.location.origin}/listings/${listing.id}`,
        }),
      });
      window.location.href = checkout.checkout_url;
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Checkout failed');
      setLoading(false);
    }
  }

  if (!listing) return <p>Loading...</p>;

  const fee = Math.round(listing.price_cents * 0.1);
  const payout = listing.price_cents - fee;

  return (
    <>
      <h1>Checkout — {listing.oss_project_name}</h1>
      <PayoutBreakdown gross={listing.price_cents} fee={fee} payout={payout} />
      {!user && <p className="error">Please <a href="/login">login</a> to enroll.</p>}
      {error && <p className="error">{error}</p>}
      <button className="btn" onClick={handleCheckout} disabled={!user || loading}>
        {loading ? 'Processing...' : `Pay ${formatPrice(listing.price_cents)}`}
      </button>
    </>
  );
}
