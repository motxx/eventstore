package lmdb

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/PowerDNS/lmdb-go/lmdb"
	"github.com/motxx/eventstore"
	"github.com/nbd-wtf/go-nostr"
	nostr_binary "github.com/nbd-wtf/go-nostr/binary"
)

func (b *LMDBBackend) SaveEvent(ctx context.Context, evt *nostr.Event) error {
	// sanity checking
	if evt.CreatedAt > maxuint32 || evt.Kind > maxuint16 {
		return fmt.Errorf("event with values out of expected boundaries")
	}

	return b.lmdbEnv.Update(func(txn *lmdb.Txn) error {
		// check if we already have this id
		id, _ := hex.DecodeString(evt.ID)
		_, err := txn.Get(b.indexId, id)
		if operr, ok := err.(*lmdb.OpError); ok && operr.Errno != lmdb.NotFound {
			// we will only proceed if we get a NotFound
			return eventstore.ErrDupEvent
		}

		// encode to binary form so we'll save it
		bin, err := nostr_binary.Marshal(evt)
		if err != nil {
			return err
		}

		idx := b.Serial()
		// raw event store
		if err := txn.Put(b.rawEventStore, idx, bin, 0); err != nil {
			return err
		}

		for _, k := range b.getIndexKeysForEvent(evt) {
			if err := txn.Put(k.dbi, k.key, idx, 0); err != nil {
				return err
			}
		}

		return nil
	})
}
