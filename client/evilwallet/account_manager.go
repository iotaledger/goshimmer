package evilwallet

type AccountType int8

const (
	baseAccount AccountType = iota
	conflictAccount
	deepAccount
)

type Account struct {
	accType AccountType
}

type AccountManager struct {
	accounts map[AccountType][]Account
}
