package local

import (
	"context"
	"fmt"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	eth "github.com/keep-network/keep-ecdsa/pkg/chain"
)

func TestOnBondedECDSAKeepCreated(t *testing.T) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelCtx()

	chain := initializeLocalChain(ctx)
	eventFired := make(chan *eth.BondedECDSAKeepCreatedEvent)
	keepAddress := common.Address([20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	expectedEvent := &eth.BondedECDSAKeepCreatedEvent{
		KeepAddress: keepAddress,
	}

	subscription := chain.OnBondedECDSAKeepCreated(
		func(event *eth.BondedECDSAKeepCreatedEvent) {
			eventFired <- event
		},
	)
	defer subscription.Unsubscribe()

	err := chain.createKeep(keepAddress)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case event := <-eventFired:
		if !reflect.DeepEqual(event, expectedEvent) {
			t.Fatalf(
				"unexpected keep creation event\nexpected: [%v]\nactual:   [%v]",
				expectedEvent,
				event,
			)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestOnSignatureRequested(t *testing.T) {
	ctx, cancelCtx := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancelCtx()

	chain := initializeLocalChain(ctx)
	eventFired := make(chan *eth.SignatureRequestedEvent)
	keepAddress := common.Address([20]byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})
	digest := [32]byte{1}

	err := chain.createKeep(keepAddress)
	if err != nil {
		t.Fatal(err)
	}

	var keepPubkey [64]byte
	rand.Read(keepPubkey[:])

	err = chain.SubmitKeepPublicKey(keepAddress, keepPubkey)
	if err != nil {
		t.Fatal(err)
	}

	subscription, err := chain.OnSignatureRequested(
		keepAddress,
		func(event *eth.SignatureRequestedEvent) {
			eventFired <- event
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	defer subscription.Unsubscribe()

	err = chain.requestSignature(keepAddress, digest)
	if err != nil {
		t.Fatal(err)
	}

	expectedEvent := &eth.SignatureRequestedEvent{
		Digest: digest,
	}

	select {
	case event := <-eventFired:
		if !reflect.DeepEqual(event, expectedEvent) {
			t.Fatalf(
				"unexpected signature requested event\nexpected: [%v]\nactual:   [%v]",
				expectedEvent,
				event,
			)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
}

func TestSubmitKeepPublicKey(t *testing.T) {
	ctx, cancelCtx := context.WithCancel(context.Background())
	defer cancelCtx()

	chain := initializeLocalChain(ctx)
	keepAddress := common.HexToAddress("0x41048F9B90290A2e96D07f537F3A7E97620E9e47")
	keepPublicKey := [64]byte{11, 12, 13, 14, 15, 16}
	expectedDuplicationError := fmt.Errorf(
		"public key already submitted for keep [%s]",
		keepAddress.String(),
	)

	err := chain.createKeep(keepAddress)
	if err != nil {
		t.Fatal(err)
	}

	err = chain.SubmitKeepPublicKey(
		keepAddress,
		keepPublicKey,
	)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(keepPublicKey, chain.keeps[keepAddress].publicKey) {
		t.Errorf(
			"unexpected result\nexpected: [%+v]\nactual:   [%+v]",
			keepPublicKey,
			chain.keeps[keepAddress].publicKey,
		)
	}

	err = chain.SubmitKeepPublicKey(
		keepAddress,
		keepPublicKey,
	)
	if !reflect.DeepEqual(expectedDuplicationError, err) {
		t.Errorf(
			"unexpected error\nexpected: [%+v]\nactual:   [%+v]",
			expectedDuplicationError,
			err,
		)
	}
}

func initializeLocalChain(ctx context.Context) *localChain {
	return Connect(ctx).(*localChain)
}
