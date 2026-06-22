import type { Listing } from '@/lib/api';
import { formatPrice } from '@/lib/api';
import Link from 'next/link';

export function ListingCard({ listing }: { listing: Listing }) {
  const slotsLeft = listing.total_slots - listing.filled_slots;
  return (
    <Link href={`/listings/${listing.id}`} className="card">
      <h3>{listing.oss_project_name}</h3>
      <p className="muted">{listing.description.slice(0, 120)}...</p>
      <div className="meta">
        <span>{formatPrice(listing.price_cents)}</span>
        <span>{listing.duration_weeks} weeks</span>
        <span>{slotsLeft} slots left</span>
      </div>
    </Link>
  );
}

export function LedgerSummary({ summary }: { summary: { total_gross_cents: number; total_mentor_payout_cents: number; total_platform_fee_cents: number; transaction_count: number } }) {
  return (
    <div className="card">
      <h3>Public Payout Transparency</h3>
      <p className="muted">Aggregated, anonymized payout data</p>
      <ul className="stats">
        <li><strong>{formatPrice(summary.total_gross_cents)}</strong> total processed</li>
        <li><strong>{formatPrice(summary.total_mentor_payout_cents)}</strong> to mentors</li>
        <li><strong>{formatPrice(summary.total_platform_fee_cents)}</strong> platform fees</li>
        <li><strong>{summary.transaction_count}</strong> transactions</li>
      </ul>
    </div>
  );
}

export function PayoutBreakdown({ gross, fee, payout }: { gross: number; fee: number; payout: number }) {
  return (
    <div className="card">
      <h3>Payment Breakdown</h3>
      <ul className="stats">
        <li>Total: {formatPrice(gross)}</li>
        <li>Platform fee: {formatPrice(fee)}</li>
        <li>Mentor receives: <strong>{formatPrice(payout)}</strong></li>
      </ul>
    </div>
  );
}

export function JitsiEmbed({ url }: { url: string }) {
  return (
    <div className="jitsi">
      <iframe src={url} allow="camera; microphone; fullscreen" title="Jitsi session" />
      <a href={url} target="_blank" rel="noopener noreferrer" className="btn secondary">Open in new tab</a>
    </div>
  );
}
