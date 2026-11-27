// Copyright 2026 BlueboxC
// Licensed under the Apache License, Version 2.0.
package message

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dolthub/dolt/go/store/hash"
	"github.com/dolthub/dolt/go/store/val"
)

// This fixture calls the original serializer and address walker. It does not
// establish remote admission or the GC worker boundary by itself.
func TestArtifactMetadataValidation(t *testing.T) {
	kd := val.NewTupleDescriptor(val.Type{Enc: val.Int32Enc}, val.Type{Enc: val.CommitAddrEnc}, val.Type{Enc: val.Uint8Enc}, val.Type{Enc: val.ByteStringEnc})
	vd := val.NewTupleDescriptor(val.Type{Enc: val.JSONEnc})
	theirs, base := testAddr(1), testAddr(2)
	// ConflictMetadata stores hash.Hash as a JSON byte array, not its base32 text.
	legacy, err := json.Marshal(struct {
		Base hash.Hash `json:"bc"`
	}{Base: base})
	if err != nil {
		t.Fatal(err)
	}
	jsonTuple := func(body []byte) []byte { return val.NewTuple(sharedPool, append(append([]byte{}, body...), 0)) }
	key := func(kind byte) []byte {
		return val.NewTuple(sharedPool, []byte{1, 0, 0, 0}, theirs[:], []byte{kind}, make([]byte, hash.ByteLen))
	}
	for _, tc := range []struct {
		name      string
		kind      byte
		value     []byte
		wantBase  bool
		wantError bool
	}{
		{"legacy conflict base remains reachable", 1, jsonTuple(legacy), true, false},
		{"unrelated metadata remains valid", 1, jsonTuple([]byte(`{"other":true}`)), false, false},
		{"empty tuple remains absent", 1, val.NewTuple(sharedPool), false, false},
		{"nonconflict value stays unparsed", 2, []byte{1}, false, false},
		{"malformed JSON returns error", 1, jsonTuple([]byte(`{"bc":`)), false, true},
		{"short conflict value returns error without panic", 1, []byte{1}, false, true},
		{"first field offset beyond data returns error without panic", 1, []byte{'x', 0, 255, 255, 2, 0}, false, true},
		{"invalid field count returns error without panic", 1, []byte{0, 0, 255, 255}, false, true},
		{"malformed base hash retains error", 1, jsonTuple([]byte(`{"bc":"x"}`)), false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := NewMergeArtifactSerializer(kd, vd, sharedPool)
			msg := s.Serialize([][]byte{key(tc.kind)}, [][]byte{tc.value}, nil, 0)
			found := map[hash.Hash]bool{}
			var err error
			func() {
				defer func() {
					if p := recover(); p != nil {
						t.Errorf("original address walker panicked: %v", p)
					}
				}()
				err = WalkAddresses(context.Background(), msg, func(_ context.Context, h hash.Hash) error { found[h] = true; return nil })
			}()
			if (err != nil) != tc.wantError {
				t.Errorf("error=%v; wantError=%v", err, tc.wantError)
			}
			if !tc.wantError {
				if !found[theirs] {
					t.Error("source commit address disappeared")
				}
				if found[base] != tc.wantBase {
					t.Errorf("base reachable=%v; want=%v", found[base], tc.wantBase)
				}
			}
		})
	}
}
