import { ListingCard, LedgerSummary } from '@/components';
import { serverFetchListings, serverFetchPayoutSummary } from '@/lib/api';

export default async function HomePage() {
  const [listings, summary] = await Promise.all([
    serverFetchListings(),
    serverFetchPayoutSummary(),
  ]);

  return (
    <>
      <section className="hero">
        <h1>Open Source Mentorship Platform</h1>
        <p className="muted">
          Find structured mentorship on real OSS projects. Transparent payouts, live sessions, verifiable progress.
        </p>
      </section>

      {summary && (
        <section className="section">
          <LedgerSummary summary={summary} />
        </section>
      )}

      <section className="section">
        <h2>Active Listings</h2>
        {listings.length === 0 ? (
          <p className="muted">No active listings yet. Check back soon.</p>
        ) : (
          <div className="grid">
            {listings.map((l) => (
              <ListingCard key={l.id} listing={l} />
            ))}
          </div>
        )}
      </section>
    </>
  );
}
