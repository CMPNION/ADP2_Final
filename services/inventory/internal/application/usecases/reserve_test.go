package usecases

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type testLocker struct {
	lockOK  bool
	lockErr error
}

func (l testLocker) Lock(_ context.Context, _ string, _ time.Duration) (bool, error) {
	return l.lockOK, l.lockErr
}
func (l testLocker) Unlock(_ context.Context, _ string) error { return nil }

func TestReserveStock_EmptyItems(t *testing.T) {
	svc := NewReserveService(nil, nil, nil, nil)
	res, err := svc.ReserveStock(context.Background(), "order-1", nil)
	if err == nil {
		t.Fatalf("expected error for empty items")
	}
	if res != nil {
		t.Fatalf("expected nil result for empty items, got %+v", res)
	}
}

func TestReserveStock_LockFailure(t *testing.T) {
	svc := NewReserveService(nil, testLocker{lockOK: false, lockErr: errors.New("busy")}, nil, nil)
	_, err := svc.ReserveStock(context.Background(), "order-1", []ItemReq{{SKU: "SKU-1", Quantity: 1}})
	if err == nil || !strings.Contains(err.Error(), "failed to lock stock") {
		t.Fatalf("expected lock failure error, got %v", err)
	}
}

func TestReleaseAndConfirm_LockFailure(t *testing.T) {
	locker := testLocker{lockOK: false, lockErr: errors.New("busy")}
	release := NewReleaseService(nil, locker, nil, nil)
	confirm := NewConfirmService(nil, locker, nil, nil)

	if err := release.ReleaseStock(context.Background(), "order-1"); err == nil || !strings.Contains(err.Error(), "failed to lock order") {
		t.Fatalf("expected release lock failure, got %v", err)
	}
	if err := confirm.ConfirmStockDeduction(context.Background(), "order-1"); err == nil || !strings.Contains(err.Error(), "failed to lock order") {
		t.Fatalf("expected confirm lock failure, got %v", err)
	}
}

func TestAddStockReceipt_Validation(t *testing.T) {
	svc := NewReserveService(nil, nil, nil, nil)
	if err := svc.AddStockReceipt(context.Background(), "", "wh-1", 1, "r-1"); err == nil {
		t.Fatalf("expected sku validation error")
	}
	if err := svc.AddStockReceipt(context.Background(), "SKU-1", "wh-1", 0, "r-1"); err == nil {
		t.Fatalf("expected quantity validation error")
	}
}

func TestTransferStock_Validation(t *testing.T) {
	svc := NewReserveService(nil, nil, nil, nil)
	if err := svc.TransferStock(context.Background(), "SKU-1", "wh-1", "wh-1", 1, "ref-1"); err == nil {
		t.Fatalf("expected same-warehouse validation error")
	}
	if err := svc.TransferStock(context.Background(), "SKU-1", "wh-1", "wh-2", 0, "ref-1"); err == nil {
		t.Fatalf("expected quantity validation error")
	}
}
