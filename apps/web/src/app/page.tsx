import { CatalogSearch } from '@/components/CatalogSearch';
import { Hero } from '@/components/Hero';
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
      <Hero />

      {summary && (
        <section className="section ledger-panel">
          <p className="section-label">public ledger</p>
          <LedgerSummary summary={summary} />
        </section>
      )}

      <section className="section" id="listings">
        <div className="section-header">
          <h2>active_listings</h2>
          <CatalogSearch initialQuery={q ?? ''} />
        </div>
        {listings.length === 0 ? (
          <p className="muted">
            {q ? `no results for "${q}"` : 'no active listings — check back soon'}
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
