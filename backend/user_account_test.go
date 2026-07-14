package app

import (
	"testing"
)

func TestValidateUserAccount(t *testing.T) {
	app := NewApp()

	tests := []struct {
		name        string
		data        UserAccountData
		expectError bool
		errorMsg    string
	}{
		{
			name:        "valid account data",
			data:        UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "password123"},
			expectError: false,
		},
		{
			name:        "valid minimum length username and password",
			data:        UserAccountData{Username: "abc", Password: "abc123", ConfirmPassword: "abc123"},
			expectError: false,
		},
		{
			name:        "empty username",
			data:        UserAccountData{Username: "", Password: "password123", ConfirmPassword: "password123"},
			expectError: true,
			errorMsg:    "username is required",
		},
		{
			name:        "username too short",
			data:        UserAccountData{Username: "ab", Password: "password123", ConfirmPassword: "password123"},
			expectError: true,
			errorMsg:    "username must be at least 3 characters",
		},
		{
			name:        "empty password",
			data:        UserAccountData{Username: "john_doe", Password: "", ConfirmPassword: ""},
			expectError: true,
			errorMsg:    "password is required",
		},
		{
			name:        "password too short",
			data:        UserAccountData{Username: "john_doe", Password: "abc12", ConfirmPassword: "abc12"},
			expectError: true,
			errorMsg:    "password must be at least 6 characters",
		},
		{
			name:        "empty confirm password",
			data:        UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: ""},
			expectError: true,
			errorMsg:    "please confirm your password",
		},
		{
			name:        "passwords do not match",
			data:        UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "different"},
			expectError: true,
			errorMsg:    "passwords do not match",
		},
		{
			name:        "all fields empty",
			data:        UserAccountData{Username: "", Password: "", ConfirmPassword: ""},
			expectError: true,
			errorMsg:    "username is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := app.ValidateUserAccount(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
				return
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tt.expectError && err != nil && err.Error() != tt.errorMsg {
				t.Errorf("expected error message %q, got %q", tt.errorMsg, err.Error())
			}
		})
	}
}

func TestValidateUserAccount_SavesOnSuccess(t *testing.T) {
	app := NewApp()
	data := UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "password123"}

	err := app.ValidateUserAccount(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if app.userAccount.Username != "john_doe" {
		t.Errorf("expected username to be saved as 'john_doe', got %q", app.userAccount.Username)
	}
	if app.userAccount.Password != "password123" {
		t.Errorf("expected password to be saved, got %q", app.userAccount.Password)
	}
}

func TestValidateUserAccount_DoesNotSaveOnFailure(t *testing.T) {
	app := NewApp()
	validData := UserAccountData{Username: "john_doe", Password: "password123", ConfirmPassword: "password123"}
	invalidData := UserAccountData{Username: "", Password: "", ConfirmPassword: ""}

	// First, save valid data
	err := app.ValidateUserAccount(validData)
	if err != nil {
		t.Fatalf("unexpected error on valid data: %v", err)
	}

	// Then try invalid data
	err = app.ValidateUserAccount(invalidData)
	if err == nil {
		t.Fatal("expected error on invalid data")
	}

	// Original data should still be saved
	if app.userAccount.Username != "john_doe" {
		t.Errorf("expected saved username to still be 'john_doe', got %q", app.userAccount.Username)
	}
}
