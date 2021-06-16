package tss

import (
	"context"
	cecdsa "crypto/ecdsa"
	"fmt"
	"sort"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil"
	"github.com/keep-network/keep-core/pkg/net"
)

// recoveryInfo represents the broadcasted information needed from the other
// signers to complete liquidation recovery.
type recoveryInfo struct {
	btcRecoveryAddress string
	maxFeePerVByte     int32
}

// BroadcastRecoveryAddress broadcasts and receives the BTC recovery addresses
// of each client so that each client can retrieve the underlying bitcoin in
// the case that a keep is terminated.
func BroadcastRecoveryAddress(
	parentCtx context.Context,
	btcRecoveryAddress string,
	maxFeePerVByte int32,
	groupID string,
	memberID MemberID,
	groupMemberIDs []MemberID,
	dishonestThreshold uint,
	networkProvider net.Provider,
	pubKeyToAddressFn func(cecdsa.PublicKey) []byte,
	chainParams *chaincfg.Params,
) ([]string, int32, error) {
	const protocolReadyTimeout = 2 * time.Minute

	group := &groupInfo{
		groupID:            groupID,
		memberID:           memberID,
		groupMemberIDs:     groupMemberIDs,
		dishonestThreshold: int(dishonestThreshold),
	}

	netBridge, _ := newNetworkBridge(group, networkProvider)
	broadcastChannel, _ := netBridge.getBroadcastChannel()
	ctx, cancel := context.WithTimeout(parentCtx, protocolReadyTimeout)
	defer cancel()

	msgInChan := make(chan *LiquidationRecoveryAnnounceMessage, len(group.groupMemberIDs))
	handleLiquidationRecoveryAnnounceMessage := func(netMsg net.Message) {
		switch msg := netMsg.Payload().(type) {
		case *LiquidationRecoveryAnnounceMessage:
			msgInChan <- msg
		}
	}
	broadcastChannel.Recv(ctx, handleLiquidationRecoveryAnnounceMessage)

	memberRecoveryInfo := make(map[string]recoveryInfo)

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-msgInChan:
				for _, memberID := range group.groupMemberIDs {
					if msg.SenderID.Equal(memberID) {
						memberAddress, err := memberIDToAddress(
							memberID,
							pubKeyToAddressFn,
						)
						if err != nil {
							logger.Errorf(
								"could not convert member ID to address for "+
									"a member of keep [%s]: [%v]",
								group.groupID,
								err,
							)
							break
						}
						memberRecoveryInfo[memberAddress] = recoveryInfo{btcRecoveryAddress: msg.BtcRecoveryAddress, maxFeePerVByte: msg.MaxFeePerVByte}

						logger.Infof(
							"member [%s] from keep [%s] announced supplied btc address [%s] for "+
								"liquidation recovery with a max fee of [%v]",
							memberAddress,
							group.groupID,
							msg.BtcRecoveryAddress,
							msg.MaxFeePerVByte,
						)

						break
					}
				}

				if len(memberRecoveryInfo) == len(group.groupMemberIDs) {
					cancel()
				}
			}
		}
	}()

	go func() {
		sendMessage := func() {
			if err := broadcastChannel.Send(ctx,
				&LiquidationRecoveryAnnounceMessage{
					SenderID:           group.memberID,
					BtcRecoveryAddress: btcRecoveryAddress,
					MaxFeePerVByte:     maxFeePerVByte,
				},
			); err != nil {
				logger.Errorf("failed to send btc recovery address: [%v]", err)
			}
		}

		// Send the message first time. It will be periodically retransmitted
		// by the broadcast channel for the entire lifetime of the context.
		sendMessage()

		<-ctx.Done()
		// Send the message once again as the member received messages
		// from all peer members but not all peer members could receive
		// the message from the member as some peer member could join
		// the protocol after the member sent the last message.
		sendMessage()
		return
	}()

	<-ctx.Done()

	switch ctx.Err() {
	case context.DeadlineExceeded:
		for _, memberID := range group.groupMemberIDs {
			memberAddress, err := memberIDToAddress(memberID, pubKeyToAddressFn)
			if err != nil {
				logger.Errorf(
					"could not convert member ID to address for a member of "+
						"keep [%s]: [%v]",
					group.groupID,
					err,
				)
				continue
			}
			if _, present := memberRecoveryInfo[memberAddress]; !present {
				logger.Errorf(
					"member [%s] has not supplied a btc recovery address for keep [%s]; "+
						"check if keep client for that operator is active and "+
						"connected",
					memberAddress,
					group.groupID,
				)
			}
		}
		return nil, 0, fmt.Errorf(
			"waiting for btc recovery addresses timed out after: [%v]", protocolReadyTimeout,
		)
	case context.Canceled:
		logger.Infof("successfully gathered all btc addresses")

		retrievalAddresses := make([]string, 0, len(memberRecoveryInfo))
		maxFeePerVByte := int32(2147483647) // since we're taking the min fee among the signers, start with the max int32

		for memberID, recoveryInfo := range memberRecoveryInfo {
			if err := ValidateReceivedBtcAddress(recoveryInfo.btcRecoveryAddress, chainParams); err != nil {
				return nil, 0, fmt.Errorf(
					"failed to validate btc address [%s] received from [%s] during recovery broadcast: [%w]",
					recoveryInfo.btcRecoveryAddress,
					memberID,
					err,
				)
			}
			retrievalAddresses = append(retrievalAddresses, recoveryInfo.btcRecoveryAddress)

			if recoveryInfo.maxFeePerVByte < maxFeePerVByte {
				maxFeePerVByte = recoveryInfo.maxFeePerVByte
			}
		}

		sort.Strings(retrievalAddresses)

		return retrievalAddresses, maxFeePerVByte, nil
	default:
		return nil, 0, fmt.Errorf("unexpected context error: [%v]", ctx.Err())
	}
}

// ValidateReceivedBtcAddress checks to see if the btc address is valid on the
// supplied chain. At this stage we expect to receive final btc address from a peer
// member, so address derivation from *pub key won't be executed.
func ValidateReceivedBtcAddress(btcAddress string, chainParams *chaincfg.Params) error {
	decodedAddress, decodeErr := btcutil.DecodeAddress(btcAddress, chainParams)
	if decodeErr != nil {
		return fmt.Errorf(
			"failed to decode address [%s] for chain [%s]",
			btcAddress,
			chainParams.Name,
		)
	}

	if !decodedAddress.IsForNet(chainParams) {
		return fmt.Errorf(
			"address [%s] is not a valid btc address for chain [%s]",
			btcAddress,
			chainParams.Name,
		)
	}

	return nil
}
