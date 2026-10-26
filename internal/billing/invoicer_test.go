package billing

import (
	"context"
	"errors"
	"testing"
	"time"
)

type noCustomers struct{}

func (noCustomers) Lookup(string) *Customer { return nil }

func TestChargeUnknownCustomer(t *testing.T) {
	inv := NewInvoicer(noCustomers{}, nil, time.Now)
	order := Order{ID: "o1", CustomerID: "guest", Items: []Item{{SKU: "a", Quantity: 1, UnitPrice: 100}}}
	_, err := inv.Charge(context.Background(), order)
	if !errors.Is(err, ErrUnknownCustomer) {
		t.Fatalf("err = %v, want ErrUnknownCustomer", err)
	}
}
