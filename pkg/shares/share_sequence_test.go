package shares

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	testns "github.com/celestiaorg/celestia-app/testutil/namespace"
	"github.com/celestiaorg/celestia-app/testutil/testfactory"
	"github.com/celestiaorg/nmt/namespace"
	"github.com/stretchr/testify/assert"
)

func TestShareSequenceRawData(t *testing.T) {
	type testCase struct {
		name          string
		shareSequence ShareSequence
		want          []byte
		wantErr       bool
	}
	blobNamespace := testns.RandomBlobNamespace()

	testCases := []testCase{
		{
			name: "empty share sequence",
			shareSequence: ShareSequence{
				NamespaceID: appconsts.TxNamespaceID,
				Shares:      []Share{},
			},
			want:    []byte{},
			wantErr: false,
		},
		{
			name: "one empty share",
			shareSequence: ShareSequence{
				NamespaceID: appconsts.TxNamespaceID,
				Shares: []Share{
					shareWithData(blobNamespace, true, 0, []byte{}),
				},
			},
			want:    []byte{},
			wantErr: false,
		},
		{
			name: "one share with one byte",
			shareSequence: ShareSequence{
				NamespaceID: appconsts.TxNamespaceID,
				Shares: []Share{
					shareWithData(blobNamespace, true, 1, []byte{0x0f}),
				},
			},
			want:    []byte{0xf},
			wantErr: false,
		},
		{
			name: "removes padding from last share",
			shareSequence: ShareSequence{
				NamespaceID: appconsts.TxNamespaceID,
				Shares: []Share{
					shareWithData(blobNamespace, true, appconsts.FirstSparseShareContentSize+1, bytes.Repeat([]byte{0xf}, appconsts.FirstSparseShareContentSize)),
					shareWithData(blobNamespace, false, 0, []byte{0x0f}),
				},
			},
			want:    bytes.Repeat([]byte{0xf}, appconsts.FirstSparseShareContentSize+1),
			wantErr: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.shareSequence.RawData()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.Equal(t, tc.want, got)
		})
	}
}

func Test_compactSharesNeeded(t *testing.T) {
	type testCase struct {
		sequenceLen int
		want        int
	}
	testCases := []testCase{
		{0, 0},
		{1, 1},
		{2, 1},
		{appconsts.FirstCompactShareContentSize, 1},
		{appconsts.FirstCompactShareContentSize + 1, 2},
		{appconsts.FirstCompactShareContentSize + appconsts.ContinuationCompactShareContentSize, 2},
		{appconsts.FirstCompactShareContentSize + appconsts.ContinuationCompactShareContentSize*100, 101},
	}
	for _, tc := range testCases {
		got := CompactSharesNeeded(tc.sequenceLen)
		assert.Equal(t, tc.want, got)
	}
}

func Test_sparseSharesNeeded(t *testing.T) {
	type testCase struct {
		sequenceLen uint32
		want        int
	}
	testCases := []testCase{
		{0, 0},
		{1, 1},
		{2, 1},
		{appconsts.FirstSparseShareContentSize, 1},
		{appconsts.FirstSparseShareContentSize + 1, 2},
		{appconsts.FirstSparseShareContentSize + appconsts.ContinuationSparseShareContentSize, 2},
		{appconsts.FirstSparseShareContentSize + appconsts.ContinuationCompactShareContentSize*2, 3},
		{appconsts.FirstSparseShareContentSize + appconsts.ContinuationCompactShareContentSize*99, 100},
		{1000, 2},
		{10000, 20},
		{100000, 199},
	}
	for _, tc := range testCases {
		got := SparseSharesNeeded(tc.sequenceLen)
		assert.Equal(t, tc.want, got)
	}
}

func shareWithData(namespace namespace.ID, isSequenceStart bool, sequenceLen uint32, data []byte) (rawShare []byte) {
	infoByte, _ := NewInfoByte(appconsts.ShareVersionZero, isSequenceStart)
	rawShare = append(rawShare, namespace...)
	rawShare = append(rawShare, byte(infoByte))
	if isSequenceStart {
		sequenceLenBuf := make([]byte, appconsts.SequenceLenBytes)
		binary.BigEndian.PutUint32(sequenceLenBuf, sequenceLen)
		rawShare = append(rawShare, sequenceLenBuf...)
	}
	rawShare = append(rawShare, data...)

	return padShare(rawShare)
}

func Test_validSequenceLen(t *testing.T) {
	type testCase struct {
		name          string
		shareSequence ShareSequence
		wantErr       bool
	}

	tailPadding := ShareSequence{
		NamespaceID: appconsts.TailPaddingNamespaceID,
		Shares:      []Share{TailPaddingShare()},
	}

	ns1 := bytes.Repeat([]byte{1}, appconsts.NamespaceSize)
	share := NamespacePaddingShare(ns1)
	namespacePadding := ShareSequence{
		NamespaceID: ns1,
		Shares:      []Share{share},
	}

	reservedPadding := ShareSequence{
		NamespaceID: appconsts.ReservedPaddingNamespaceID,
		Shares:      []Share{ReservedPaddingShare()},
	}

	notSequenceStart := ShareSequence{
		NamespaceID: ns1,
		Shares: []Share{
			shareWithData(ns1, false, 0, []byte{0x0f}),
		},
	}

	testCases := []testCase{
		{
			name:          "empty share sequence",
			shareSequence: ShareSequence{},
			wantErr:       true,
		},
		{
			name:          "valid share sequence",
			shareSequence: generateValidShareSequence(t),
			wantErr:       false,
		},
		{
			name:          "tail padding",
			shareSequence: tailPadding,
			wantErr:       false,
		},
		{
			name:          "namespace padding",
			shareSequence: namespacePadding,
			wantErr:       false,
		},
		{
			name:          "reserved padding",
			shareSequence: reservedPadding,
			wantErr:       false,
		},
		{
			name:          "sequence length where first share is not sequence start",
			shareSequence: notSequenceStart,
			wantErr:       true, // error: "share sequence has 1 shares but needed 0 shares"
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.shareSequence.validSequenceLen()
			if tc.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func generateValidShareSequence(t *testing.T) ShareSequence {
	css := NewCompactShareSplitter(appconsts.TxNamespaceID, appconsts.ShareVersionZero)
	txs := testfactory.GenerateRandomTxs(5, 200)
	for _, tx := range txs {
		css.WriteTx(tx)
	}
	shares, _ := css.Export(0)

	return ShareSequence{
		NamespaceID: appconsts.TxNamespaceID,
		Shares:      shares,
	}
}
