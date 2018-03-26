package helper

import (
	"errors"
	"fmt"
	"nkn-core/wallet"
	. "nkn-core/common"
	. "nkn-core/core/asset"
	"nkn-core/core/contract"
	"nkn-core/core/transaction"
)

type BatchOut struct {
	Address string
	Value   string
}

func MakeRegTransaction(wallet wallet.Wallet, name string, value string) (*transaction.Transaction, error) {
	admin, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	issuer := admin
	asset := &Asset{name, name, byte(MaxPrecision), AssetType(Token), UTXO}
	transactionContract, err := contract.CreateSignatureContract(admin.PubKey())
	if err != nil {
		fmt.Println("CreateSignatureContract failed")
		return nil, err
	}
	fixedValue, err := StringToFixed64(value)
	if err != nil {
		return nil, err
	}
	txn, err := transaction.NewRegisterAssetTransaction(asset, fixedValue, issuer.PubKey(), transactionContract.ProgramHash)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeIssueTransaction(wallet wallet.Wallet, assetID Uint256, address string, value string) (*transaction.Transaction, error) {
	programHash, err := ToScriptHash(address)
	if err != nil {
		return nil, err
	}
	fixedValue, err := StringToFixed64(value)
	if err != nil {
		return nil, err
	}
	issueTxOutput := &transaction.TxOutput{
		AssetID:     assetID,
		Value:       fixedValue,
		ProgramHash: programHash,
	}
	outputs := []*transaction.TxOutput{issueTxOutput}
	txn, err := transaction.NewIssueAssetTransaction(outputs)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}

func MakeTransferTransaction(wallet wallet.Wallet, assetID Uint256, batchOut ...BatchOut) (*transaction.Transaction, error) {
	outputNum := len(batchOut)
	if outputNum == 0 {
		return nil, errors.New("nil outputs")
	}

	account, err := wallet.GetDefaultAccount()
	if err != nil {
		return nil, err
	}
	perOutputFee := Fixed64(0)
	var expected Fixed64
	input := []*transaction.UTXOTxInput{}
	output := []*transaction.TxOutput{}
	// construct transaction outputs
	for _, o := range batchOut {
		outputValue, err := StringToFixed64(o.Value)
		if err != nil {
			return nil, err
		}
		if outputValue <= perOutputFee {
			return nil, errors.New("token is not enough for transaction fee")
		}
		expected += outputValue
		address, err := ToScriptHash(o.Address)
		if err != nil {
			return nil, errors.New("invalid address")
		}
		tmp := &transaction.TxOutput{
			AssetID:     assetID,
			Value:       outputValue - perOutputFee,
			ProgramHash: address,
		}
		output = append(output, tmp)
	}

	// construct transaction inputs and changes
	unspent, err := wallet.GetUnspent()
	if err != nil {
		return nil, errors.New("get asset error")
	}
	for _, item := range unspent[assetID] {
		tmpInput := &transaction.UTXOTxInput{
			ReferTxID:          item.Txid,
			ReferTxOutputIndex: uint16(item.Index),
		}
		input = append(input, tmpInput)
		if item.Value > expected {
			changes := &transaction.TxOutput{
				AssetID:     assetID,
				Value:       item.Value - expected,
				ProgramHash: account.ProgramHash,
			}
			output = append(output, changes)
			expected = 0
			break
		} else if item.Value == expected {
			expected = 0
			break
		} else if item.Value < expected {
			expected = expected - item.Value
		}

	}
	if expected > 0 {
		return nil, errors.New("token is not enough")
	}

	// construct transaction
	txn, err := transaction.NewTransferAssetTransaction(input, output)
	if err != nil {
		return nil, err
	}

	// sign transaction contract
	ctx := contract.NewContractContext(txn)
	wallet.Sign(ctx)
	txn.SetPrograms(ctx.GetPrograms())

	return txn, nil
}