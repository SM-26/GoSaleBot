package bot

import (
	"os"
	"strings"
	"testing"
)

func TestIsAdmin(t *testing.T) {
	os.Setenv("ADMINS", "123,456, 789")
	LoadAdminsFromEnv()

	if !IsAdmin(123) {
		t.Error("Expected user 123 to be an admin")
	}
	if !IsAdmin(456) {
		t.Error("Expected user 456 to be an admin")
	}
	if !IsAdmin(789) {
		t.Error("Expected user 789 to be an admin")
	}
	if IsAdmin(999) {
		t.Error("Expected user 999 not to be an admin")
	}
}

func TestIsSafeUsername(t *testing.T) {
	safeUsernames := []string{"testuser", "test_user", "TestUser123"}
	unsafeUsernames := []string{"test-user", "test user", "test@user"}

	for _, username := range safeUsernames {
		if !isSafeUsername(username) {
			t.Errorf("Expected username '%s' to be safe, but it was not", username)
		}
	}

	for _, username := range unsafeUsernames {
		if isSafeUsername(username) {
			t.Errorf("Expected username '%s' to be unsafe, but it was safe", username)
		}
	}
}

func TestEscapeMarkdown(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"hello_world", "hello\\_world"},
		{"*bold* text", "\\*bold\\* text"},
		{"[link](url)", "\\[link\\]\\(url\\)"},
	}

	for _, tc := range testCases {
		result := EscapeMarkdown(tc.input)
		if result != tc.expected {
			t.Errorf("For input '%s', expected '%s', but got '%s'", tc.input, tc.expected, result)
		}
	}
}

func TestGetHelpMessage(t *testing.T) {
	// Test for non-admin user
	helpMessage := GetHelpMessage(999)
	if strings.Contains(helpMessage, "Admin commands") {
		t.Error("Expected help message for non-admin user not to contain admin commands")
	}

	// Test for admin user
	os.Setenv("ADMINS", "123")
	LoadAdminsFromEnv()
	helpMessage = GetHelpMessage(123)
	if !strings.Contains(helpMessage, "Admin commands") {
		t.Error("Expected help message for admin user to contain admin commands")
	}
}
