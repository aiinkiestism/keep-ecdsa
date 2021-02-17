module github.com/keep-network/keep-ecdsa

go 1.13

replace (
	github.com/BurntSushi/toml => github.com/keep-network/toml v0.3.0
	github.com/blockcypher/gobcy => github.com/keep-network/gobcy v1.3.1
	github.com/btcsuite/btcd => github.com/keep-network/btcd v0.0.0-20190427004231-96897255fd17
	github.com/btcsuite/btcutil => github.com/keep-network/btcutil v0.0.0-20190425235716-9e5f4b9a998d
	github.com/urfave/cli => github.com/keep-network/cli v1.20.0
)

require (
	github.com/BurntSushi/toml v0.3.1
	github.com/binance-chain/tss-lib v1.3.1
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/celo-org/celo-blockchain v0.0.0-20210211195335-cbc4f555cf87
	github.com/ethereum/go-ethereum v1.9.10
	github.com/gogo/protobuf v1.3.1
	github.com/google/gofuzz v1.1.0
	github.com/ipfs/go-log v1.0.4
	github.com/keep-network/keep-common v1.3.1-0.20210217111734-903205a570d0
	github.com/keep-network/keep-core v1.3.2-0.20210217115335-806bf9a58b01
	github.com/keep-network/tbtc v1.1.1-0.20210217115559-4a6a60e5274a
	github.com/pkg/errors v0.9.1
	github.com/urfave/cli v1.22.1
)
