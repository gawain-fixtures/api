// Package billing prices orders and charges customers for them.
package billing

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Money is an amount in the smallest unit of a currency (cents).
type Money int64

// Item is one line of an order.
type Item struct {
	SKU       string
	Quantity  int
	UnitPrice Money
}

// Order is what a customer is buying.
type Order struct {
	ID         string
	CustomerID string
	Items      []Item
	Coupon     string
	PlacedAt   time.Time
}

// Profile holds the billing details kept for a customer.
type Profile struct {
	Name         string
	Email        string
	Country      string
	TaxExempt    bool
	PaymentToken string
}

// Customer is a billing account. Guest checkouts have none.
type Customer struct {
	ID      string
	Profile Profile
}

type Customers interface {
	// Lookup returns nil when the customer is unknown.
	Lookup(id string) *Customer
}

// Gateway moves money.
type Gateway interface {
	Authorize(ctx context.Context, token string, amount Money) (string, error)
	Capture(ctx context.Context, authorization string) error
}

// Invoice is the result of a successful charge.
type Invoice struct {
	OrderID       string
	Number        string
	Subtotal      Money
	Discount      Money
	Tax           Money
	Total         Money
	Authorization string
	IssuedAt      time.Time
}

var (
	// ErrEmptyOrder is returned for an order without items.
	ErrEmptyOrder = errors.New("billing: order has no items")
	// ErrBadQuantity is returned for an item with a quantity below one.
	ErrBadQuantity = errors.New("billing: item quantity must be positive")
)

// taxRates are percentages by country code. Unlisted countries pay no tax.
var taxRates = map[string]int64{
	"US": 7,
	"DE": 19,
	"FR": 20,
	"GB": 20,
}

// coupons are percentage discounts by coupon code.
var coupons = map[string]int64{
	"WELCOME10": 10,
	"VIP25":     25,
}

// Invoicer prices orders and charges them through a payment gateway.
type Invoicer struct {
	customers Customers
	gateway   Gateway
	clock     func() time.Time
	sequence  int
}

// NewInvoicer returns an Invoicer that reads time from clock.
func NewInvoicer(customers Customers, gateway Gateway, clock func() time.Time) *Invoicer {
	return &Invoicer{customers: customers, gateway: gateway, clock: clock}
}

// subtotal adds up the order's lines.
func subtotal(order Order) (Money, error) {
	if len(order.Items) == 0 {
		return 0, ErrEmptyOrder
	}
	var total Money
	for _, item := range order.Items {
		if item.Quantity < 1 {
			return 0, fmt.Errorf("%w: %s", ErrBadQuantity, item.SKU)
		}
		total += item.UnitPrice * Money(item.Quantity)
	}
	return total, nil
}

// discount applies the order's coupon, if it is a known one, to the subtotal.
func discount(order Order, sub Money) Money {
	percent, ok := coupons[strings.ToUpper(order.Coupon)]
	if !ok {
		return 0
	}
	return sub * Money(percent) / 100
}

// tax is charged on the discounted amount unless the customer is exempt.
func tax(profile Profile, amount Money) Money {
	if profile.TaxExempt {
		return 0
	}
	return amount * Money(taxRates[profile.Country]) / 100
}

// Charge prices the order and charges the customer's stored payment method.
func (i *Invoicer) Charge(ctx context.Context, order Order) (Invoice, error) {
	sub, err := subtotal(order)
	if err != nil {
		return Invoice{}, err
	}
	customer := i.customers.Lookup(order.CustomerID).Profile
	off := discount(order, sub)
	owed := tax(customer, sub-off)
	total := sub - off + owed

	authorization, err := i.gateway.Authorize(ctx, customer.PaymentToken, total)
	if err != nil {
		return Invoice{}, fmt.Errorf("billing: authorize order %s: %w", order.ID, err)
	}
	if err := i.gateway.Capture(ctx, authorization); err != nil {
		return Invoice{}, fmt.Errorf("billing: capture order %s: %w", order.ID, err)
	}

	i.sequence++
	return Invoice{
		OrderID:       order.ID,
		Number:        fmt.Sprintf("INV-%06d", i.sequence),
		Subtotal:      sub,
		Discount:      off,
		Tax:           owed,
		Total:         total,
		Authorization: authorization,
		IssuedAt:      i.clock(),
	}, nil
}
