//+build !celo

// Package ethereum contains implementation of ethereum chain interface.
package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ipfs/go-log"

	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-common/pkg/subscription"
	corechain "github.com/keep-network/keep-core/pkg/chain"
	"github.com/keep-network/keep-ecdsa/pkg/chain"
)

var logger = log.Logger("keep-chain-eth-ethereum")

// Address returns client's ethereum address.
func (ec *ethereumChain) Address() common.Address {
	return ec.accountKey.Address
}

// Signing returns signing interface for creating and verifying signatures.
func (ec *ethereumChain) Signing() corechain.Signing {
	return ethutil.NewSigner(ec.accountKey.PrivateKey)
}

// BlockCounter returns a block counter.
func (ec *ethereumChain) BlockCounter() corechain.BlockCounter {
	return ec.blockCounter
}

// RegisterAsMemberCandidate registers client as a candidate to be selected
// to a keep.
func (ec *ethereumChain) RegisterAsMemberCandidate(application common.Address) error {
	gasEstimate, err := ec.bondedECDSAKeepFactoryContract.RegisterMemberCandidateGasEstimate(application)
	if err != nil {
		return fmt.Errorf("failed to estimate gas [%v]", err)
	}

	// If we have multiple sortition pool join transactions queued - and that
	// happens when multiple operators become eligible to join at the same time,
	// e.g. after lowering the minimum bond requirement, transactions mined at
	// the end may no longer have valid gas limits as they were estimated based
	// on a different state of the pool. We add 20% safety margin to the original
	// gas estimation to account for that.
	gasEstimateWithMargin := float64(gasEstimate) * float64(1.2)
	transaction, err := ec.bondedECDSAKeepFactoryContract.RegisterMemberCandidate(
		application,
		ethutil.TransactionOptions{
			GasLimit: uint64(gasEstimateWithMargin),
		},
	)
	if err != nil {
		return err
	}

	logger.Debugf("submitted RegisterMemberCandidate transaction with hash: [%x]", transaction.Hash())

	return nil
}

// OnBondedECDSAKeepCreated installs a callback that is invoked when an on-chain
// notification of a new ECDSA keep creation is seen.
func (ec *ethereumChain) OnBondedECDSAKeepCreated(
	handler func(event *chain.BondedECDSAKeepCreatedEvent),
) subscription.EventSubscription {
	onEvent := func(
		KeepAddress common.Address,
		Members []common.Address,
		Owner common.Address,
		Application common.Address,
		HonestThreshold *big.Int,
		blockNumber uint64,
	) {
		handler(&chain.BondedECDSAKeepCreatedEvent{
			KeepAddress:     KeepAddress,
			Members:         Members,
			HonestThreshold: HonestThreshold.Uint64(),
			BlockNumber:     blockNumber,
		})
	}

	return ec.bondedECDSAKeepFactoryContract.BondedECDSAKeepCreated(
		nil,
		nil,
		nil,
		nil,
	).OnEvent(onEvent)
}

// HasMinimumStake returns true if the specified address is staked.  False will
// be returned if not staked.  If err != nil then it was not possible to determine
// if the address is staked or not.
func (ec *ethereumChain) HasMinimumStake(address common.Address) (bool, error) {
	return ec.bondedECDSAKeepFactoryContract.HasMinimumStake(address)
}

// BalanceOf returns the stake balance of the specified address.
func (ec *ethereumChain) BalanceOf(address common.Address) (*big.Int, error) {
	return ec.bondedECDSAKeepFactoryContract.BalanceOf(address)
}

// IsRegisteredForApplication checks if the operator is registered
// as a signer candidate in the factory for the given application.
func (ec *ethereumChain) IsRegisteredForApplication(application common.Address) (bool, error) {
	return ec.bondedECDSAKeepFactoryContract.IsOperatorRegistered(
		ec.Address(),
		application,
	)
}

// IsEligibleForApplication checks if the operator is eligible to register
// as a signer candidate for the given application.
func (ec *ethereumChain) IsEligibleForApplication(application common.Address) (bool, error) {
	return ec.bondedECDSAKeepFactoryContract.IsOperatorEligible(
		ec.Address(),
		application,
	)
}

// IsStatusUpToDateForApplication checks if the operator's status
// is up to date in the signers' pool of the given application.
func (ec *ethereumChain) IsStatusUpToDateForApplication(application common.Address) (bool, error) {
	return ec.bondedECDSAKeepFactoryContract.IsOperatorUpToDate(
		ec.Address(),
		application,
	)
}

// UpdateStatusForApplication updates the operator's status in the signers'
// pool for the given application.
func (ec *ethereumChain) UpdateStatusForApplication(application common.Address) error {
	transaction, err := ec.bondedECDSAKeepFactoryContract.UpdateOperatorStatus(
		ec.Address(),
		application,
	)
	if err != nil {
		return err
	}

	logger.Debugf(
		"submitted UpdateOperatorStatus transaction with hash: [%x]",
		transaction.Hash(),
	)

	return nil
}

// IsOperatorAuthorized checks if the factory has the authorization to
// operate on stake represented by the provided operator.
func (ec *ethereumChain) IsOperatorAuthorized(operator common.Address) (bool, error) {
	return ec.bondedECDSAKeepFactoryContract.IsOperatorAuthorized(operator)
}

// GetKeepCount returns number of keeps.
func (ec *ethereumChain) GetKeepCount() (*big.Int, error) {
	return ec.bondedECDSAKeepFactoryContract.GetKeepCount()
}

// GetKeepAtIndex returns the address of the keep at the given index.
func (ec *ethereumChain) GetKeepAtIndex(
	keepIndex *big.Int,
) (common.Address, error) {
	return ec.bondedECDSAKeepFactoryContract.GetKeepAtIndex(keepIndex)
}

// BlockTimestamp returns given block's timestamp.
func (ec *ethereumChain) BlockTimestamp(blockNumber *big.Int) (uint64, error) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelCtx()

	header, err := ec.client.HeaderByNumber(ctx, blockNumber)
	if err != nil {
		return 0, err
	}

	return header.Time, nil
}

// WeiBalanceOf returns the wei balance of the given address from the latest
// known block.
func (ec *ethereumChain) WeiBalanceOf(
	address common.Address,
) (*ethereum.Wei, error) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancelCtx()

	balance, err := ec.client.BalanceAt(ctx, address, nil)
	if err != nil {
		return nil, err
	}

	return ethereum.WrapWei(balance), err
}

// BalanceMonitor returns a balance monitor.
func (ec *ethereumChain) BalanceMonitor() (*ethutil.BalanceMonitor, error) {
	return ethutil.NewBalanceMonitor(ec.WeiBalanceOf), nil
}
