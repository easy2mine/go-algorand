// Copyright (C) 2019 Algorand, Inc.
// This file is part of go-algorand
//
// go-algorand is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// go-algorand is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with go-algorand.  If not, see <https://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/algorand/go-algorand/config"
	"github.com/algorand/go-algorand/crypto"
	"github.com/algorand/go-algorand/crypto/passphrase"
	algodAcct "github.com/algorand/go-algorand/data/account"
	"github.com/algorand/go-algorand/data/basics"
	"github.com/algorand/go-algorand/data/transactions"
	"github.com/algorand/go-algorand/libgoal"
	"github.com/algorand/go-algorand/protocol"
	"github.com/algorand/go-algorand/util"
	"github.com/algorand/go-algorand/util/db"
)

var (
	accountAddress     string
	walletName         string
	defaultAccountName string
	defaultAccount     bool
	unencryptedWallet  bool
	online             bool
	accountName        string
	transactionFee     uint64
	onlineFirstRound   uint64
	onlineValidRounds  uint64
	onlineTxFile       string
	roundFirstValid    uint64
	roundLastValid     uint64
	keyDilution        uint64
	threshold          uint8
	partKeyOutDir      string
	importDefault      bool
	mnemonic           string
)

func init() {
	accountCmd.AddCommand(newCmd)
	accountCmd.AddCommand(deleteCmd)
	accountCmd.AddCommand(listCmd)
	accountCmd.AddCommand(renameCmd)
	accountCmd.AddCommand(balanceCmd)
	accountCmd.AddCommand(rewardsCmd)
	accountCmd.AddCommand(changeOnlineCmd)
	accountCmd.AddCommand(addParticipationKeyCmd)
	accountCmd.AddCommand(listParticipationKeysCmd)
	accountCmd.AddCommand(importCmd)
	accountCmd.AddCommand(exportCmd)
	accountCmd.AddCommand(importRootKeysCmd)
	accountCmd.AddCommand(accountMultisigCmd)

	accountMultisigCmd.AddCommand(newMultisigCmd)
	accountMultisigCmd.AddCommand(deleteMultisigCmd)
	accountMultisigCmd.AddCommand(infoMultisigCmd)

	accountCmd.AddCommand(renewParticipationKeyCmd)
	accountCmd.AddCommand(renewAllParticipationKeyCmd)

	accountCmd.AddCommand(partkeyInfoCmd)

	// Wallet to be used for the account operation
	accountCmd.PersistentFlags().StringVarP(&walletName, "wallet", "w", "", "Set the wallet to be used for the selected operation")

	// Account Flag
	accountCmd.Flags().StringVarP(&defaultAccountName, "default", "f", "", "Set the account with this name to be the default account")

	// New Account flag
	newCmd.Flags().BoolVarP(&defaultAccount, "default", "f", false, "Set this account as the default one")

	// Delete account flag
	deleteCmd.Flags().StringVarP(&accountAddress, "addr", "a", "", "Address of account to delete")
	deleteCmd.MarkFlagRequired("addr")

	// New Multisig account flag
	newMultisigCmd.Flags().Uint8VarP(&threshold, "threshold", "T", 1, "Number of signatures required to spend from this address")
	newMultisigCmd.MarkFlagRequired("threshold")

	// Delete multisig account flag
	deleteMultisigCmd.Flags().StringVarP(&accountAddress, "addr", "a", "", "Address of multisig account to delete")
	deleteMultisigCmd.MarkFlagRequired("addr")

	// Lookup info for multisig account flag
	infoMultisigCmd.Flags().StringVarP(&accountAddress, "addr", "a", "", "Address of multisig account to look up")
	infoMultisigCmd.MarkFlagRequired("addr")

	// Balance flags
	balanceCmd.Flags().StringVarP(&accountAddress, "address", "a", "", "Account address to retrieve balance (required)")
	balanceCmd.MarkFlagRequired("address")

	// Rewards flags
	rewardsCmd.Flags().StringVarP(&accountAddress, "address", "a", "", "Account address to retrieve rewards (required)")
	rewardsCmd.MarkFlagRequired("address")

	// changeOnlineStatus flags
	changeOnlineCmd.Flags().StringVarP(&accountAddress, "address", "a", "", "Account address to change (required)")
	changeOnlineCmd.MarkFlagRequired("address")
	changeOnlineCmd.Flags().BoolVarP(&online, "online", "o", true, "Set this account to online or offline")
	changeOnlineCmd.MarkFlagRequired("online")
	changeOnlineCmd.Flags().Uint64VarP(&transactionFee, "fee", "f", 0, "The Fee to set on the status change transaction (defaults to suggested fee)")
	changeOnlineCmd.Flags().Uint64VarP(&onlineFirstRound, "firstRound", "", 0, "FirstValid for the status change transaction (0 for current)")
	changeOnlineCmd.Flags().Uint64VarP(&onlineValidRounds, "validRounds", "v", 0, "The validity period for the status change transaction")
	changeOnlineCmd.Flags().StringVarP(&onlineTxFile, "txfile", "t", "", "Write status change transaction to this file")
	changeOnlineCmd.Flags().BoolVarP(&noWaitAfterSend, "no-wait", "N", false, "Don't wait for transaction to commit")

	// addParticipationKey flags
	addParticipationKeyCmd.Flags().StringVarP(&accountAddress, "address", "a", "", "Account to associate with the generated partkey")
	addParticipationKeyCmd.MarkFlagRequired("address")
	addParticipationKeyCmd.Flags().Uint64VarP(&roundFirstValid, "roundFirstValid", "", 0, "The first round for which the generated partkey will be valid")
	addParticipationKeyCmd.MarkFlagRequired("roundFirstValid")
	addParticipationKeyCmd.Flags().Uint64VarP(&roundLastValid, "roundLastValid", "", 0, "The last round for which the generated partkey will be valid")
	addParticipationKeyCmd.MarkFlagRequired("roundLastValid")
	addParticipationKeyCmd.Flags().StringVarP(&partKeyOutDir, "outdir", "o", "", "Save participation key file to specified output directory to (for offline creation)")
	addParticipationKeyCmd.Flags().Uint64VarP(&keyDilution, "keyDilution", "", 0, "Key dilution for two-level participation keys")

	// import flags
	importCmd.Flags().BoolVarP(&importDefault, "default", "f", false, "Set this account as the default one")
	importCmd.Flags().StringVarP(&mnemonic, "mnemonic", "m", "", "Mnemonic to import (will prompt otherwise)")
	// export flags
	exportCmd.Flags().StringVarP(&accountAddress, "address", "a", "", "Address of account to export")
	exportCmd.MarkFlagRequired("address")
	// importRootKeys flags
	importRootKeysCmd.Flags().BoolVarP(&unencryptedWallet, "unencrypted-wallet", "u", false, "Import into the default unencrypted wallet, potentially creating it")

	// renewParticipationKeyCmd
	renewParticipationKeyCmd.Flags().StringVarP(&accountAddress, "address", "a", "", "Account address to update (required)")
	renewParticipationKeyCmd.MarkFlagRequired("address")
	renewParticipationKeyCmd.Flags().Uint64VarP(&transactionFee, "fee", "f", 0, "The Fee to set on the status change transaction (defaults to suggested fee)")
	renewParticipationKeyCmd.Flags().Uint64VarP(&roundLastValid, "roundLastValid", "", 0, "The last round for which the generated partkey will be valid")
	renewParticipationKeyCmd.MarkFlagRequired("roundLastValid")
	renewParticipationKeyCmd.Flags().Uint64VarP(&keyDilution, "keyDilution", "", 0, "Key dilution for two-level participation keys")
	renewParticipationKeyCmd.Flags().BoolVarP(&noWaitAfterSend, "no-wait", "N", false, "Don't wait for transaction to commit")

	// renewAllParticipationKeyCmd
	renewAllParticipationKeyCmd.Flags().Uint64VarP(&transactionFee, "fee", "f", 0, "The Fee to set on the status change transactions (defaults to suggested fee)")
	renewAllParticipationKeyCmd.Flags().Uint64VarP(&roundLastValid, "roundLastValid", "", 0, "The last round for which the generated partkeys will be valid")
	renewAllParticipationKeyCmd.MarkFlagRequired("roundLastValid")
	renewAllParticipationKeyCmd.Flags().Uint64VarP(&keyDilution, "keyDilution", "", 0, "Key dilution for two-level participation keys")
	renewAllParticipationKeyCmd.Flags().BoolVarP(&noWaitAfterSend, "no-wait", "N", false, "Don't wait for transaction to commit")
}

var accountCmd = &cobra.Command{
	Use:   "account",
	Short: "Control and manage Algorand accounts",
	Long:  `Collection of commands to support the creation and management of accounts / wallets tied to a specific Algorand node instance.`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		accountList := makeAccountsList(ensureSingleDataDir())

		// Update the default account
		if defaultAccountName != "" {
			// If the name doesn't exist, return an error
			if !accountList.isTaken(defaultAccountName) {
				reportErrorf(errorNameDoesntExist, defaultAccountName)
			}
			// Set the account with this name to be default
			accountList.setDefault(defaultAccountName)
			reportInfof(infoSetAccountToDefault, defaultAccountName)
			os.Exit(0)
		}

		// Return the help text
		cmd.HelpFunc()(cmd, args)
	},
}

var accountMultisigCmd = &cobra.Command{
	Use:   "multisig",
	Short: "Control and manage multisig accounts",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		// Return the help text
		cmd.HelpFunc()(cmd, args)
	},
}

var renameCmd = &cobra.Command{
	Use:   "rename [old name] [new name]",
	Short: "Change the human-friendly name of an account",
	Long:  `Change the human-friendly name of an account`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		accountList := makeAccountsList(ensureSingleDataDir())

		oldName := args[0]
		newName := args[1]

		// If not valid name, return an error
		if ok, err := isValidName(newName); !ok {
			reportErrorln(err)
		}

		// If the old name isn't in use, return an error
		if !accountList.isTaken(oldName) {
			reportErrorf(errorNameDoesntExist, oldName)
		}

		// If the new name isn't available, return an error
		if accountList.isTaken(newName) {
			reportErrorf(errorNameAlreadyTaken, newName)
		}

		// Otherwise, rename
		accountList.rename(oldName, newName)
		reportInfof(infoRenamedAccount, oldName, newName)
	},
}

var newCmd = &cobra.Command{
	Use:   "new",
	Short: "Create a new account",
	Long:  `Coordinates the creation of a new account with KMD. The name specified here is stored in a local configuration file and is only used by goal when working against that specific node instance.`,
	Args:  cobra.RangeArgs(0, 1),
	Run: func(cmd *cobra.Command, args []string) {
		accountList := makeAccountsList(ensureSingleDataDir())
		// Choose an account name
		if len(args) == 0 {
			accountName = accountList.getUnnamed()
		} else {
			accountName = args[0]
		}

		// If not valid name, return an error
		if ok, err := isValidName(accountName); !ok {
			reportErrorln(err)
		}

		// Ensure the user's name choice isn't taken
		if accountList.isTaken(accountName) {
			reportErrorf(errorNameAlreadyTaken, accountName)
		}

		dataDir := ensureSingleDataDir()

		// Get a wallet handle
		wh := ensureWalletHandle(dataDir, walletName)

		// Generate a new address in the default wallet
		client := ensureKmdClient(dataDir)
		genAddr, err := client.GenerateAddress(wh)
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		// Add account to list
		accountList.addAccount(accountName, genAddr)

		// Set account to default if required
		if defaultAccount {
			accountList.setDefault(accountName)
		}

		reportInfof(infoCreatedNewAccount, genAddr)
	},
}

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete an account",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()
		accountList := makeAccountsList(dataDir)

		client := ensureKmdClient(dataDir)
		wh, pw := ensureWalletHandleMaybePassword(dataDir, walletName, true)

		err := client.DeleteAccount(wh, pw, accountAddress)
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		accountList.removeAccount(accountAddress)
	},
}

var newMultisigCmd = &cobra.Command{
	Use:   "new [addr1 addr2 ...]",
	Short: "Create a new multisig account",
	Long:  `Create a new multisig account from a list of existing non-multisig addresses`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()
		accountList := makeAccountsList(dataDir)

		// Get a wallet handle to the default wallet
		client := ensureKmdClient(dataDir)

		// Get a wallet handle
		wh := ensureWalletHandle(dataDir, walletName)

		// Detect duplicate PKs
		duplicateDetector := make(map[string]int)
		for _, addrStr := range args {
			duplicateDetector[addrStr]++
		}
		duplicatesDetected := false
		for _, counter := range duplicateDetector {
			if counter > 1 {
				duplicatesDetected = true
				break
			}
		}
		if duplicatesDetected {
			reportWarnln(warnMultisigDuplicatesDetected)
		}
		// Generate a new address in the default wallet
		addr, err := client.CreateMultisigAccount(wh, threshold, args)
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		// Add account to list
		accountList.addAccount(accountList.getUnnamed(), addr)

		reportInfof(infoCreatedNewAccount, addr)
	},
}

var deleteMultisigCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a multisig account",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()
		accountList := makeAccountsList(dataDir)

		client := ensureKmdClient(dataDir)
		wh, pw := ensureWalletHandleMaybePassword(dataDir, walletName, true)

		err := client.DeleteMultisigAccount(wh, pw, accountAddress)
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		accountList.removeAccount(accountAddress)
	},
}

var infoMultisigCmd = &cobra.Command{
	Use:   "info",
	Short: "Print information about a multisig account",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()
		client := ensureKmdClient(dataDir)
		wh := ensureWalletHandle(dataDir, walletName)

		multisigInfo, err := client.LookupMultisigAccount(wh, accountAddress)
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		fmt.Printf("Version: %d\n", multisigInfo.Version)
		fmt.Printf("Threshold: %d\n", multisigInfo.Threshold)
		fmt.Printf("Public keys:\n")
		for _, pk := range multisigInfo.PKs {
			fmt.Printf("  %s\n", pk)
		}
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "Show the list of Algorand accounts on this machine",
	Long:  `Show the list of Algorand accounts on this machine. Also indicates whether the account is [offline] or [online], and if the account is the default account for goal.`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()
		accountList := makeAccountsList(dataDir)

		// Get a wallet handle to the specified wallet
		wh := ensureWalletHandle(dataDir, walletName)

		// List the addresses in the wallet
		client := ensureKmdClient(dataDir)
		addrs, err := client.ListAddressesWithInfo(wh)
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		// Special response if there are no addresses
		if len(addrs) == 0 {
			reportInfoln(infoNoAccounts)
			os.Exit(0)
		}

		// For each address, request information about it from algod
		for _, addr := range addrs {
			response, _ := client.AccountInformation(addr.Addr)
			// it's okay to procede with out algod info

			// Display this information to the user
			if addr.Multisig {
				multisigInfo, err := client.LookupMultisigAccount(wh, addr.Addr)
				if err != nil {
					fmt.Println("multisig lookup err")
					reportErrorf(errorRequestFail, err)
				}

				accountList.outputAccount(addr.Addr, response, &multisigInfo)
			} else {
				accountList.outputAccount(addr.Addr, response, nil)
			}
		}
	},
}

var balanceCmd = &cobra.Command{
	Use:   "balance",
	Short: "Retrieve the balance for the specified account, in microAlgos",
	Long:  `Retrieve the balance for the specified account, in microAlgos`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()
		client := ensureAlgodClient(dataDir)
		response, err := client.AccountInformation(accountAddress)
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		fmt.Printf("%v microAlgos\n", response.Amount)
	},
}

var rewardsCmd = &cobra.Command{
	Use:   "rewards",
	Short: "Retrieve the rewards for the specified account",
	Long:  `Retrieve the rewards for the specified account`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()
		client := ensureAlgodClient(dataDir)
		response, err := client.AccountInformation(accountAddress)
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		fmt.Printf("%v microAlgos\n", response.Rewards)
	},
}

var changeOnlineCmd = &cobra.Command{
	Use:   "changeonlinestatus",
	Short: "Change online status for the specified account",
	Long:  `Change online status for the specified account. Set online should be 1 to set online, 0 to set offline. The broadcast transaction will be valid for a limited number of rounds. goal will provide the TXID of the transaction if successful. Going online requires that the given account have a valid participation key.`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		// Pull the current round for use in our new transactions
		dataDir := ensureSingleDataDir()
		client := ensureFullClient(dataDir)

		err := changeAccountOnlineStatus(accountAddress, nil, online, onlineTxFile, walletName, onlineFirstRound, onlineValidRounds, transactionFee, dataDir, client)
		if err != nil {
			reportErrorf(err.Error())
		}
	},
}

func changeAccountOnlineStatus(acct string, part *algodAcct.Participation, goOnline bool, txFile string, wallet string, firstTxRound, validTxRounds, fee uint64, dataDir string, client libgoal.Client) error {
	// Generate an unsigned online/offline tx
	var utx transactions.Transaction
	var err error
	if goOnline {
		utx, err = client.MakeUnsignedGoOnlineTx(acct, part, firstTxRound, validTxRounds, fee)
	} else {
		utx, err = client.MakeUnsignedGoOfflineTx(acct, firstTxRound, validTxRounds, fee)
	}
	if err != nil {
		return err
	}

	if txFile == "" {
		// Sign & broadcast the transaction
		wh, pw := ensureWalletHandleMaybePassword(dataDir, wallet, true)
		txid, err := client.SignAndBroadcastTransaction(wh, pw, utx)
		if err != nil {
			return fmt.Errorf(errorOnlineTX, err)
		}
		fmt.Printf("Transaction id for status change transaction: %s\n", txid)

		if noWaitAfterSend {
			fmt.Println("Note: status will not change until transaction is finalized")
			return nil
		}

		// Get current round information
		stat, err := client.Status()
		if err != nil {
			return fmt.Errorf(errorRequestFail, err)
		}

		for {
			// Check if we know about the transaction yet
			txn, err := client.PendingTransactionInformation(txid)
			if err != nil {
				return fmt.Errorf(errorRequestFail, err)
			}

			if txn.ConfirmedRound > 0 {
				reportInfof(infoTxCommitted, txid, txn.ConfirmedRound)
				break
			}

			if txn.PoolError != "" {
				return fmt.Errorf(txPoolError, txid, txn.PoolError)
			}

			reportInfof(infoTxPending, txid, stat.LastRound)
			stat, err = client.WaitForRound(stat.LastRound + 1)
			if err != nil {
				return fmt.Errorf(errorRequestFail, err)
			}
		}
	} else {
		// Wrap in a transactions.SignedTxn with an empty sig.
		// This way protocol.Encode will encode the transaction type
		stxn, err := transactions.AssembleSignedTxn(utx, crypto.Signature{}, crypto.MultisigSig{})
		if err != nil {
			return fmt.Errorf(errorConstructingTX, err)
		}

		stxn = populateBlankMultisig(client, dataDir, wallet, stxn)

		// Write the SignedTxn to the output file
		err = ioutil.WriteFile(txFile, protocol.Encode(stxn), 0600)
		if err != nil {
			return fmt.Errorf(fileWriteError, txFile, err)
		}
	}
	return nil
}

var addParticipationKeyCmd = &cobra.Command{
	Use:   "addpartkey",
	Short: "Generate a participation key for the specified account",
	Long:  `Generate a participation key for the specified account`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()

		if partKeyOutDir != "" && !util.IsDir(partKeyOutDir) {
			reportErrorf(errorDirectoryNotExist, partKeyOutDir)
		}

		// Generate a participation keys database and install it
		client := ensureFullClient(dataDir)

		_, _, err := client.GenParticipationKeysTo(accountAddress, roundFirstValid, roundLastValid, keyDilution, partKeyOutDir)
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}
		fmt.Println("Participation key generation successful")
	},
}

var renewParticipationKeyCmd = &cobra.Command{
	Use:   "renewpartkey",
	Short: "Renew an account's participation key",
	Long:  `Generate a participation key for the specified account and register it`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()

		client := ensureAlgodClient(dataDir)

		currentRound, err := client.CurrentRound()
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		params, err := client.SuggestedParams()
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}
		proto := config.Consensus[protocol.ConsensusVersion(params.ConsensusVersion)]

		if roundLastValid <= (currentRound + proto.MaxTxnLife) {
			reportErrorf(errLastRoundInvalid, currentRound)
		}

		// Make sure we don't already have a partkey valid for (or after) specified roundLastValid
		parts, err := client.ListParticipationKeys()
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}
		for _, part := range parts {
			if part.Address().GetChecksumAddress().String() == accountAddress {
				if part.LastValid >= basics.Round(roundLastValid) {
					reportErrorf(errExistingPartKey, roundLastValid, part.LastValid)
				}
			}
		}

		err = generateAndRegisterPartKey(accountAddress, currentRound, roundLastValid, proto.MaxTxnLife, transactionFee, keyDilution, walletName, dataDir, client)
		if err != nil {
			reportErrorf(err.Error())
		}
	},
}

func generateAndRegisterPartKey(address string, currentRound, lastValidRound, maxTxnLife uint64, fee, dilution uint64, wallet string, dataDir string, client libgoal.Client) error {
	// Generate a participation keys database and install it
	part, keyPath, err := client.GenParticipationKeysTo(address, currentRound, lastValidRound, dilution, "")
	if err != nil {
		return fmt.Errorf(errorRequestFail, err)
	}
	fmt.Printf("  Generated participation key for %s (Valid %d - %d)\n", address, currentRound, lastValidRound)

	// Now register it as our new online participation key
	goOnline := true
	txFile := ""
	err = changeAccountOnlineStatus(address, &part, goOnline, txFile, wallet, currentRound, maxTxnLife, fee, dataDir, client)
	if err != nil {
		part.Close()
		os.Remove(keyPath)
		fmt.Fprintf(os.Stderr, "  Error registering keys - deleting newly-generated key file: %s\n", keyPath)
	}
	return err
}

var renewAllParticipationKeyCmd = &cobra.Command{
	Use:   "renewallpartkeys",
	Short: "Renew all existing participation keys",
	Long:  `Generate new participation keys for all existing accounts with participation keys and register them`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {

		onDataDirs(func(dataDir string) {
			fmt.Printf("Renewing participation keys in %s...\n", dataDir)
			err := renewPartKeysInDir(dataDir, roundLastValid, transactionFee, keyDilution, walletName)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  Error: %s\n", err)
			}
		})
	},
}

func renewPartKeysInDir(dataDir string, lastValidRound uint64, fee uint64, dilution uint64, wallet string) error {
	client := ensureAlgodClient(dataDir)

	// Build list of accounts to renew from all accounts with part keys present
	parts, err := client.ListParticipationKeys()
	if err != nil {
		return fmt.Errorf(errorRequestFail, err)
	}
	renewAccounts := make(map[basics.Address]algodAcct.Participation)
	for _, part := range parts {
		if existing, has := renewAccounts[part.Address()]; has {
			if existing.LastValid >= part.LastValid {
				// We already saw a partkey that expires later
				continue
			}
		}
		renewAccounts[part.Address()] = part
	}

	currentRound, err := client.CurrentRound()
	if err != nil {
		return fmt.Errorf(errorRequestFail, err)
	}

	params, err := client.SuggestedParams()
	if err != nil {
		return fmt.Errorf(errorRequestFail, err)
	}
	proto := config.Consensus[protocol.ConsensusVersion(params.ConsensusVersion)]

	if lastValidRound <= (currentRound + proto.MaxTxnLife) {
		return fmt.Errorf(errLastRoundInvalid, currentRound)
	}

	var anyErrors bool

	// Now go through each account and if it doesn't have a part key that's valid
	// at least through lastValidRound, generate a new key and register it.
	// Make sure we don't already have a partkey valid for (or after) specified roundLastValid
	for _, renewPart := range renewAccounts {
		if renewPart.LastValid >= basics.Round(lastValidRound) {
			fmt.Printf("  Skipping account %s: Already has a part key valid beyond %d (currently %d)\n", renewPart.Address().GetChecksumAddress(), lastValidRound, renewPart.LastValid)
			continue
		}

		// If the account's latest partkey expired before the current round, don't automatically renew and instead instruct the user to explicitly renew it.
		if renewPart.LastValid < basics.Round(lastValidRound) {
			fmt.Printf("  Skipping account %s: This account has part keys that have expired.  Please renew this account explicitly using 'renewpartkey'\n", renewPart.Address().GetChecksumAddress())
			continue
		}

		address := renewPart.Address().GetChecksumAddress().String()
		err = generateAndRegisterPartKey(address, currentRound, lastValidRound, proto.MaxTxnLife, fee, dilution, wallet, dataDir, client)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  Error renewing part key for account %s: %v\n", address, err)
			anyErrors = true
		}
	}
	if anyErrors {
		return fmt.Errorf("one or more renewal attempts had errors")
	}
	return nil
}

var listParticipationKeysCmd = &cobra.Command{
	Use:   "listpartkeys",
	Short: "List participation keys",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()

		client := ensureGoalClient(dataDir, libgoal.DynamicClient)
		parts, err := client.ListParticipationKeys()
		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		var filenames []string
		for fn := range parts {
			filenames = append(filenames, fn)
		}
		sort.Strings(filenames)

		rowFormat := "%-80s\t%-60s\t%12s\t%12s\t%12s\n"
		fmt.Printf(rowFormat, "Filename", "Parent address", "First round", "Last round", "First key")
		for _, fn := range filenames {
			first, last := parts[fn].ValidInterval()
			fmt.Printf(rowFormat, fn, parts[fn].Address().GetUserAddress(),
				fmt.Sprintf("%d", first),
				fmt.Sprintf("%d", last),
				fmt.Sprintf("%d.%d", parts[fn].Voting.FirstBatch, parts[fn].Voting.FirstOffset))
		}
	},
}

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import an account key from mnemonic",
	Long:  "Import an account key from a mnemonic generated by the export command or by algokey (NOT a mnemonic from the goal wallet command). The imported account will be listed alongside your wallet-generated accounts, but will not be tied to your wallet.",
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()
		accountList := makeAccountsList(dataDir)
		// Choose an account name
		if len(args) == 0 {
			accountName = accountList.getUnnamed()
		} else {
			accountName = args[0]
		}

		// If not valid name, return an error
		if ok, err := isValidName(accountName); !ok {
			reportErrorln(err)
		}

		// Ensure the user's name choice isn't taken
		if accountList.isTaken(accountName) {
			reportErrorf(errorNameAlreadyTaken, accountName)
		}

		client := ensureKmdClient(dataDir)
		wh := ensureWalletHandle(dataDir, walletName)
		//wh, pw := ensureWalletHandleMaybePassword(dataDir, walletName, true)

		if mnemonic == "" {
			fmt.Println(infoRecoveryPrompt)
			reader := bufio.NewReader(os.Stdin)
			resp, err := reader.ReadString('\n')
			resp = strings.TrimSpace(resp)
			if err != nil {
				reportErrorf(errorFailedToReadResponse, err)
			}
			mnemonic = resp
		}
		var key []byte
		key, err := passphrase.MnemonicToKey(mnemonic)
		if err != nil {
			reportErrorf(errorBadMnemonic, err)
		}

		importedKey, err := client.ImportKey(wh, key)
		if err != nil {
			reportErrorf(errorRequestFail, err)
		} else {
			reportInfof(infoImportedKey, importedKey.Address)

			accountList.addAccount(accountName, importedKey.Address)
			if importDefault {
				accountList.setDefault(accountName)
			}
		}
	},
}

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export an account key for use with account import",
	Long:  "Export an account mnemonic seed, for use with account import. This exports the seed for a single account and should not be confused with the wallet mnemonic.",
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()
		client := ensureKmdClient(dataDir)

		wh, pw := ensureWalletHandleMaybePassword(dataDir, walletName, true)
		passwordString := string(pw)

		response, err := client.ExportKey(wh, passwordString, accountAddress)

		if err != nil {
			reportErrorf(errorRequestFail, err)
		}

		seed, err := crypto.SecretKeyToSeed(response.PrivateKey)

		if err != nil {
			reportErrorf(errorSeedConversion, accountAddress, err)
		}

		privKeyAsMnemonic, err := passphrase.KeyToMnemonic(seed[:])

		if err != nil {
			reportErrorf(errorMnemonicConversion, accountAddress, err)
		}

		reportInfof(infoExportedKey, accountAddress, privKeyAsMnemonic)
	},
}

var importRootKeysCmd = &cobra.Command{
	Use:   "importrootkey",
	Short: "Import .rootkey files from the data directory into a kmd wallet",
	Long:  "Import .rootkey files from the data directory into a kmd wallet. This is analogous to using the import command with an account seed mnemonic: the imported account will be displayed alongside your wallet-derived accounts, but will not be tied to your wallet mnemonic.",
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {
		dataDir := ensureSingleDataDir()
		// Generate a participation keys database and install it
		client := ensureKmdClient(dataDir)

		genID, err := client.GenesisID()
		if err != nil {
			return
		}

		keyDir := filepath.Join(dataDir, genID)
		files, err := ioutil.ReadDir(keyDir)
		if err != nil {
			return
		}

		// For each of these files
		cnt := 0
		for _, info := range files {
			var handle db.Accessor

			// If it can't be a participation key database, skip it
			if !config.IsRootKeyFilename(info.Name()) {
				continue
			}

			filename := info.Name()

			// Fetch a handle to this database
			handle, err = db.MakeErasableAccessor(filepath.Join(keyDir, filename))
			if err != nil {
				// Couldn't open it, skip it
				err = nil
				continue
			}

			// Fetch an account.Participation from the database
			root, err := algodAcct.RestoreRoot(handle)
			if err != nil {
				// Couldn't read it, skip it
				err = nil
				continue
			}

			secretKey := root.Secrets().SK

			// Determine which wallet to import into
			var wh []byte
			if unencryptedWallet {
				wh, err = client.GetUnencryptedWalletHandle()
				if err != nil {
					reportErrorf(errorRequestFail, err)
				}
			} else {
				wh = ensureWalletHandle(dataDir, walletName)
			}

			resp, err := client.ImportKey(wh, secretKey[:])
			if err != nil {
				// If error is 'like' "key already exists", treat as warning and not an error
				if strings.Contains(err.Error(), "key already exists") {
					reportWarnf(errorRequestFail, err.Error()+"\n > Key File: "+filename)
				} else {
					reportErrorf(errorRequestFail, err)
				}
			} else {
				// Count the number of keys imported
				cnt++
				reportInfof(infoImportedKey, resp.Address)
			}
		}

		// Provide feedback on how many keys were imported
		plural := "s"
		if cnt == 1 {
			plural = ""
		}
		reportInfof(infoImportedNKeys, cnt, plural)
	},
}

type partkeyInfo struct {
	_struct         struct{}                        `codec:",omitempty,omitemptyarray"`
	Address         string                          `codec:"acct"`
	FirstValid      basics.Round                    `codec:"first"`
	LastValid       basics.Round                    `codec:"last"`
	VoteID          crypto.OneTimeSignatureVerifier `codec:"vote"`
	SelectionID     crypto.VRFVerifier              `codec:"sel"`
	VoteKeyDilution uint64                          `codec:"voteKD"`
}

var partkeyInfoCmd = &cobra.Command{
	Use:   "partkeyinfo",
	Short: "Output details about all available part keys",
	Long:  `Output details about all available part keys in the specified data directory(ies)`,
	Args:  validateNoPosArgsFn,
	Run: func(cmd *cobra.Command, args []string) {

		onDataDirs(func(dataDir string) {
			fmt.Printf("Dumping participation key info from %s...\n", dataDir)
			client := ensureGoalClient(dataDir, libgoal.DynamicClient)

			// Make sure we don't already have a partkey valid for (or after) specified roundLastValid
			parts, err := client.ListParticipationKeys()
			if err != nil {
				reportErrorf(errorRequestFail, err)
			}

			for filename, part := range parts {
				fmt.Println("------------------------------------------------------------------")
				info := partkeyInfo{
					Address:         part.Address().GetChecksumAddress().String(),
					FirstValid:      part.FirstValid,
					LastValid:       part.LastValid,
					VoteID:          part.VotingSecrets().OneTimeSignatureVerifier,
					SelectionID:     part.VRFSecrets().PK,
					VoteKeyDilution: part.KeyDilution,
				}
				infoString := protocol.EncodeJSON(&info)
				fmt.Printf("File: %s\n%s\n", filename, string(infoString))
			}
		})
	},
}
