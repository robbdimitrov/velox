import { describe, expect, it } from 'vitest';
import { mockDiscovery } from '$lib/api/mock';
import { load } from './+page';

describe('discovery page load', () => {
  it('uses mock discovery data when the gateway proxy is unavailable', async () => {
    const fetcher = async () => new Response('Gateway error', { status: 502 });

    const result = await load(
      loadEvent(fetcher as typeof fetch, new URL('http://localhost/'))
    );
    if (!result) throw new Error('expected discovery load result');

    expect(result.discovery).toBe(mockDiscovery);
    expect(result.tickerURL).toContain('city=all');
  });

  it('does not hide non-proxy gateway failures behind mock data', async () => {
    const fetcher = async () =>
      new Response(JSON.stringify({ message: 'Gateway request failed' }), {
        status: 500,
        headers: { 'Content-Type': 'application/json' }
      });

    await expect(
      load(loadEvent(fetcher as typeof fetch, new URL('http://localhost/')))
    ).rejects.toMatchObject({ status: 500 });
  });
});

function loadEvent(
  fetcher: typeof fetch,
  url: URL
): Parameters<typeof load>[0] {
  return { fetch: fetcher, url } as unknown as Parameters<typeof load>[0];
}
