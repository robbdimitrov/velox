import { describe, expect, it } from 'vitest';
import { load } from './+layout.server';

describe('organizer layout server load', () => {
  it('redirects unauthenticated visitors to login', async () => {
    await expect(load(loadEvent(null))).rejects.toMatchObject({
      status: 303,
      location: '/login'
    });
  });

  it('rejects authenticated non-organizers with a forbidden error', async () => {
    await expect(
      load(
        loadEvent({
          id: 'usr_1',
          email: 'fan@velox.local',
          role: 'reserver'
        })
      )
    ).rejects.toMatchObject({ status: 403 });
  });

  it('allows organizers through', async () => {
    const user = {
      id: 'usr_2',
      email: 'organizer@velox.local',
      role: 'organizer' as const
    };
    const result = await load(loadEvent(user));
    if (!result) throw new Error('expected organizer layout result');
    expect(result.user).toEqual(user);
  });
});

function loadEvent(user: App.Locals['user']): Parameters<typeof load>[0] {
  return { parent: async () => ({ user }) } as unknown as Parameters<
    typeof load
  >[0];
}
