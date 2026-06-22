import { LedgerSummary } from '@/components';
import { serverFetchListings, serverFetchListing, serverFetchPayoutSummary, formatPrice } from '@/lib/api';
import Link from 'next/link';
import { notFound } from 'next/navigation';

export default async function ListingDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const listing = await serverFetchListing(id);
  if (!listing) notFound();

  const slotsLeft = listing.total_slots - listing.filled_slots;
  const platformFee = Math.round(listing.price_cents * 0.1);
  const mentorPayout = listing.price_cents - platformFee;

  return (
    <>
      <h1>{listing.oss_project_name}</h1>
      <p className="muted"><a href={listing.oss_repo_url} target="_blank" rel="noopener noreferrer">{listing.oss_repo_url}</a></p>
      <p>{listing.description}</p>
      <ul className="stats">
        <li>Price: <strong>{formatPrice(listing.price_cents)}</strong></li>
        <li>Duration: {listing.duration_weeks} weeks</li>
        <li>Slots available: {slotsLeft} / {listing.total_slots}</li>
      </ul>
      <div className="section">
        <h3>What you pay for</h3>
        <p className="muted">Live sessions with a project maintainer, structured mentorship, progress tracking.</p>
        <ul className="stats">
          <li>Mentor receives: {formatPrice(mentorPayout)}</li>
          <li>Platform fee (10%): {formatPrice(platformFee)}</li>
        </ul>
      </div>
      {slotsLeft > 0 && (
        <Link href={`/checkout/${listing.id}`} className="btn">Enroll & Pay</Link>
      )}
    </>
  );
}
