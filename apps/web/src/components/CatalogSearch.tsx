'use client';

import { useRouter } from 'next/navigation';
import { FormEvent, useState } from 'react';

export function CatalogSearch({ initialQuery }: { initialQuery: string }) {
  const router = useRouter();
  const [query, setQuery] = useState(initialQuery);

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const trimmed = query.trim();
    router.push(trimmed ? `/?q=${encodeURIComponent(trimmed)}` : '/');
  }

  return (
    <form className="catalog-search" onSubmit={handleSubmit}>
      <input
        type="search"
        placeholder="grep --project ..."
        value={query}
        onChange={(e) => setQuery(e.target.value)}
        aria-label="Filter listings by OSS project"
      />
      <button type="submit" className="btn secondary">run</button>
    </form>
  );
}
