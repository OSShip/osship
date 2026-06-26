import { CatalogSearch } from '@/components/CatalogSearch';
import { ListingCard, LedgerSummary } from '@/components';
import { serverFetchListings, serverFetchPayoutSummary } from '@/lib/api';

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string }>;
}) {
  const { q } = await searchParams;
  const [listings, summary] = await Promise.all([
    serverFetchListings(q),
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
        <div className="section-header">
          <h2>Active Listings</h2>
          <CatalogSearch initialQuery={q ?? ''} />
        </div>
        {listings.length === 0 ? (
          <p className="muted">
            {q ? `No listings matching "${q}".` : 'No active listings yet. Check back soon.'}
          </p>
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
