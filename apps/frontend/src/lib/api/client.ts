import type {
  CheckoutRequest,
  CheckoutResponse,
  DiscoveryResponse,
  EventAnnouncement,
  EventSection,
  EventSummary,
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
    checkout(
      body: CheckoutRequest,
      idempotencyKey: string,
      reservationToken: string
    ) {
      return request<{ order_id?: string; status?: string }>(
        `/reservations/${encodeURIComponent(body.reservation_id)}/confirm`,
        {
          method: 'POST',
          headers: {
            'Idempotency-Key': idempotencyKey,
            'Reservation-Token': reservationToken
          }
        }
      ).then((res) => mapCheckout(body.reservation_id, res));
    },
    cancelReservation(
      reservationId: string,
      idempotencyKey: string,
      reservationToken: string
    ) {
      return request<{ order_id?: string; status?: string }>(
        `/reservations/${encodeURIComponent(reservationId)}/cancel`,
        {
          method: 'POST',
          headers: {
            'Idempotency-Key': idempotencyKey,
            'Reservation-Token': reservationToken
          }
        }
      ).then((res) => mapCheckout(reservationId, res));
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
  image_key?: string;
  image_url?: string;
  venue?: string;
  city?: string;
  starts_at?: string;
  sale_starts_at?: string;
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
  price_cents: number;
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
  event_id: string;
  section_id: string;
  seat_ids: string[];
  status: 'PENDING' | 'CONFIRMED' | 'EXPIRED' | 'FAILED';
  total_cents: number;
  expires_at_server_ms?: number;
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
  const startsAt = event.starts_at ?? event.sale_starts_at ?? '';
  const saleStartsAt = event.sale_starts_at ?? startsAt;
  const sections = mapEventSections(event);

  return {
    id: event.id,
    title: event.name ?? event.title ?? event.id,
    description: event.description,
    venue: event.venue ?? 'Venue pending',
    city: event.city ?? '',
    category: event.category ?? 'Live',
    image_key: event.image_key,
    image_url:
      localImageURL(event.image_url) ?? imageURLForKey(event.image_key),
    starts_at: startsAt,
    sale_starts_at: saleStartsAt,
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
  if (seatsOpen <= 0) return 'SOLD_OUT';
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

function imageURLForKey(imageKey: string | undefined) {
  if (!imageKey) return '/event-midnight-array.svg';

  const localImages: Record<string, string> = {
    'event-midnight-array': '/event-midnight-array.svg',
    'midnight-array': '/event-midnight-array.svg',
    'event-final-whistle': '/event-final-whistle.svg',
    'final-whistle': '/event-final-whistle.svg',
    'event-zero-hour': '/event-zero-hour.svg',
    'zero-hour': '/event-zero-hour.svg'
  };

  if (localImages[imageKey]) return localImages[imageKey];
  if (/^[a-z0-9_-]+\.svg$/i.test(imageKey)) return `/${imageKey}`;
  return '/event-midnight-array.svg';
}

function localImageURL(imageURL: string | undefined) {
  if (!imageURL) return undefined;
  if (/^\/[a-z0-9/_-]+\.(svg|png|jpe?g|webp)$/i.test(imageURL)) {
    return imageURL;
  }
  return undefined;
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
      price_cents: seat.price_cents,
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
    status === 'SOLD' ||
    status === 'SELECTED' ||
    status === 'UNKNOWN'
  ) {
    return status;
  }
  return 'UNKNOWN';
}

function mapReservation(body: { order: GatewayOrder }): ReserveOrderResponse {
  const order = body.order;
  const subtotal = order.total_cents;
  return {
    order_id: order.id,
    reservation_id: order.reservation_id,
    reservation_token: order.reservation_id,
    expires_at_server_ms: order.expires_at_server_ms ?? Date.now(),
    server_time_ms: Date.now(),
    version: 1,
    seats: order.seat_ids.map((seat_id) => ({
      seat_id,
      price_cents: Math.round(subtotal / order.seat_ids.length)
    })),
    fees_cents: 0,
    total_cents: order.total_cents
  };
}

function mapCheckout(
  reservationId: string,
  body: { order_id?: string; status?: string }
): CheckoutResponse {
  const status: CheckoutResponse['status'] =
    body.status === 'CANCELLED' ? body.status : 'CONFIRMED';

  return {
    order_id: body.order_id ?? reservationId.replace('res_', ''),
    status,
    wallet_ticket_ids: []
  };
}
