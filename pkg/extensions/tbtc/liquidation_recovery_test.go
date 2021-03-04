package tbtc

import (
	"testing"
)

func deriveAddressTester(t *testing.T, extendedAddress string, addressIndex int, expectedAddress string) {
	address, err := DeriveAddress(extendedAddress, addressIndex)

	if err != nil {
		t.Errorf(
			"Got %s while trying to derive %s at index %v",
			err,
			extendedAddress,
			addressIndex,
		)
	}

	if address != expectedAddress {
		t.Errorf(
			"Got %s instead of %s while trying to derive %s at index %v",
			address,
			expectedAddress,
			extendedAddress,
			addressIndex,
		)
	}
}

// These tests use https://iancoleman.io/bip39/ with the bip39 mnemonic: loyal
// chuckle trade magnet tobacco jungle craft cram reduce climb size flip tongue
// tornado height
func Test_DeriveAddress(t *testing.T) {
	// BIP44: xpub
	deriveAddressTester(
		t,
		"xpub6Cg41S21VrxkW1WBTZJn95KNpHozP2Xc6AhG27ZcvZvH8XyNzunEqLdk9dxyXQUoy7ALWQFNn5K1me74aEMtS6pUgNDuCYTTMsJzCAk9sk1",
		0,
		"1MjCqoLqMZ6Ru64TTtP16XnpSdiE8Kpgcx",
	)

	deriveAddressTester(
		t,
		"xpub6Cg41S21VrxkW1WBTZJn95KNpHozP2Xc6AhG27ZcvZvH8XyNzunEqLdk9dxyXQUoy7ALWQFNn5K1me74aEMtS6pUgNDuCYTTMsJzCAk9sk1",
		4,
		"1EEX8qZnTw1thadyxsueV748v3Y6tTMccc",
	)

	// BIP49: ypub
	deriveAddressTester(
		t,
		"ypub6Xxan668aiJqvh4SVfd7EzqjWvf36gWufTkhWHv3gaxnBh44HpkTi2TTkm1u136qjUxk7F3jGzoyfrGpHvALMgJgbF4WNXpoPu3QYrqogMK",
		0,
		"3Aobe26f7QzKN73mvYQVbt1KLrCU1CgQpD",
	)

	deriveAddressTester(
		t,
		"ypub6Xxan668aiJqvh4SVfd7EzqjWvf36gWufTkhWHv3gaxnBh44HpkTi2TTkm1u136qjUxk7F3jGzoyfrGpHvALMgJgbF4WNXpoPu3QYrqogMK",
		4,
		"3Ap2E4ap2ZqzUHkTT8ZZv2DJm6TqKukBAL",
	)

	// BIP84: zpub
	deriveAddressTester(
		t,
		"zpub6rePDVHfRP14VpYiejwepBhzu45UbvqvzE3ZMdDnNykG47mZYyGTjsuq6uzQYRakSrHyix1YTXKohag4GDZLcHcLvhSAs2MQNF8VDaZuQT9",
		0,
		"bc1q46uejlhm9vkswfcqs9plvujzzmqjvtfda3mra6",
	)

	deriveAddressTester(
		t,
		"zpub6rePDVHfRP14VpYiejwepBhzu45UbvqvzE3ZMdDnNykG47mZYyGTjsuq6uzQYRakSrHyix1YTXKohag4GDZLcHcLvhSAs2MQNF8VDaZuQT9",
		8,
		"bc1quq0vrufxy05ypk45xmu3hpk6qsmlhr5vr3n8kz",
	)

	// BIP141: ypub again
	deriveAddressTester(
		t,
		"ypub6ZrwUDiiKM2b6CmJy6TSntc3b7FTE7dcUcu36jQLsXDu1uKt8jt19ZZc43vzfGs2f2r1hxWpbz8tCzJibmzwp2piYjzkhUCRzdrWU5qUoVZ",
		0,
		"3Aobe26f7QzKN73mvYQVbt1KLrCU1CgQpD",
	)

	deriveAddressTester(
		t,
		"ypub6ZrwUDiiKM2b6CmJy6TSntc3b7FTE7dcUcu36jQLsXDu1uKt8jt19ZZc43vzfGs2f2r1hxWpbz8tCzJibmzwp2piYjzkhUCRzdrWU5qUoVZ",
		16,
		"3JuNnoMh8eWhtY5YLk3SMXfw7vm8y6zPLg",
	)
}
