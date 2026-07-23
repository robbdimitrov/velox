package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"velox/apps/viewservice/internal"

	"github.com/twmb/franz-go/pkg/kgo"
)

// fakeKafkaClient replaces a live broker for the consumer loop tests. The
// first PollFetches call returns the configured batch; later calls block
// until the context is canceled, mirroring a client with nothing left to
// deliver before shutdown.
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
