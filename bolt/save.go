package bolt

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"

	"github.com/motxx/eventstore"
	"github.com/nbd-wtf/go-nostr"
	nostr_binary "github.com/nbd-wtf/go-nostr/binary"
	bolt "go.etcd.io/bbolt"
)

func (b *BoltBackend) SaveEvent(ctx context.Context, evt *nostr.Event) error {
	// sanity checking
	if evt.CreatedAt > maxuint32 || evt.Kind > maxuint16 {
		return fmt.Errorf("event with values out of expected boundaries")
	}

	return b.db.Update(func(txn *bolt.Tx) error {
		id, _ := hex.DecodeString(evt.ID)

		// check if we already have this id
		bucket := txn.Bucket(bucketId)
		res := bucket.Get(id)
		if res != nil {
			return eventstore.ErrDupEvent
		}

		// encode to binary form so we'll save it
		bin, err := nostr_binary.Marshal(evt)
		if err != nil {
			return err
		}

		// raw event store
		raw := txn.Bucket(bucketRaw)
		seq, _ := raw.NextSequence()
		seqb := binary.BigEndian.AppendUint64(nil, seq)
		if err := raw.Put(seqb, bin); err != nil {
			return err
		}

		for _, km := range getIndexKeysForEvent(evt) {
			bucket := txn.Bucket(km.bucket)
			if err := bucket.Put(km.key, seqb); err != nil {
				return err
			}
		}

		return nil
	})
}
