//+build !celo

package ethereum

import (
	"context"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/keep-network/keep-common/pkg/rate"

	"github.com/keep-network/keep-common/pkg/chain/ethereum/ethutil"
	"github.com/keep-network/keep-common/pkg/chain/ethlike"

	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/keep-network/keep-common/pkg/chain/ethereum"
	"github.com/keep-network/keep-ecdsa/pkg/chain"
	"github.com/keep-network/keep-ecdsa/pkg/chain/gen/ethereum/contract"
)

// Definitions of contract names.
const (
	BondedECDSAKeepFactoryContractName = "BondedECDSAKeepFactory"
)

var (
	// DefaultMiningCheckInterval is the default interval in which transaction
	// mining status is checked. If the transaction is not mined within this
	// time, the gas price is increased and transaction is resubmitted.
	// This value can be overwritten in the configuration file.
	DefaultMiningCheckInterval = 60 * time.Second

	// DefaultMaxGasPrice specifies the default maximum gas price the client is
	// willing to pay for the transaction to be mined. The offered transaction
	// gas price can not be higher than the max gas price value. If the maximum
	// allowed gas price is reached, no further resubmission attempts are
	// performed. This value can be overwritten in the configuration file.
	DefaultMaxGasPrice = big.NewInt(500000000000) // 500 Gwei
)

// Chain is an implementation of ethereum blockchain interface.
type Chain struct {
	config                         *ethereum.Config
	accountKey                     *keystore.Key
	client                         ethutil.EthereumClient
	bondedECDSAKeepFactoryContract *contract.BondedECDSAKeepFactory
	blockCounter                   *ethlike.BlockCounter
	miningWaiter                   *ethlike.MiningWaiter
	nonceManager                   *ethlike.NonceManager

	// transactionMutex allows interested parties to forcibly serialize
	// transaction submission.
	//
	// When transactions are submitted, they require a valid nonce. The nonce is
	// equal to the count of transactions the account has submitted so far, and
	// for a transaction to be accepted it should be monotonically greater than
	// any previous submitted transaction. To do this, transaction submission
	// asks the Ethereum client it is connected to for the next pending nonce,
	// and uses that value for the transaction. Unfortunately, if multiple
	// transactions are submitted in short order, they may all get the same
	// nonce. Serializing submission ensures that each nonce is requested after
	// a previous transaction has been submitted.
	transactionMutex *sync.Mutex
}

// Connect performs initialization for communication with Ethereum blockchain
// based on provided config.
func Connect(
	ctx context.Context,
	accountKey *keystore.Key,
	config *ethereum.Config,
	tbtcSystemAddress string,
) (*Chain, chain.TBTCHandle, error) {
	client, err := ethclient.Dial(config.URL)
	if err != nil {
		return nil, nil, err
	}

	wrappedClient := addClientWrappers(config, client)

	transactionMutex := &sync.Mutex{}

	nonceManager := ethutil.NewNonceManager(wrappedClient, accountKey.Address)

	checkInterval := DefaultMiningCheckInterval
	maxGasPrice := DefaultMaxGasPrice
	if config.MiningCheckInterval != 0 {
		checkInterval = time.Duration(config.MiningCheckInterval) * time.Second
	}
	if config.MaxGasPrice != nil {
		maxGasPrice = config.MaxGasPrice.Int
	}

	logger.Infof("using [%v] mining check interval", checkInterval)
	logger.Infof("using [%v] wei max gas price", maxGasPrice)

	miningWaiter := ethutil.NewMiningWaiter(
		wrappedClient,
		checkInterval,
		maxGasPrice,
	)

	blockCounter, err := ethutil.NewBlockCounter(wrappedClient)
	if err != nil {
		return nil, nil, fmt.Errorf(
			"failed to create Ethereum blockcounter: [%v]",
			err,
		)
	}

	bondedECDSAKeepFactoryContractAddress, err := config.ContractAddress(
		BondedECDSAKeepFactoryContractName,
	)
	if err != nil {
		return nil, nil, err
	}
	bondedECDSAKeepFactoryContract, err := contract.NewBondedECDSAKeepFactory(
		*bondedECDSAKeepFactoryContractAddress,
		accountKey,
		wrappedClient,
		nonceManager,
		miningWaiter,
		blockCounter,
		transactionMutex,
	)
	if err != nil {
		return nil, nil, err
	}

	ethereum := &Chain{
		config:                         config,
		accountKey:                     accountKey,
		client:                         wrappedClient,
		bondedECDSAKeepFactoryContract: bondedECDSAKeepFactoryContract,
		blockCounter:                   blockCounter,
		nonceManager:                   nonceManager,
		miningWaiter:                   miningWaiter,
		transactionMutex:               transactionMutex,
	}

	ethereum.initializeBalanceMonitoring(ctx)

	return ethereum, ethereum.buildTBTC(tbtcSystemAddress), nil
}

func addClientWrappers(
	config *ethereum.Config,
	client ethutil.EthereumClient,
) ethutil.EthereumClient {
	loggingClient := ethutil.WrapCallLogging(logger, client)

	if config.RequestsPerSecondLimit > 0 || config.ConcurrencyLimit > 0 {
		logger.Infof(
			"enabled ethereum rate limiter; "+
				"rps limit [%v]; "+
				"concurrency limit [%v]",
			config.RequestsPerSecondLimit,
			config.ConcurrencyLimit,
		)

		return ethutil.WrapRateLimiting(
			loggingClient,
			&rate.LimiterConfig{
				RequestsPerSecondLimit: config.RequestsPerSecondLimit,
				ConcurrencyLimit:       config.ConcurrencyLimit,
			},
		)
	}

	return loggingClient
}

// FIXME Rip this back out to connector_ethereum.go after tBTC handle refactor.
func (c *Chain) buildTBTC(tbtcSystemAddress string) chain.TBTCHandle {
	if len(tbtcSystemAddress) > 0 {
		tbtcEthereumChain, err := WithTBTCExtension(
			c,
			tbtcSystemAddress,
		)
		if err != nil {
			return nil
		}

		return tbtcEthereumChain
	}

	return nil
}
