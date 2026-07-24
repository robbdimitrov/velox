package main

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"velox/apps/viewservice/internal"

	"github.com/twmb/franz-go/pkg/kgo"
)

// fakeKafkaClient replaces a live broker; PollFetches returns its batch once,
// then blocks until context cancellation like an exhausted client.
type fakeKafkaClient struct {
	mu        sync.Mutex
	fetches   kgo.Fetches
	polled    bool
	callOrder []string
	committed []*kgo.Record
	produced  []*kgo.Record
}

func (f *fakeKafkaClient) PollFetches(ctx context.Context) kgo.Fetches {
	f.mu.Lock()
	if !f.polled {
		f.polled = true
		fs := f.fetches
		f.mu.Unlock()
		return fs
	}
	f.mu.Unlock()
	<-ctx.Done()
	return kgo.Fetches{}
}

func (f *fakeKafkaClient) CommitRecords(ctx context.Context, rs ...*kgo.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callOrder = append(f.callOrder, "commit")
	f.committed = append(f.committed, rs...)
	return nil
}

func (f *fakeKafkaClient) ProduceSync(ctx context.Context, rs ...*kgo.Record) kgo.ProduceResults {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.callOrder = append(f.callOrder, "produce")
	f.produced = append(f.produced, rs...)
	results := make(kgo.ProduceResults, len(rs))
	for i, r := range rs {
		results[i] = kgo.ProduceResult{Record: r}
	}
	return results
}

type noopEventStore struct{}

func (noopEventStore) ApplyEvent(ctx context.Context, event internal.Event, sourceTopic string, sourcePartition int32, sourceOffset int64) error {
	return nil
}

type invalidSignatureEventStore struct{}

func (invalidSignatureEventStore) ApplyEvent(ctx context.Context, event internal.Event, sourceTopic string, sourcePartition int32, sourceOffset int64) error {
	return internal.ErrInvalidSignature
}

func TestMalformedRecordPublishedToDLQBeforeCommit(t *testing.T) {
	malformed := &kgo.Record{
		Topic:     "inventory.events.v1",
		Partition: 0,
		Offset:    42,
		Value:     []byte("not json"),
	}
	fake := &fakeKafkaClient{
		fetches: kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: malformed.Topic, Partitions: []kgo.FetchPartition{{Partition: malformed.Partition, Records: []*kgo.Record{malformed}}}}}}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go consumeEvents(ctx, fake, noopEventStore{}, &consumerHealth{}, &wg)

	deadline := time.Now().Add(2 * time.Second)
	for {
		fake.mu.Lock()
		done := len(fake.callOrder) >= 2
		fake.mu.Unlock()
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for DLQ publish and commit")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	wg.Wait()

	if len(fake.callOrder) < 2 || fake.callOrder[0] != "produce" || fake.callOrder[1] != "commit" {
		t.Fatalf("expected produce before commit, got order %v", fake.callOrder)
	}
	if len(fake.produced) != 1 {
		t.Fatalf("expected exactly one DLQ record, got %d", len(fake.produced))
	}
	if fake.produced[0].Topic != dlqTopic {
		t.Fatalf("expected DLQ record on topic %q, got %q", dlqTopic, fake.produced[0].Topic)
	}
	if len(fake.committed) != 1 || fake.committed[0] != malformed {
		t.Fatalf("expected the malformed source record to be committed, got %v", fake.committed)
	}
}

// A bad signature never becomes valid on retry, so it must route to DLQ and
// commit, not retry forever.
func TestInvalidSignatureRecordPublishedToDLQBeforeCommit(t *testing.T) {
	unsigned := &kgo.Record{
		Topic:     "inventory.events.v1",
		Partition: 0,
		Offset:    7,
		Value:     []byte(`{"Type":"SeatReservationHeld"}`),
	}
	fake := &fakeKafkaClient{
		fetches: kgo.Fetches{{Topics: []kgo.FetchTopic{{Topic: unsigned.Topic, Partitions: []kgo.FetchPartition{{Partition: unsigned.Partition, Records: []*kgo.Record{unsigned}}}}}}},
	}

	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	go consumeEvents(ctx, fake, invalidSignatureEventStore{}, &consumerHealth{}, &wg)

	deadline := time.Now().Add(2 * time.Second)
	for {
		fake.mu.Lock()
		done := len(fake.callOrder) >= 2
		fake.mu.Unlock()
		if done {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for DLQ publish and commit")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	wg.Wait()

	if len(fake.callOrder) < 2 || fake.callOrder[0] != "produce" || fake.callOrder[1] != "commit" {
		t.Fatalf("expected produce before commit, got order %v", fake.callOrder)
	}
	if len(fake.committed) != 1 || fake.committed[0] != unsigned {
		t.Fatalf("expected the rejected source record to be committed, got %v", fake.committed)
	}
}

func TestDLQCountReflectedInMetrics(t *testing.T) {
	health := &consumerHealth{}
	health.markDLQ()
	health.markDLQ()

	body := health.metrics("viewservice")
	if !strings.Contains(body, `velox_consumer_dlq_events_total{service="viewservice",consumer="events"} 2`) {
		t.Fatalf("metrics output missing dlq counter: %s", body)
	}
}
