package badger

import (
	"encoding/binary"
	"encoding/hex"

	"github.com/dgraph-io/badger/v4"
	"github.com/fiatjaf/eventstore"
	"github.com/nbd-wtf/go-nostr"
)

const (
	dbVersionKey          byte = 255
	rawEventStorePrefix   byte = 0
	indexCreatedAtPrefix  byte = 1
	indexIdPrefix         byte = 2
	indexKindPrefix       byte = 3
	indexPubkeyPrefix     byte = 4
	indexPubkeyKindPrefix byte = 5
	indexTagPrefix        byte = 6
	indexTag32Prefix      byte = 7
)

var _ eventstore.Store = (*BadgerBackend)(nil)

type BadgerBackend struct {
	Path     string
	MaxLimit int

	*badger.DB
	seq *badger.Sequence
}

func (b *BadgerBackend) Init() error {
	db, err := badger.Open(badger.DefaultOptions(b.Path))
	if err != nil {
		return err
	}
	b.DB = db
	b.seq, err = db.GetSequence([]byte("events"), 1000)
	if err != nil {
		return err
	}

	if b.MaxLimit == 0 {
		b.MaxLimit = 500
	}

	return nil
}

func (b BadgerBackend) Close() {
	b.DB.Close()
	b.seq.Release()
}

func (b BadgerBackend) Serial() []byte {
	v, _ := b.seq.Next()
	vb := make([]byte, 5)
	vb[0] = rawEventStorePrefix
	binary.BigEndian.PutUint32(vb[1:], uint32(v))
	return vb
}

func getIndexKeysForEvent(evt *nostr.Event, idx []byte) [][]byte {
	keys := make([][]byte, 0, 18)

	// indexes
	{
		// ~ by id
		id, _ := hex.DecodeString(evt.ID)
		k := make([]byte, 1+32+4)
		k[0] = indexIdPrefix
		copy(k[1:], id)
		copy(k[1+32:], idx)
		keys = append(keys, k)
	}

	{
		// ~ by pubkey+date
		pubkey, _ := hex.DecodeString(evt.PubKey)
		k := make([]byte, 1+32+4+4)
		k[0] = indexPubkeyPrefix
		copy(k[1:], pubkey)
		binary.BigEndian.PutUint32(k[1+32:], uint32(evt.CreatedAt))
		copy(k[1+32+4:], idx)
		keys = append(keys, k)
	}

	{
		// ~ by kind+date
		k := make([]byte, 1+2+4+4)
		k[0] = indexKindPrefix
		binary.BigEndian.PutUint16(k[1:], uint16(evt.Kind))
		binary.BigEndian.PutUint32(k[1+2:], uint32(evt.CreatedAt))
		copy(k[1+2+4:], idx)
		keys = append(keys, k)
	}

	{
		// ~ by pubkey+kind+date
		pubkey, _ := hex.DecodeString(evt.PubKey)
		k := make([]byte, 1+32+2+4+4)
		k[0] = indexPubkeyKindPrefix
		copy(k[1:], pubkey)
		binary.BigEndian.PutUint16(k[1+32:], uint16(evt.Kind))
		binary.BigEndian.PutUint32(k[1+32+2:], uint32(evt.CreatedAt))
		copy(k[1+32+2+4:], idx)
		keys = append(keys, k)
	}

	// ~ by tagvalue+date
	for _, tag := range evt.Tags {
		if len(tag) < 2 || len(tag[0]) != 1 || len(tag[1]) == 0 || len(tag[1]) > 100 {
			continue
		}

		var v []byte
		var indexPrefix byte
		if vb, _ := hex.DecodeString(tag[1]); len(vb) == 32 {
			// store value as bytes
			v = vb
			indexPrefix = indexTag32Prefix
		} else {
			v = []byte(tag[1])
			indexPrefix = indexTagPrefix
		}

		k := make([]byte, 1+len(v)+4+4)
		k[0] = indexPrefix
		copy(k[1:], v)
		binary.BigEndian.PutUint32(k[1+len(v):], uint32(evt.CreatedAt))
		copy(k[1+len(v)+4:], idx)
		keys = append(keys, k)
	}

	{
		// ~ by date only
		k := make([]byte, 1+4+4)
		k[0] = indexCreatedAtPrefix
		binary.BigEndian.PutUint32(k[1:], uint32(evt.CreatedAt))
		copy(k[1+4:], idx)
		keys = append(keys, k)
	}

	return keys
}
