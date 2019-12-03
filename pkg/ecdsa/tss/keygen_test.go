package tss

import (
	"context"
	cecdsa "crypto/ecdsa"
	"crypto/sha256"
	"fmt"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/ipfs/go-log"
	"github.com/keep-network/keep-core/pkg/beacon/relay/group"
	"github.com/keep-network/keep-core/pkg/net/key"
	"github.com/keep-network/keep-tecdsa/internal/testdata"
	"github.com/keep-network/keep-tecdsa/pkg/ecdsa"
	"github.com/keep-network/keep-tecdsa/pkg/net"
	"github.com/keep-network/keep-tecdsa/pkg/net/local"
	"github.com/keep-network/keep-tecdsa/pkg/utils/testutils"
)

func TestGenerateKeyAndSign(t *testing.T) {
	ctx, _ := context.WithTimeout(context.Background(), 10*time.Second)

	groupSize := 5
	threshold := groupSize - 1

	err := log.SetLogLevel("*", "INFO")
	if err != nil {
		t.Errorf("logger initialization failed: [%v]", err)
	}

	completed := make(chan interface{})
	errChan := make(chan error)

	// go func() {
	groupMemberIDs := []group.MemberIndex{}
	groupMembersKeys := make(map[group.MemberIndex]*key.NetworkPublic, groupSize)

	for i := 0; i < groupSize; i++ {
		_, publicKey, err := key.GenerateStaticNetworkKey()
		if err != nil {
			t.Fatalf("failed to generate network key: [%v]", err)
		}

		memberIndex := group.MemberIndex(i + 1)

		groupMemberIDs = append(groupMemberIDs, memberIndex)
		groupMembersKeys[memberIndex] = publicKey
	}

	testData, err := testdata.LoadKeygenTestFixtures(groupSize)
	if err != nil {
		t.Fatalf("failed to load test data: [%v]", err)
	}

	// Signer initialization.
	signers := []*Signer{}

	// Signer initialization.
	for i, memberID := range groupMemberIDs {
		network, err := newTestNetProvider(memberID, groupMembersKeys, errChan)

		preParams := testData[i].LocalPreParams

		signer, err := InitializeSigner(
			memberID,
			groupSize,
			threshold,
			&preParams,
			network,
		)
		if err != nil {
			t.Fatalf("failed to initialize signer: [%v]", err)
		}

		signers = append(signers, signer)
	}

	if len(signers) != len(groupMemberIDs) {
		t.Fatalf(
			"unexpected number of signers\nexpected: %d\nactual:   %d\n",
			len(groupMemberIDs),
			len(signers),
		)
	}

	// Key generaton.
	go func() {
		var keyGenWait sync.WaitGroup
		keyGenWait.Add(len(signers))

		for _, signer := range signers {
			go func(signer *Signer) {
				go func() {
					for {
						select {
						case err := <-signer.keygenErrChan:
							errChan <- err
							return
						}
					}
				}()

				err = signer.GenerateKey()
				if err != nil {
					errChan <- fmt.Errorf("failed to generate key: [%v]", err)
					return
				}

				keyGenWait.Done()
			}(signer)
		}

		keyGenWait.Wait()
		completed <- "DONE"
	}()

	select {
	case <-completed:
	case err := <-errChan:
		t.Fatalf("unexpected error on key generation: [%v]", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	firstPublicKey := signers[0].PublicKey()
	curve := secp256k1.S256()

	if !curve.IsOnCurve(firstPublicKey.X, firstPublicKey.Y) {
		t.Error("public key is not on curve")
	}

	for i, signer := range signers {
		publicKey := signer.PublicKey()
		if publicKey.X.Cmp(firstPublicKey.X) != 0 || publicKey.Y.Cmp(firstPublicKey.Y) != 0 {
			t.Errorf("public key for party [%v] doesn't match expected", i)
		}
	}

	// Signing initialization.
	message := []byte("message to sign")
	digest := sha256.Sum256(message)

	var initSigningWait sync.WaitGroup
	initSigningWait.Add(len(signers))

	for _, signer := range signers {
		go func(signer *Signer) {
			networkChannel, err := network.getTestChannel(signer.keygenParty.PartyID().GetKey())
			if err != nil {
				t.Errorf("failed to get test channel: [%v]", err)
			}

			signer.InitializeSigning(digest[:], networkChannel)

			initSigningWait.Done()
		}(signer)
	}

	initSigningWait.Wait()

	// Signing.
	signatures := []*ecdsa.Signature{}
	signaturesMutex := &sync.Mutex{}

	var signingWait sync.WaitGroup
	signingWait.Add(len(signers))

	for _, signer := range signers {
		go func(signer *Signer) {
			signature, err := signer.Sign()
			if err != nil {
				t.Errorf("failed to sign: [%v]", err)
			}

			signaturesMutex.Lock()
			signatures = append(signatures, signature)
			signaturesMutex.Unlock()

			signingWait.Done()
		}(signer)
	}

	signingWait.Wait()

	if len(signatures) != groupSize {
		t.Errorf("invalid number of signatures\nexpected: %d\nactual:   %d", len(signers), len(signatures))
	}

	firstSignature := signatures[0]
	for i, signature := range signatures {
		if !reflect.DeepEqual(firstSignature, signature) {
			t.Errorf(
				"signature for party [%v] doesn't match expected\nexpected: [%v]\nactual: [%v]",
				i,
				firstSignature,
				signature,
			)
		}
	}

	signerPublicKey := signers[0].PublicKey()

	if !cecdsa.Verify(
		(*cecdsa.PublicKey)(signerPublicKey),
		digest[:],
		firstSignature.R,
		firstSignature.S,
	) {
		t.Errorf("invalid signature: [%+v]", firstSignature)
	}

	testutils.VerifyEthereumSignature(t, digest[:], firstSignature, signerPublicKey)
}

type testNetProvider struct {
}

func newTestNetProvider(
	memberID group.MemberIndex,
	membersNetworkKeys map[group.MemberIndex]*key.NetworkPublic,
	errChan chan error,
) (net.Provider, error) {
	provider := local.LocalProvider(
		memberID.Int().String(),
		membersNetworkKeys[memberID],
		errChan,
	)

	return provider, nil
}
