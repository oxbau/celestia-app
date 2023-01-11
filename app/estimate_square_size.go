package app

import (
	"math"

	"github.com/celestiaorg/celestia-app/pkg/appconsts"
	"github.com/celestiaorg/celestia-app/pkg/shares"
	coretypes "github.com/tendermint/tendermint/types"
)

// estimateSquareSize uses the provided block data to over estimate the square
// size and the starting share index of non-reserved namespaces. The estimates
// returned are liberal in the sense that we assume close to worst case and
// round up.
//
// NOTE: The estimation process does not have to be perfect. We can overestimate
// because the cost of padding is limited.
func estimateSquareSize(txs []parsedTx) (squareSize uint64, nonreserveStart int) {
	txSharesUsed := estimateCompactShares(appconsts.DefaultMaxSquareSize, txs)
	blobSharesUsed := 0

	for _, ptx := range txs {
		if len(ptx.normalTx) != 0 {
			continue
		}
		blobSharesUsed += shares.SparseSharesNeeded(uint32(ptx.blobTx.DataUsed()))
	}

	totalSharesUsed := float64(txSharesUsed + blobSharesUsed)
	if totalSharesUsed <= 1 {
		return appconsts.DefaultMinSquareSize, txSharesUsed
	}
	// increase the total shares used by the worst case padding ratio
	totalSharesUsed *= 1.35
	minSize := uint64(math.Ceil(math.Sqrt(totalSharesUsed)))
	squareSize = shares.RoundUpPowerOfTwo(minSize)
	if squareSize >= appconsts.DefaultMaxSquareSize {
		squareSize = appconsts.DefaultMaxSquareSize
	}
	if squareSize <= appconsts.DefaultMinSquareSize {
		squareSize = appconsts.DefaultMinSquareSize
	}

	return squareSize, txSharesUsed
}

// estimateCompactShares estimates the number of shares used by compact shares
func estimateCompactShares(squareSize uint64, ptxs []parsedTx) int {
	maxWTxOverhead := maxWrappedTxOverhead(squareSize)
	txbytes := 0
	for _, pTx := range ptxs {
		if len(pTx.normalTx) != 0 {
			txLen := len(pTx.normalTx)
			txLen += shares.DelimLen(uint64(txLen))
			txbytes += txLen
			continue
		}
		txLen := len(pTx.blobTx.Tx) + maxWTxOverhead
		txLen += shares.DelimLen(uint64(txLen))
		txbytes += txLen
	}

	sharesUsed := 1
	if txbytes <= appconsts.FirstCompactShareContentSize {
		return sharesUsed
	}

	// account for the first share
	txbytes -= appconsts.FirstCompactShareContentSize
	sharesUsed += (txbytes / appconsts.ContinuationCompactShareContentSize) + 1 // add 1 to round up and another to account for the first share

	return sharesUsed
}

// maxWrappedTxOverhead calculates the maximum amount of overhead introduced by
// wrapping a transaction with a shares index
//
// TODO: make more efficient by only generating these numbers once or something
// similar. This function alone can take up to 5ms.
func maxWrappedTxOverhead(squareSize uint64) int {
	maxTxLen := squareSize * squareSize * appconsts.ContinuationCompactShareContentSize
	wtx, err := coretypes.MarshalIndexWrapper(
		uint32(squareSize*squareSize),
		make([]byte, maxTxLen))
	if err != nil {
		panic(err)
	}
	return len(wtx) - int(maxTxLen)
}
