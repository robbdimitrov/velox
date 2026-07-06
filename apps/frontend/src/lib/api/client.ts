import type {
  CheckoutRequest,
  CheckoutResponse,
  DiscoveryResponse,
  EventAnnouncement,
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
        body.message ?? 'Gateway request failed',
        response.status,
        body.code
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
    getSeatSnapshot(eventID: string, sectionID: string) {
      return request<{ seats: GatewaySeat[]; snapshot_age_ms?: number }>(
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

export function formatMoney(cents: number) {
  return new Intl.NumberFormat('en-US', {
    style: 'currency',
    currency: 'USD'
  }).format(cents / 100);
}

type GatewayEvent = {
  id: string;
  name: string;
  venue: string;
  city: string;
  starts_at: string;
  seats_open: number;
  demand_score: number;
  status?: string;
};

type GatewaySeat = {
  seat_id: string;
  section_id: string;
  row: string;
  number: number;
  price_cents: number;
  status: 'AVAILABLE' | 'HELD' | 'SOLD';
  version: number;
  expires_at_server_ms?: number;
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
  const events = body.events.map((event): EventSummary => {
    const bucket =
      event.seats_open < 20 ? 'LOW' : event.seats_open < 80 ? 'MEDIUM' : 'HIGH';
    let image_url = '/event-midnight-array.svg';
    if (event.id === 'evt_neon_riot' || event.id === 'evt_summer_fests') {
      image_url = '/event-final-whistle.svg';
    } else if (event.id === 'evt_north_pier' || event.id === 'evt_civic_bowl') {
      image_url = '/event-zero-hour.svg';
    }

    return {
      id: event.id,
      title: event.name,
      venue: event.venue,
      city: event.city,
      category: 'Live',
      image_url,
      sale_starts_at: event.starts_at,
      remaining_bucket: bucket,
      demand_score: event.demand_score,
      min_price_cents: 8650,
      projection_lag_ms: body.projection_lag_ms ?? 0,
      status: event.status
    };
  });
  return {
    events,
    featured: events,
    meta: {
      projection_lag_ms: body.projection_lag_ms ?? 0,
      cache_status: 'gateway'
    }
  };
}

function mapSeatSnapshot(
  eventID: string,
  sectionID: string,
  body: { seats: GatewaySeat[]; snapshot_age_ms?: number }
): SeatSnapshot {
  const seats = body.seats.map((seat, index): Seat => {
    const col = index % 10;
    const row = Math.floor(index / 10);
    return {
      index,
      seat_id: seat.seat_id,
      section_id: seat.section_id,
      row: seat.row,
      x: 44 + col * 42,
      y: 42 + row * 42,
      price_cents: seat.price_cents,
      accessibility: col === 0 || col === 9,
      status: seat.status,
      version: seat.version,
      expires_at_server_ms: seat.expires_at_server_ms
    };
  });
  return {
    event_id: eventID,
    section_id: sectionID,
    server_time_ms: Date.now(),
    snapshot_age_ms: body.snapshot_age_ms ?? 0,
    projection_lag_ms: 0,
    seats
  };
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
