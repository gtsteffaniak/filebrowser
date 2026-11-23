package opensw

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"hash/crc32"
	"testing"
)

type blockEntry struct {
	name string
	raw  []byte
}

func blockStream(t *testing.T, entries ...blockEntry) []byte {
	t.Helper()
	var out []byte
	for _, entry := range entries {
		var compressed bytes.Buffer
		zw, err := flate.NewWriter(&compressed, flate.DefaultCompression)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := zw.Write(entry.raw); err != nil {
			t.Fatal(err)
		}
		if err := zw.Close(); err != nil {
			t.Fatal(err)
		}
		name := nibbleSwap([]byte(entry.name), 4)
		header := make([]byte, blockHeaderSize)
		copy(header, blockMarker)
		header[6], header[7] = 0, 4 // header key = 4
		binary.LittleEndian.PutUint32(header[10:14], crc32.ChecksumIEEE(entry.raw))
		binary.LittleEndian.PutUint32(header[14:18], uint32(compressed.Len()))
		binary.LittleEndian.PutUint32(header[18:22], uint32(len(entry.raw)))
		binary.LittleEndian.PutUint32(header[22:26], uint32(len(name)))
		out = append(out, header...)
		out = append(out, name...)
		out = append(out, compressed.Bytes()...)
	}
	return out
}

func TestCanRetainAtDocumentBudgetBoundary(t *testing.T) {
	if !canRetain(maxDecodedPerDoc, int(maxDecodedPerDoc)) {
		t.Fatal("a payload exactly filling the document budget must be accepted")
	}
	if canRetain(maxDecodedPerDoc-1, int(maxDecodedPerDoc)) {
		t.Fatal("a payload exceeding the document budget must be rejected")
	}
}

func TestReadBlockBlobsDuplicateDoesNotChargeBudget(t *testing.T) {
	data := blockStream(t,
		blockEntry{"Dup", []byte("aaaa")},
		blockEntry{"Dup", []byte("bbbb")},
		blockEntry{"Keep", []byte("cccc")},
	)

	// 8 bytes fit the two distinct payloads; a charged duplicate would fail.
	blobs, err := readBlockBlobsWithBudget(data, 8)
	if err != nil {
		t.Fatalf("unexpected budget error: %v", err)
	}
	if string(blobs["Dup"]) != "aaaa" {
		t.Fatalf("first valid stream kept, got %q", blobs["Dup"])
	}
	if string(blobs["Keep"]) != "cccc" {
		t.Fatalf("third stream should fit after discarding duplicate, got %q", blobs["Keep"])
	}
	if _, err := readBlockBlobsWithBudget(data, 7); err == nil {
		t.Fatal("expected an error when the retained total exceeds the budget")
	}
}
