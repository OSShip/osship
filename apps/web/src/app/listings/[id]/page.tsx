import { formatPrice, serverFetchListing } from '@/lib/api';
import type { Metadata } from 'next';
import Link from 'next/link';
import { notFound } from 'next/navigation';

export async function generateMetadata({
  params,
}: {
  params: Promise<{ id: string }>;
}): Promise<Metadata> {
  const { id } = await params;
  const listing = await serverFetchListing(id);
  if (!listing) return { title: 'Listing not found — OSShip' };
  return {
    title: `${listing.oss_project_name} — OSShip`,
    description: listing.description.slice(0, 160),
  };
}

export default async function ListingDetailPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = await params;
  const listing = await serverFetchListing(id);
  if (!listing) notFound();

  const slotsLeft = listing.total_slots - listing.filled_slots;
  const platformFee = Math.round(listing.price_cents * 0.1);
  const mentorPayout = listing.price_cents - platformFee;
  const mentorLabel = listing.mentor_display_name || listing.mentor_github_username || 'Verified mentor';

  return (
    <>
      <nav className="breadcrumb">
        <Link href="/">← Back to catalog</Link>
      </nav>
      <h1>{listing.oss_project_name}</h1>
      <p className="muted">
        Mentored by <strong>{mentorLabel}</strong>
        {listing.mentor_github_username && (
          <> · <a href={`https://github.com/${listing.mentor_github_username}`} target="_blank" rel="noopener noreferrer">@{listing.mentor_github_username}</a></>
        )}
      </p>
      <p className="muted">
        <a href={listing.oss_repo_url} target="_blank" rel="noopener noreferrer">{listing.oss_repo_url}</a>
      </p>
      <p>{listing.description}</p>
      <ul className="stats">
        <li>Price: <strong>{formatPrice(listing.price_cents)}</strong></li>
        <li>Duration: {listing.duration_weeks} weeks</li>
        <li>Slots available: {slotsLeft} / {listing.total_slots}</li>
        <li>Status: {listing.status}</li>
      </ul>
      <div className="section">
        <h3>What you pay for</h3>
        <p className="muted">Live sessions with a project maintainer, structured mentorship, progress tracking.</p>
        <ul className="stats">
          <li>Mentor receives: {formatPrice(mentorPayout)}</li>
          <li>Platform fee (10%): {formatPrice(platformFee)}</li>
        </ul>
      </div>
      {slotsLeft > 0 && listing.status === 'active' && (
        <Link href={`/checkout/${listing.id}`} className="btn">Enroll & Pay</Link>
      )}
    </>
  );
}
