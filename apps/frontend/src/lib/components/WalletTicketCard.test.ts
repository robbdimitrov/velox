// @vitest-environment jsdom
import '@testing-library/jest-dom/vitest';
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/svelte';
import WalletTicketCard from './WalletTicketCard.svelte';
import type { WalletTicket } from '$lib/api/types';

afterEach(() => cleanup());

function makeTicket(overrides: Partial<WalletTicket> = {}): WalletTicket {
  return {
    ticket_id: 'tkt_test',
    event_id: 'evt_test',
    section_id: 'S1',
    event: 'Neon Riot Live',
    venue: 'Velox Arena',
    seat: 'A-12',
    status: 'ISSUED',
    transfer_status: 'LOCKED',
    qr_token: 'a1b2c3d4e5f6g7h8i9j0',
    qr_token_expires_at: '2026-08-15T20:30:00Z',
    ledger: [],
    ...overrides
  };
}

describe('WalletTicketCard', () => {
  it('renders an issued ticket with QR-derived display and entry-token-expiry text', () => {
    render(WalletTicketCard, { ticket: makeTicket() });

    expect(screen.getByText('Neon Riot Live')).toBeInTheDocument();
    expect(screen.getByText(/Velox Arena/)).toBeInTheDocument();
    expect(
      screen.getByLabelText('Signed ticket token pattern')
    ).toBeInTheDocument();
    expect(screen.getByText(/Entry token expires/)).toBeInTheDocument();
  });

  it('does not dim an issued ticket', () => {
    render(WalletTicketCard, { ticket: makeTicket({ status: 'ISSUED' }) });

    const article = screen.getByText('Neon Riot Live').closest('article');
    expect(article?.className).not.toContain('opacity-50');
    expect(article?.className).not.toContain('grayscale');
  });

  it('applies cancelled styling and exposes no active transfer control', () => {
    render(WalletTicketCard, {
      ticket: makeTicket({ status: 'CANCELLED', transfer_status: 'LOCKED' })
    });

    const article = screen.getByText('Neon Riot Live').closest('article');
    expect(article?.className).toContain('opacity-50');
    expect(article?.className).toContain('grayscale');
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });
});
