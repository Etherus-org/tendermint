package state

import (
	"bytes"
	"errors"
	"fmt"
	"time"

	dbm "github.com/tendermint/tendermint/libs/db"
	"github.com/tendermint/tendermint/types"
)

const (
	//BlocksMinInterval - 1 second interval for the next block
	BlocksMinInterval time.Duration = 15e9
	//RightProposerBlock - The starting block of right proposer strategy
	RightProposerBlock int64 = 1000
	//BlocksMaxFuture - Maximum future block time
	BlocksMaxFuture time.Duration = 29e9 //To match ethereum future block constant
)

//-----------------------------------------------------
// Validate block

func validateBlock(stateDB dbm.DB, state State, block *types.Block) error {
	// validate internal consistency
	if err := block.ValidateBasic(); err != nil {
		return err
	}

	// validate basic info
	if block.ChainID != state.ChainID {
		return fmt.Errorf("Wrong Block.Header.ChainID. Expected %v, got %v", state.ChainID, block.ChainID)
	}
	if block.Height != state.LastBlockHeight+1 {
		return fmt.Errorf("Wrong Block.Header.Height. Expected %v, got %v", state.LastBlockHeight+1, block.Height)
	}
	/*	TODO: Determine bounds for Time
		See blockchain/reactor "stopSyncingDurationMinutes"
	*/
	if !block.Time.After(state.LastBlockTime.Add(BlocksMinInterval - 1)) {
		return errors.New("Invalid Block.Header.Time")
	}

	//Do not allow too distant future blocks
	if block.Time.After(time.Now().Add(BlocksMaxFuture)) {
		return fmt.Errorf("Too future block time. Now %v, got %v", time.Now(), block.Time)
	}

	// validate prev block info
	if !block.LastBlockID.Equals(state.LastBlockID) {
		return fmt.Errorf("Wrong Block.Header.LastBlockID.  Expected %v, got %v", state.LastBlockID, block.LastBlockID)
	}
	newTxs := int64(len(block.Data.Txs))
	if block.TotalTxs != state.LastBlockTotalTx+newTxs {
		return fmt.Errorf("Wrong Block.Header.TotalTxs. Expected %v, got %v", state.LastBlockTotalTx+newTxs, block.TotalTxs)
	}

	// validate app info
	if !bytes.Equal(block.AppHash, state.AppHash) {
		return fmt.Errorf("Wrong Block.Header.AppHash.  Expected %X, got %v", state.AppHash, block.AppHash)
	}
	if !bytes.Equal(block.ConsensusHash, state.ConsensusParams.Hash()) {
		return fmt.Errorf("Wrong Block.Header.ConsensusHash.  Expected %X, got %v", state.ConsensusParams.Hash(), block.ConsensusHash)
	}
	if !bytes.Equal(block.LastResultsHash, state.LastResultsHash) {
		return fmt.Errorf("Wrong Block.Header.LastResultsHash.  Expected %X, got %v", state.LastResultsHash, block.LastResultsHash)
	}
	if !bytes.Equal(block.ValidatorsHash, state.Validators.Hash()) {
		return fmt.Errorf("Wrong Block.Header.ValidatorsHash.  Expected %X, got %v", state.Validators.Hash(), block.ValidatorsHash)
	}

	// Validate block LastCommit.
	if block.Height == 1 {
		if len(block.LastCommit.Precommits) != 0 {
			return errors.New("Block at height 1 (first block) should have no LastCommit precommits")
		}
	} else {
		if len(block.LastCommit.Precommits) != state.LastValidators.Size() {
			return fmt.Errorf("Invalid block commit size. Expected %v, got %v",
				state.LastValidators.Size(), len(block.LastCommit.Precommits))
		}
		err := state.LastValidators.VerifyCommit(
			state.ChainID, state.LastBlockID, block.Height-1, block.LastCommit)
		if err != nil {
			return err
		}
	}

	// TODO: Each check requires loading an old validator set.
	// We should cap the amount of evidence per block
	// to prevent potential proposer DoS.
	for _, ev := range block.Evidence.Evidence {
		if err := VerifyEvidence(stateDB, state, ev); err != nil {
			return types.NewEvidenceInvalidErr(ev, err)
		}
	}

	return nil
}

// VerifyEvidence verifies the evidence fully by checking:
// - it is sufficiently recent (MaxAge)
// - it is from a key who was a validator at the given height
// - it is internally consistent
// - it was properly signed by the alleged equivocator
func VerifyEvidence(stateDB dbm.DB, state State, evidence types.Evidence) error {
	height := state.LastBlockHeight

	evidenceAge := height - evidence.Height()
	maxAge := state.ConsensusParams.EvidenceParams.MaxAge
	if evidenceAge > maxAge {
		return fmt.Errorf("Evidence from height %d is too old. Min height is %d",
			evidence.Height(), height-maxAge)
	}

	valset, err := LoadValidators(stateDB, evidence.Height())
	if err != nil {
		// TODO: if err is just that we cant find it cuz we pruned, ignore.
		// TODO: if its actually bad evidence, punish peer
		return err
	}

	// The address must have been an active validator at the height.
	// NOTE: we will ignore evidence from H if the key was not a validator
	// at H, even if it is a validator at some nearby H'
	ev := evidence
	height, addr := ev.Height(), ev.Address()
	_, val := valset.GetByAddress(addr)
	if val == nil {
		return fmt.Errorf("Address %X was not a validator at height %d", addr, height)
	}

	if err := evidence.Verify(state.ChainID, val.PubKey); err != nil {
		return err
	}

	return nil
}
