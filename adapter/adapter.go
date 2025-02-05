package adapter

import (
	"encoding/binary"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

type RawTransaction struct {
	Slot        uint64
	Transaction *solana.Transaction
	Meta        *rpc.TransactionMeta
	Index       uint32
	BlockTime   time.Time
}

func GrpcToJsonRpc(txn *pb.SubscribeUpdateTransaction) *RawTransaction {
	tx := txn.GetTransaction().GetTransaction()

	gtxSigntrues := tx.GetSignatures()
	var signatures []solana.Signature
	for _, b := range gtxSigntrues {
		signatures = append(signatures, solana.SignatureFromBytes(b))
	}

	var pubk []solana.PublicKey
	for _, k := range tx.Message.AccountKeys {
		pubk = append(pubk, solana.PublicKeyFromBytes(k))
	}

	blockHash, err := ConvertToFixed32(tx.Message.RecentBlockhash)
	if err != nil {
		log.Fatal(err)
	}

	var in []solana.CompiledInstruction
	for _, ins := range tx.Message.Instructions {
		in = append(in, solana.CompiledInstruction{
			ProgramIDIndex: uint16(ins.ProgramIdIndex),
			Accounts:       ConvertAccountsToUint16(ins.Accounts),
			Data:           ins.Data,
		})
	}

	var addressTable []solana.MessageAddressTableLookup
	for _, ins := range tx.Message.AddressTableLookups {
		addressTable = append(addressTable, solana.MessageAddressTableLookup{
			AccountKey:      solana.PublicKeyFromBytes(ins.AccountKey),
			WritableIndexes: ins.WritableIndexes,
			ReadonlyIndexes: ins.ReadonlyIndexes,
		})
	}
	message := solana.Message{
		AccountKeys: pubk,
		Header: solana.MessageHeader{
			NumRequiredSignatures:       uint8(tx.Message.Header.NumRequiredSignatures),
			NumReadonlySignedAccounts:   uint8(tx.Message.Header.NumReadonlySignedAccounts),
			NumReadonlyUnsignedAccounts: uint8(tx.Message.Header.NumReadonlyUnsignedAccounts),
		},
		RecentBlockhash:     blockHash,
		Instructions:        in,
		AddressTableLookups: addressTable,
	}
	// change Transaction
	transaction := &solana.Transaction{
		Signatures: signatures,
		Message:    message,
	}

	meta := txn.Transaction.Meta

	innerInstructions := func() []rpc.InnerInstruction {
		var in []solana.CompiledInstruction
		for _, ii := range meta.GetInnerInstructions() {
			iii := ii.GetInstructions()
			for _, iiii := range iii {
				in = append(in, solana.CompiledInstruction{
					ProgramIDIndex: uint16(iiii.ProgramIdIndex),
					Accounts:       ConvertAccountsToUint16(iiii.Accounts),
					Data:           iiii.Data,
				})
			}
		}
		var result []rpc.InnerInstruction
		for _, inner := range meta.GetInnerInstructions() {
			result = append(result, rpc.InnerInstruction{
				Index:        uint16(inner.Index),
				Instructions: in,
			})

		}
		return result
	}()

	preTokenBalance := func() []rpc.TokenBalance {
		var result []rpc.TokenBalance

		for _, ii := range meta.PreTokenBalances {
			owner := solana.MustPublicKeyFromBase58(ii.Owner)
			pgId := solana.MustPublicKeyFromBase58(ii.ProgramId)
			mint := solana.MustPublicKeyFromBase58(ii.Mint)
			result = append(result, rpc.TokenBalance{
				AccountIndex: uint16(ii.AccountIndex),
				Owner:        &owner,
				ProgramId:    &pgId,
				Mint:         mint,
				UiTokenAmount: &rpc.UiTokenAmount{
					Amount:         ii.UiTokenAmount.Amount,
					Decimals:       uint8(ii.UiTokenAmount.Decimals),
					UiAmount:       &ii.UiTokenAmount.UiAmount,
					UiAmountString: ii.UiTokenAmount.UiAmountString,
				},
			})
		}

		return result

	}()
	postTokenBalance := func() []rpc.TokenBalance {
		var result []rpc.TokenBalance

		for _, ii := range meta.PostTokenBalances {
			owner := solana.MustPublicKeyFromBase58(ii.Owner)
			pgId := solana.MustPublicKeyFromBase58(ii.ProgramId)
			mint := solana.MustPublicKeyFromBase58(ii.Mint)
			result = append(result, rpc.TokenBalance{
				AccountIndex: uint16(ii.AccountIndex),
				Owner:        &owner,
				ProgramId:    &pgId,
				Mint:         mint,
				UiTokenAmount: &rpc.UiTokenAmount{
					Amount:         ii.UiTokenAmount.Amount,
					Decimals:       uint8(ii.UiTokenAmount.Decimals),
					UiAmount:       &ii.UiTokenAmount.UiAmount,
					UiAmountString: ii.UiTokenAmount.UiAmountString,
				},
			})
		}

		return result

	}()

	blockReward := func() []rpc.BlockReward {
		var result []rpc.BlockReward
		for _, ii := range meta.Rewards {
			pk := solana.MustPublicKeyFromBase58(ii.Pubkey)
			commission, err := strconv.Atoi(ii.Commission)
			if err != nil {
				return nil
			}
			commissionU8 := uint8(commission)
			result = append(result, rpc.BlockReward{
				Pubkey:      pk,
				Lamports:    ii.Lamports,
				PostBalance: ii.PostBalance,
				RewardType:  rpc.RewardType(ii.RewardType.String()),
				Commission:  &commissionU8,
			})
		}
		return result

	}()

	loadedAddress := func() rpc.LoadedAddresses {
		var readOnly []solana.PublicKey
		for _, ii := range meta.LoadedReadonlyAddresses {
			readOnly = append(readOnly, solana.PublicKeyFromBytes(ii))
		}

		var writable []solana.PublicKey
		for _, ii := range meta.LoadedWritableAddresses {
			writable = append(readOnly, solana.PublicKeyFromBytes(ii))
		}

		return rpc.LoadedAddresses{
			ReadOnly: readOnly,
			Writable: writable,
		}

	}()

	return &RawTransaction{
		Slot:        txn.Slot,
		Transaction: transaction,
		Meta: &rpc.TransactionMeta{
			Err:               meta.Err,
			Fee:               meta.Fee,
			PreBalances:       meta.PreBalances,
			PostBalances:      meta.PostBalances,
			InnerInstructions: innerInstructions,
			PreTokenBalances:  preTokenBalance,
			PostTokenBalances: postTokenBalance,
			LogMessages:       meta.GetLogMessages(),
			Status:            map[string]interface{}{},
			Rewards:           blockReward,
			LoadedAddresses:   loadedAddress,
			ReturnData: rpc.ReturnData{
				ProgramId: solana.PublicKey(meta.ReturnData.ProgramId),
				Data: solana.Data{
					Content:  meta.ReturnData.GetData(),
					Encoding: "base64",
				},
			},
			ComputeUnitsConsumed: meta.ComputeUnitsConsumed,
		},
		Index:     uint32(txn.GetTransaction().Index),
		BlockTime: time.Time{},
	}

}

func ConvertToFixed32(input []byte) ([32]byte, error) {
	var result [32]byte
	if len(input) != 32 {
		return result, fmt.Errorf("invalid length: expected 32, got %d", len(input))
	}
	copy(result[:], input)
	return result, nil
}

func ConvertAccountsToUint16(accountsBytes []byte) []uint16 {
	if len(accountsBytes)%2 != 0 {
		return nil
	}

	result := make([]uint16, len(accountsBytes)/2)

	for i := 0; i < len(accountsBytes); i += 2 {
		result[i/2] = binary.LittleEndian.Uint16(accountsBytes[i:])
	}

	return result
}
