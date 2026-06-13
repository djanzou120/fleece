package domain

import (
	"errors"
	"testing"
)

// --- Tests Money ---

func TestNewMoney_PositiveAmount(t *testing.T) {
	m, err := NewMoney(1000, "XAF")
	if err != nil {
		t.Fatalf("NewMoney inattendu: %v", err)
	}
	if m.Amount != 1000 {
		t.Errorf("Amount attendu 1000, obtenu %d", m.Amount)
	}
	if m.Currency != "XAF" {
		t.Errorf("Currency attendu XAF, obtenu %s", m.Currency)
	}
}

func TestNewMoney_ZeroAmount(t *testing.T) {
	m, err := NewMoney(0, "XAF")
	if err != nil {
		t.Fatalf("NewMoney(0) inattendu: %v", err)
	}
	if m.Amount != 0 {
		t.Errorf("Amount attendu 0, obtenu %d", m.Amount)
	}
}

func TestNewMoney_NegativeAmount(t *testing.T) {
	_, err := NewMoney(-1, "XAF")
	if err == nil {
		t.Fatal("attendu une erreur pour montant negatif")
	}
	if !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("attendu ErrInvalidAmount, obtenu %v", err)
	}
}

func TestMoney_Add_SameCurrency(t *testing.T) {
	a := Money{Amount: 500, Currency: "XAF"}
	b := Money{Amount: 300, Currency: "XAF"}
	result, err := a.Add(b)
	if err != nil {
		t.Fatalf("Add inattendu: %v", err)
	}
	if result.Amount != 800 {
		t.Errorf("Amount attendu 800, obtenu %d", result.Amount)
	}
	if result.Currency != "XAF" {
		t.Errorf("Currency attendu XAF, obtenu %s", result.Currency)
	}
}

func TestMoney_Add_MismatchCurrency(t *testing.T) {
	a := Money{Amount: 500, Currency: "XAF"}
	b := Money{Amount: 300, Currency: "EUR"}
	_, err := a.Add(b)
	if err == nil {
		t.Fatal("attendu une erreur pour devises differentes")
	}
	if !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("attendu ErrInvalidAmount, obtenu %v", err)
	}
}

func TestMoney_Sub_SameCurrency(t *testing.T) {
	a := Money{Amount: 500, Currency: "XAF"}
	b := Money{Amount: 200, Currency: "XAF"}
	result, err := a.Sub(b)
	if err != nil {
		t.Fatalf("Sub inattendu: %v", err)
	}
	if result.Amount != 300 {
		t.Errorf("Amount attendu 300, obtenu %d", result.Amount)
	}
}

func TestMoney_Sub_MismatchCurrency(t *testing.T) {
	a := Money{Amount: 500, Currency: "XAF"}
	b := Money{Amount: 200, Currency: "EUR"}
	_, err := a.Sub(b)
	if err == nil {
		t.Fatal("attendu une erreur pour devises differentes")
	}
	if !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("attendu ErrInvalidAmount, obtenu %v", err)
	}
}

func TestMoney_IsNegative(t *testing.T) {
	pos := Money{Amount: 1, Currency: "XAF"}
	if pos.IsNegative() {
		t.Error("1 XAF ne devrait pas etre negatif")
	}

	zero := Money{Amount: 0, Currency: "XAF"}
	if zero.IsNegative() {
		t.Error("0 XAF ne devrait pas etre negatif")
	}

	neg := Money{Amount: -1, Currency: "XAF"}
	if !neg.IsNegative() {
		t.Error("-1 XAF devrait etre negatif")
	}
}

// --- Tests Wallet ---

func TestNewWallet(t *testing.T) {
	w := NewWallet("ws-1", "XAF")
	if w.WorkspaceID != "ws-1" {
		t.Errorf("WorkspaceID attendu ws-1, obtenu %s", w.WorkspaceID)
	}
	if w.Balance.Amount != 0 {
		t.Errorf("solde initial attendu 0, obtenu %d", w.Balance.Amount)
	}
	if w.Balance.Currency != "XAF" {
		t.Errorf("devise attendue XAF, obtenu %s", w.Balance.Currency)
	}
}

func TestWallet_Debit_Success(t *testing.T) {
	w := &Wallet{WorkspaceID: "ws-1", Balance: Money{Amount: 1000, Currency: "XAF"}}
	amount := Money{Amount: 300, Currency: "XAF"}
	if err := w.Debit(amount); err != nil {
		t.Fatalf("Debit inattendu: %v", err)
	}
	if w.Balance.Amount != 700 {
		t.Errorf("solde attendu 700, obtenu %d", w.Balance.Amount)
	}
}

func TestWallet_Debit_ExactBalance(t *testing.T) {
	w := &Wallet{WorkspaceID: "ws-1", Balance: Money{Amount: 500, Currency: "XAF"}}
	amount := Money{Amount: 500, Currency: "XAF"}
	if err := w.Debit(amount); err != nil {
		t.Fatalf("Debit exact inattendu: %v", err)
	}
	if w.Balance.Amount != 0 {
		t.Errorf("solde attendu 0, obtenu %d", w.Balance.Amount)
	}
}

func TestWallet_Debit_InsufficientFunds(t *testing.T) {
	w := &Wallet{WorkspaceID: "ws-1", Balance: Money{Amount: 100, Currency: "XAF"}}
	amount := Money{Amount: 200, Currency: "XAF"}
	err := w.Debit(amount)
	if err == nil {
		t.Fatal("attendu ErrInsufficientFunds")
	}
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Errorf("attendu ErrInsufficientFunds, obtenu %v", err)
	}
	// Le solde ne doit pas etre modifie.
	if w.Balance.Amount != 100 {
		t.Errorf("solde ne devrait pas avoir change, obtenu %d", w.Balance.Amount)
	}
}

func TestWallet_Debit_MismatchCurrency(t *testing.T) {
	w := &Wallet{WorkspaceID: "ws-1", Balance: Money{Amount: 1000, Currency: "XAF"}}
	amount := Money{Amount: 100, Currency: "EUR"}
	err := w.Debit(amount)
	if err == nil {
		t.Fatal("attendu une erreur pour devises differentes")
	}
	if !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("attendu ErrInvalidAmount, obtenu %v", err)
	}
}

func TestWallet_Credit_Success(t *testing.T) {
	w := &Wallet{WorkspaceID: "ws-1", Balance: Money{Amount: 500, Currency: "XAF"}}
	amount := Money{Amount: 200, Currency: "XAF"}
	if err := w.Credit(amount); err != nil {
		t.Fatalf("Credit inattendu: %v", err)
	}
	if w.Balance.Amount != 700 {
		t.Errorf("solde attendu 700, obtenu %d", w.Balance.Amount)
	}
}

func TestWallet_Credit_MismatchCurrency(t *testing.T) {
	w := &Wallet{WorkspaceID: "ws-1", Balance: Money{Amount: 500, Currency: "XAF"}}
	amount := Money{Amount: 100, Currency: "EUR"}
	err := w.Credit(amount)
	if err == nil {
		t.Fatal("attendu une erreur pour devises differentes")
	}
	if !errors.Is(err, ErrInvalidAmount) {
		t.Errorf("attendu ErrInvalidAmount, obtenu %v", err)
	}
}
