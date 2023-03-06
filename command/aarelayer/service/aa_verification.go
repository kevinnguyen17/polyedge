package service

import (
	"errors"
	"fmt"

	"github.com/0xPolygon/polygon-edge/types"
)

type ValidationFunc func(*AATransaction) bool

type AAVerification interface {
	Validate(*AATransaction) error
}

var _ AAVerification = (*aaVerification)(nil)

type aaVerification struct {
	validationFn   ValidationFunc
	config         *AAConfig
	chainID        int64
	invokerAddress types.Address
}

func NewAAVerification(
	config *AAConfig, invokerAddress types.Address, chainID int64, validationFn ValidationFunc) *aaVerification {
	return &aaVerification{
		validationFn:   validationFn,
		config:         config,
		chainID:        chainID,
		invokerAddress: invokerAddress,
	}
}

func (p *aaVerification) Validate(tx *AATransaction) error {
	if tx == nil {
		return errors.New("tx is not valid")
	}

	if len(tx.Transaction.Payload) == 0 {
		return fmt.Errorf("tx from %s does not have payload", tx.Transaction.From)
	}

	for _, pl := range tx.Transaction.Payload {
		if !p.config.IsValidAddress(pl.To) {
			return fmt.Errorf("tx has invalid payload: %s", tx.Transaction.From.String())
		}
	}

	// TODO: full validation will be implemented in another PR/task
	if !tx.Transaction.IsFromValid(p.invokerAddress, p.chainID, tx.Signature) {
		return fmt.Errorf("tx has invalid from: %s", tx.Transaction.From.String())
	}

	return nil
}
