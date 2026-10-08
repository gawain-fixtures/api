// Package handlers holds the HTTP handlers of the API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"example.com/api/internal/billing"
)

type Invoicer interface {
	Charge(ctx context.Context, order billing.Order) (billing.Invoice, error)
}

type Carts interface {
	Get(ctx context.Context, id string) (Cart, error)
}

type Cart struct {
	ID         string
	CustomerID string
	Coupon     string
	Lines      []CartLine
}

type CartLine struct {
	SKU        string `json:"sku"`
	Quantity   int    `json:"quantity"`
	PriceCents int64  `json:"price_cents"`
}

var ErrCartNotFound = errors.New("handlers: cart not found")

type CheckoutHandler struct {
	carts    Carts
	invoicer Invoicer
	now      func() time.Time
}

func NewCheckoutHandler(carts Carts, invoicer Invoicer, now func() time.Time) *CheckoutHandler {
	return &CheckoutHandler{carts: carts, invoicer: invoicer, now: now}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("checkout: write response: %v", err)
	}
}

func orderFromCart(cart Cart, placedAt time.Time) billing.Order {
	items := make([]billing.Item, 0, len(cart.Lines))
	for _, line := range cart.Lines {
		items = append(items, billing.Item{
			SKU:       line.SKU,
			Quantity:  line.Quantity,
			UnitPrice: billing.Money(line.PriceCents),
		})
	}
	return billing.Order{ID: "o_" + cart.ID, CustomerID: cart.CustomerID, Items: items, Coupon: cart.Coupon, PlacedAt: placedAt}
}

// Post charges the cart named by the "cart" query parameter.
func (h *CheckoutHandler) Post(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	cartID := strings.TrimSpace(r.URL.Query().Get("cart"))
	if cartID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "cart is required"})
		return
	}
	cart, err := h.carts.Get(ctx, cartID)
	if errors.Is(err, ErrCartNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown cart"})
		return
	}
	if err != nil {
		log.Printf("checkout: load cart %s: %v", cartID, err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "cart unavailable"})
		return
	}
	order := orderFromCart(cart, h.now())
	invoice, err := h.invoicer.Charge(ctx, order)
	if err != nil {
		log.Printf("checkout: charge order %s: %v", order.ID, err)
		writeJSON(w, http.StatusPaymentRequired, map[string]string{"error": "payment failed"})
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"invoice": invoice.Number, "total_cents": int64(invoice.Total)})
}
