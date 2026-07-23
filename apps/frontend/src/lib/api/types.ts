export type EventSummary = {
  id: string;
  title: string;
  description?: string;
  venue: string;
  city: string;
  category: string;
  starts_at?: string;
  section_ids?: string[];
  sections?: EventSection[];
  remaining_bucket: 'LOW' | 'MEDIUM' | 'HIGH' | 'FULL';
  demand_score: number;
  projection_lag_ms: number;
  status?: string;
};

export type OrganizerMetrics = {
  totalReservations: number;
  activeHolds: number;
  seatsRemaining: number;
  demandScore: number;
  projectionLagMs: number;
  sectionAvailability: Record<string, number>;
};

export type EventSection = {
  id: string;
  name: string;
};

export type EventAnnouncement = {
  id: string;
  event_id: string;
  title: string;
  body: string;
  severity: 'INFO' | 'SCHEDULE_CHANGE' | 'CANCELLATION';
  created_at: string;
};

export type DiscoveryResponse = {
  events: EventSummary[];
  featured: EventSummary[];
  meta: {
    projection_lag_ms: number;
    cache_status: string;
  };
};

export type SeatStatus =
  'AVAILABLE' | 'SELECTED' | 'HELD' | 'RESERVED' | 'UNKNOWN';

export type Seat = {
  index: number;
  seat_id: string;
  section_id: string;
  row: string;
  x: number;
  y: number;
  accessibility: boolean;
  status: SeatStatus;
  version: number;
  expires_at_server_ms?: number;
};

export type SeatSnapshot = {
  event_id: string;
  section_id: string;
  server_time_ms: number;
  snapshot_age_ms: number;
  projection_lag_ms: number;
  seats: Seat[];
};

export type SeatDelta = {
  event_id: string;
  section_id: string;
  seat_id: string;
  status: Exclude<SeatStatus, 'SELECTED'>;
  version: number;
  expires_at_server_ms?: number;
};

export type ReserveOrderRequest = {
  event_id: string;
  section_id: string;
  seat_ids: string[];
  expected_versions: Record<string, number>;
};

export type ReserveOrderResponse = {
  order_id: string;
  reservation_id: string;
  reservation_token: string;
  expires_at_server_ms: number;
  server_time_ms: number;
  version: number;
  seats: Array<{ seat_id: string }>;
};

export type ReservationConfirmationRequest = {
  reservation_id: string;
  terms_accepted: boolean;
};

export type ReservationConfirmationResponse = {
  order_id: string;
  status: 'CONFIRMED' | 'CANCELLED';
  wallet_ticket_ids: string[];
};

export type WalletTicket = {
  ticket_id: string;
  event_id: string;
  section_id: string;
  event: string;
  venue: string;
  seat: string;
  status: 'ISSUED' | 'TRANSFERRED' | 'USED' | 'UPGRADED' | 'CANCELLED';
  transfer_status: 'LOCKED' | 'AVAILABLE' | 'PENDING';
  qr_token: string;
  qr_token_expires_at: string;
  ledger: Array<{
    event_type: string;
    timestamp: string;
    actor: string;
    correlation_id: string;
  }>;
};

export type WalletResponse = {
  verification_state: 'REQUIRED' | 'PENDING' | 'VERIFIED';
  tickets: WalletTicket[];
};
