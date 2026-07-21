import type {
  DiscoveryResponse,
  EventAnnouncement,
  EventSection,
  EventSummary,
  ReservationConfirmationRequest,
  ReservationConfirmationResponse,
  ReserveOrderRequest,
  ReserveOrderResponse,
  Seat,
  SeatSnapshot,
  WalletResponse
} from './types';

type Fetch = typeof fetch;

export class GatewayError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly code?: string
  ) {
    super(message);
  }
}

export type GatewayClient = ReturnType<typeof createGatewayClient>;

export function createGatewayClient(
  fetcher: Fetch,
  baseURL = 'http://localhost:8080'
) {
  const apiBase = baseURL.replace(/\/$/, '');

  async function request<T>(path: string, init?: RequestInit): Promise<T> {
    const headers = new Headers(init?.headers);
    headers.set('Accept', 'application/json');
    if (init?.body && !headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json');
    }

    const response = await fetcher(`${apiBase}${path}`, {
      ...init,
      headers
    });

    if (!response.ok) {
      const body = await response.json().catch(() => ({}));
      throw new GatewayError(
        body.message ?? body.error ?? 'Gateway request failed',
        response.status,
        body.code ?? body.error
      );
    }

    return response.json() as Promise<T>;
  }

  return {
    apiBase,
    listEvents(params: URLSearchParams) {
      // Caching is set on the response by apigateway (discoveryCacheControl),
      // not the request — Cache-Control on an outbound fetch request has no
      // effect on CDN/browser caching of the response.
      return request<{ events: GatewayEvent[]; projection_lag_ms?: number }>(
        `/events?${params.toString()}`
      ).then(mapDiscovery);
    },
    getEvent(eventID: string) {
      return request<{ event: GatewayEvent; projection_lag_ms?: number }>(
        `/events/${encodeURIComponent(eventID)}`
      ).then((body) =>
        mapEventSummary(
          body.event,
          body.projection_lag_ms ?? body.event.projection_lag_ms ?? 0
        )
      );
    },
    getSeatSnapshot(eventID: string, sectionID: string) {
      return request<GatewaySeatSnapshot>(
        `/events/${encodeURIComponent(eventID)}/sections/${encodeURIComponent(sectionID)}/seats`
      ).then((body) => mapSeatSnapshot(eventID, sectionID, body));
    },
    reserveSeats(body: ReserveOrderRequest, idempotencyKey: string) {
      return request<{ order: GatewayOrder }>('/reservations', {
        method: 'POST',
        headers: {
          'Idempotency-Key': idempotencyKey
        },
        body: JSON.stringify({
          event_id: body.event_id,
          section_id: body.section_id,
          seat_ids: body.seat_ids
        })
      }).then(mapReservation);
    },
    confirmReservation(
      body: ReservationConfirmationRequest,
      idempotencyKey: string,
      reservationToken: string
    ) {
      return request<GatewayReservationConfirmationResponse>(
        `/reservations/${encodeURIComponent(body.reservation_id)}/confirm`,
        {
          method: 'POST',
          headers: {
            'Idempotency-Key': idempotencyKey,
            'Reservation-Token': reservationToken
          }
        }
      ).then(mapReservationConfirmation);
    },
    cancelReservation(
      reservationId: string,
      idempotencyKey: string,
      reservationToken: string
    ) {
      return request<GatewayReservationConfirmationResponse>(
        `/reservations/${encodeURIComponent(reservationId)}/cancel`,
        {
          method: 'POST',
          headers: {
            'Idempotency-Key': idempotencyKey,
            'Reservation-Token': reservationToken
          }
        }
      ).then(mapReservationConfirmation);
    },
    wallet() {
      return request<WalletResponse>('/wallet/tickets');
    },
    getAnnouncements(eventId: string) {
      return request<{ announcements: EventAnnouncement[] }>(
        `/events/${encodeURIComponent(eventId)}/announcements`
      ).then((body) => body.announcements);
    },
    postAnnouncement(
      eventId: string,
      body: { title: string; body: string; severity?: string }
    ) {
      return request<EventAnnouncement>(
        `/organizer/events/${encodeURIComponent(eventId)}/announcements`,
        {
          method: 'POST',
          body: JSON.stringify(body)
        }
      );
    },
    cancelEvent(eventId: string) {
      return request<{
        event_id: string;
        status: string;
        cancelled_orders: number;
      }>(`/organizer/events/${encodeURIComponent(eventId)}/cancel`, {
        method: 'POST'
      });
    },
    tickerURL(params = new URLSearchParams()) {
      return `${apiBase}/events/evt_neon_riot/stream?${params.toString()}`;
    },
    seatSseURL(eventID: string, sectionID: string) {
      return `${apiBase}/events/${encodeURIComponent(eventID)}/stream?section_id=${encodeURIComponent(sectionID)}`;
    }
  };
}

export function createIdempotencyKey() {
  return crypto.randomUUID();
}

type GatewayEvent = {
  id: string;
  name?: string;
  title?: string;
  description?: string;
  category?: string;
  venue?: string;
  city?: string;
  starts_at?: string;
  section_ids?: string[];
  sections?: GatewaySection[];
  seats_total?: number;
  seats_open?: number;
  remaining_bucket?: EventSummary['remaining_bucket'];
  demand_score?: number;
  projection_lag_ms?: number;
  status?: string;
};

type GatewaySection =
  | string
  | {
      id?: string;
      section_id?: string;
      name?: string;
    };

type GatewaySeat = {
  seat_id: string;
  section_id: string;
  row: string;
  number: number;
  x?: number;
  y?: number;
  accessibility?: boolean;
  status: string;
  version: number;
  expires_at_server_ms?: number;
};

type GatewaySeatSnapshot = {
  seats: GatewaySeat[];
  server_time_ms?: number;
  snapshot_age_ms?: number;
  projection_lag_ms?: number;
};

type GatewayOrder = {
  id: string;
  reservation_id: string;
  reservation_token: string;
  event_id: string;
  section_id: string;
  seat_ids: string[];
  seats?: Array<{ seat_id: string }>;
  status: 'PENDING' | 'CONFIRMED' | 'EXPIRED' | 'FAILED';
  expires_at_server_ms?: number;
  server_time_ms?: number;
};

type GatewayReservationConfirmationResponse = {
  order_id: string;
  status: 'CONFIRMED' | 'CANCELLED';
  wallet_ticket_ids: string[];
};

function mapDiscovery(body: {
  events: GatewayEvent[];
  projection_lag_ms?: number;
}): DiscoveryResponse {
  const projectionLag = body.projection_lag_ms ?? 0;
  const events = body.events.map((event) =>
    mapEventSummary(event, event.projection_lag_ms ?? projectionLag)
  );
  return {
    events,
    featured: events,
    meta: {
      projection_lag_ms: body.projection_lag_ms ?? 0,
      cache_status: 'gateway'
    }
  };
}

function mapEventSummary(
  event: GatewayEvent,
  projectionLag: number
): EventSummary {
  const startsAt = event.starts_at ?? '';
  const sections = mapEventSections(event);

  return {
    id: event.id,
    title: event.name ?? event.title ?? event.id,
    description: event.description,
    venue: event.venue ?? 'Venue pending',
    city: event.city ?? '',
    category: event.category ?? 'Live',
    starts_at: startsAt,
    section_ids: sections.map((section) => section.id),
    sections,
    remaining_bucket:
      event.remaining_bucket ?? remainingBucketFromOpenSeats(event.seats_open),
    demand_score: event.demand_score ?? 0,
    projection_lag_ms: projectionLag,
    status: event.status
  };
}

function remainingBucketFromOpenSeats(
  seatsOpen: number | undefined
): EventSummary['remaining_bucket'] {
  if (seatsOpen === undefined) return 'HIGH';
  if (seatsOpen <= 0) return 'FULL';
  if (seatsOpen < 20) return 'LOW';
  if (seatsOpen < 80) return 'MEDIUM';
  return 'HIGH';
}

function mapEventSections(event: GatewayEvent): EventSection[] {
  const sections = [
    ...(event.sections ?? []).map(normalizeSection),
    ...(event.section_ids ?? []).map((id) => ({ id, name: id }))
  ];
  const byID = new Map<string, EventSection>();
  for (const section of sections) {
    if (section.id) byID.set(section.id, section);
  }
  return [...byID.values()];
}

function normalizeSection(section: GatewaySection): EventSection {
  if (typeof section === 'string') return { id: section, name: section };
  const id = section.id ?? section.section_id ?? '';
  return { id, name: section.name ?? id };
}

function mapSeatSnapshot(
  eventID: string,
  sectionID: string,
  body: GatewaySeatSnapshot
): SeatSnapshot {
  const seats = body.seats.map((seat, index): Seat => {
    const col = index % 10;
    const row = Math.floor(index / 10);
    return {
      index,
      seat_id: seat.seat_id,
      section_id: seat.section_id,
      row: seat.row,
      x: seat.x ?? 44 + col * 42,
      y: seat.y ?? 42 + row * 42,
      accessibility: seat.accessibility ?? (col === 0 || col === 9),
      status: mapSeatStatus(seat.status),
      version: seat.version,
      expires_at_server_ms: seat.expires_at_server_ms
    };
  });
  return {
    event_id: eventID,
    section_id: sectionID,
    server_time_ms: body.server_time_ms ?? Date.now(),
    snapshot_age_ms: body.snapshot_age_ms ?? 0,
    projection_lag_ms: body.projection_lag_ms ?? 0,
    seats
  };
}

function mapSeatStatus(status: string): Seat['status'] {
  if (
    status === 'AVAILABLE' ||
    status === 'HELD' ||
    status === 'RESERVED' ||
    status === 'SELECTED' ||
    status === 'UNKNOWN'
  ) {
    return status;
  }
  return 'UNKNOWN';
}

function mapReservation(body: { order: GatewayOrder }): ReserveOrderResponse {
  const order = body.order;
  if (!order.reservation_token) {
    throw new GatewayError('reservation_token missing', 502, 'upstream_error');
  }
  const seats =
    order.seats?.map((seat) => ({
      seat_id: seat.seat_id
    })) ?? [];
  return {
    order_id: order.id,
    reservation_id: order.reservation_id,
    reservation_token: order.reservation_token,
    expires_at_server_ms: order.expires_at_server_ms ?? 0,
    server_time_ms: order.server_time_ms ?? Date.now(),
    version: 1,
    seats
  };
}

function mapReservationConfirmation(
  body: GatewayReservationConfirmationResponse
): ReservationConfirmationResponse {
  if (
    !body.order_id ||
    (body.status !== 'CONFIRMED' && body.status !== 'CANCELLED') ||
    !Array.isArray(body.wallet_ticket_ids)
  ) {
    throw new GatewayError(
      'reservation confirmation response incomplete',
      502,
      'upstream_error'
    );
  }
  return {
    order_id: body.order_id,
    status: body.status,
    wallet_ticket_ids: body.wallet_ticket_ids
  };
}
