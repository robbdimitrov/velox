package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type Server struct {
	mu          sync.Mutex
	secret      []byte
	now         func() time.Time
	holdTTL     time.Duration
	users       map[string]User
	events      map[string]Event
	seats       map[string]map[string]map[string]*Seat
	orders      map[string]*Order
	idempotency map[string]idempotencyRecord
	loginFails  map[string]loginFailure
	store       *DatabaseStore
	seatClients   map[string]map[chan string]struct{}
	vendorClients map[string]map[chan string]struct{}
	httpClient    *http.Client
	orderSvcURL string
}

func NewServerWithStore(secret string, store *DatabaseStore) *Server {
	s := &Server{
		secret:      []byte(secret),
		now:         time.Now,
		holdTTL:     5 * time.Minute,
		users:       map[string]User{},
		events:      map[string]Event{},
		seats:       map[string]map[string]map[string]*Seat{},
		orders:      map[string]*Order{},
		idempotency: map[string]idempotencyRecord{},
		loginFails:  map[string]loginFailure{},
		store:       store,
		seatClients:   map[string]map[chan string]struct{}{},
		vendorClients: map[string]map[chan string]struct{}{},
		httpClient:    http.DefaultClient,
	}
	if store != nil {
		go s.listenSeatUpdates()
		go s.listenVendorUpdates()
	}
	s.seed()
	return s
}

func (s *Server) SetOrderServiceURL(url string) {
	s.orderSvcURL = url
}

func (s *Server) SetHTTPClient(client *http.Client) {
	s.httpClient = client
}

func (s *Server) listenSeatUpdates() {
	for {
		err := s.store.ListenSeatUpdates(context.Background(), func(payload string) {
			var update struct {
				EventID string `json:"event_id"`
			}
			if err := json.Unmarshal([]byte(payload), &update); err != nil {
				return
			}
			s.mu.Lock()
			clients := s.seatClients[update.EventID]
			var active []chan string
			for c := range clients {
				active = append(active, c)
			}
			s.mu.Unlock()
			for _, c := range active {
				select {
				case c <- payload:
				default:
				}
			}
		})
		if err != nil {
			time.Sleep(time.Second)
		}
	}
}

func (s *Server) seed() {
	s.users["reserver@velox.local"] = User{ID: "usr_reserver_1", Email: "reserver@velox.local", Password: "reserver", Role: RoleReserver}
	s.users["vendor@velox.local"] = User{ID: "usr_vendor_1", Email: "vendor@velox.local", Password: "vendor", Role: RoleVendor, VendorID: "ven_northstar"}
	eventsToSeed := []Event{
		{
			ID:          "evt_neon_riot",
			VendorID:    "ven_northstar",
			Name:        "Neon Riot Live",
			Venue:       "Velox Arena",
			City:        "Chicago",
			StartsAt:    time.Date(2026, 8, 15, 20, 0, 0, 0, time.UTC),
			SectionIDs:  []string{"A", "B"},
			DemandScore: 94,
		},
		{
			ID:          "evt_north_pier",
			VendorID:    "ven_northstar",
			Name:        "North Pier Symphony",
			Venue:       "North Pier Hall",
			City:        "Seattle",
			StartsAt:    time.Date(2026, 9, 10, 19, 30, 0, 0, time.UTC),
			SectionIDs:  []string{"A", "B"},
			DemandScore: 78,
		},
		{
			ID:          "evt_civic_bowl",
			VendorID:    "ven_northstar",
			Name:        "Civic Bowl Championship",
			Venue:       "Civic Bowl",
			City:        "Denver",
			StartsAt:    time.Date(2026, 10, 5, 18, 0, 0, 0, time.UTC),
			SectionIDs:  []string{"A", "B"},
			DemandScore: 88,
		},
		{
			ID:          "evt_summer_fests",
			VendorID:    "ven_northstar",
			Name:        "Summer Solstice Festival",
			Venue:       "Moonlight Grounds",
			City:        "Austin",
			StartsAt:    time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC),
			SectionIDs:  []string{"A", "B"},
			DemandScore: 99,
		},
	}

	for _, event := range eventsToSeed {
		s.events[event.ID] = event
		s.seats[event.ID] = map[string]map[string]*Seat{}
		for _, sectionID := range event.SectionIDs {
			s.seats[event.ID][sectionID] = map[string]*Seat{}
			for row := 'A'; row <= 'D'; row++ {
				for n := 1; n <= 10; n++ {
					id := string(row) + "-" + func() string {
						if n < 10 {
							return "0" + string(rune('0'+n))
						}
						return "10"
					}()
					s.seats[event.ID][sectionID][id] = &Seat{
						EventID: event.ID, SectionID: sectionID, ID: id,
						Row: string(row), Number: n, PriceCents: 8500 + n*150,
						Status: StatusAvailable, Version: 1,
					}
					event.SeatsTotal++
					event.SeatsOpen++
				}
			}
		}
		s.events[event.ID] = event
	}
}

func (s *Server) listenVendorUpdates() {
	for {
		err := s.store.ListenVendorUpdates(context.Background(), func(payload string) {
			var msg struct {
				EventID string `json:"event_id"`
			}
			if err := json.Unmarshal([]byte(payload), &msg); err == nil && msg.EventID != "" {
				s.mu.Lock()
				for ch := range s.vendorClients[msg.EventID] {
					select {
					case ch <- payload:
					default:
					}
				}
				s.mu.Unlock()
			}
		})
		if err != nil {
			time.Sleep(1 * time.Second)
		}
	}
}
