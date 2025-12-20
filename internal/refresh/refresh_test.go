package refresh

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Dicklesworthstone/coding_agent_account_manager/internal/health"
)

// =============================================================================
// ShouldRefresh Tests
// =============================================================================

func TestShouldRefresh_NilHealth(t *testing.T) {
	// Nil health should return false - we don't know if refresh is needed
	result := ShouldRefresh(nil, DefaultRefreshThreshold)
	if result {
		t.Error("ShouldRefresh(nil) = true, want false")
	}
}

func TestShouldRefresh_ZeroExpiry(t *testing.T) {
	// Zero expiry time means unknown - should return false
	h := &health.ProfileHealth{
		TokenExpiresAt: time.Time{}, // zero time
	}
	result := ShouldRefresh(h, DefaultRefreshThreshold)
	if result {
		t.Error("ShouldRefresh with zero expiry = true, want false")
	}
}

func TestShouldRefresh_NotExpiring(t *testing.T) {
	// Token expiring in 2 hours with 10min threshold - should not refresh
	h := &health.ProfileHealth{
		TokenExpiresAt: time.Now().Add(2 * time.Hour),
	}
	result := ShouldRefresh(h, 10*time.Minute)
	if result {
		t.Errorf("ShouldRefresh with 2h TTL and 10min threshold = true, want false")
	}
}

func TestShouldRefresh_Expiring(t *testing.T) {
	// Token expiring in 5 minutes with 10min threshold - should refresh
	h := &health.ProfileHealth{
		TokenExpiresAt: time.Now().Add(5 * time.Minute),
	}
	result := ShouldRefresh(h, 10*time.Minute)
	if !result {
		t.Errorf("ShouldRefresh with 5min TTL and 10min threshold = false, want true")
	}
}

func TestShouldRefresh_AlreadyExpired(t *testing.T) {
	// Token already expired - should return false (ttl <= 0)
	h := &health.ProfileHealth{
		TokenExpiresAt: time.Now().Add(-5 * time.Minute),
	}
	result := ShouldRefresh(h, 10*time.Minute)
	if result {
		t.Errorf("ShouldRefresh with expired token = true, want false")
	}
}

func TestShouldRefresh_DefaultThreshold(t *testing.T) {
	// When threshold is 0, should use DefaultRefreshThreshold (10 minutes)
	// Token expiring in 5 minutes should trigger refresh
	h := &health.ProfileHealth{
		TokenExpiresAt: time.Now().Add(5 * time.Minute),
	}
	result := ShouldRefresh(h, 0) // 0 means use default
	if !result {
		t.Errorf("ShouldRefresh with 5min TTL and default threshold = false, want true")
	}

	// Token expiring in 15 minutes should NOT trigger refresh with default threshold
	h2 := &health.ProfileHealth{
		TokenExpiresAt: time.Now().Add(15 * time.Minute),
	}
	result2 := ShouldRefresh(h2, 0)
	if result2 {
		t.Errorf("ShouldRefresh with 15min TTL and default threshold = true, want false")
	}
}

func TestShouldRefresh_CustomThreshold(t *testing.T) {
	// Custom 30-minute threshold
	h := &health.ProfileHealth{
		TokenExpiresAt: time.Now().Add(20 * time.Minute),
	}

	// With 30min threshold, 20min TTL should trigger refresh
	result := ShouldRefresh(h, 30*time.Minute)
	if !result {
		t.Errorf("ShouldRefresh with 20min TTL and 30min threshold = false, want true")
	}

	// With 10min threshold, 20min TTL should NOT trigger refresh
	result2 := ShouldRefresh(h, 10*time.Minute)
	if result2 {
		t.Errorf("ShouldRefresh with 20min TTL and 10min threshold = true, want false")
	}
}

func TestShouldRefresh_EdgeCaseJustAboveThreshold(t *testing.T) {
	// Token expiring just above threshold - should NOT refresh (ttl must be < threshold)
	threshold := 10 * time.Minute
	// Add a buffer to account for test execution time
	h := &health.ProfileHealth{
		TokenExpiresAt: time.Now().Add(threshold + 1*time.Second),
	}
	result := ShouldRefresh(h, threshold)
	if result {
		t.Errorf("ShouldRefresh with TTL above threshold = true, want false")
	}
}

func TestShouldRefresh_EdgeCaseJustBelowThreshold(t *testing.T) {
	// Token expiring just below threshold - should refresh (ttl < threshold)
	threshold := 10 * time.Minute
	h := &health.ProfileHealth{
		TokenExpiresAt: time.Now().Add(threshold - 1*time.Second),
	}
	result := ShouldRefresh(h, threshold)
	if !result {
		t.Errorf("ShouldRefresh with TTL below threshold = false, want true")
	}
}

// =============================================================================
// getRefreshTokenFromJSON Tests
// =============================================================================

func TestGetRefreshToken_SnakeCase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")

	content := map[string]interface{}{
		"access_token":  "test-access-token",
		"refresh_token": "test-refresh-token-snake",
		"token_type":    "Bearer",
	}
	writeJSON(t, path, content)

	token, err := getRefreshTokenFromJSON(path)
	if err != nil {
		t.Fatalf("getRefreshTokenFromJSON error: %v", err)
	}
	if token != "test-refresh-token-snake" {
		t.Errorf("token = %q, want %q", token, "test-refresh-token-snake")
	}
}

func TestGetRefreshToken_CamelCase(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")

	content := map[string]interface{}{
		"accessToken":  "test-access-token",
		"refreshToken": "test-refresh-token-camel",
		"tokenType":    "Bearer",
	}
	writeJSON(t, path, content)

	token, err := getRefreshTokenFromJSON(path)
	if err != nil {
		t.Fatalf("getRefreshTokenFromJSON error: %v", err)
	}
	if token != "test-refresh-token-camel" {
		t.Errorf("token = %q, want %q", token, "test-refresh-token-camel")
	}
}

func TestGetRefreshToken_SnakeCasePreferredOverCamelCase(t *testing.T) {
	// When both are present, snake_case should be preferred
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")

	content := map[string]interface{}{
		"refresh_token": "snake-wins",
		"refreshToken":  "camel-loses",
	}
	writeJSON(t, path, content)

	token, err := getRefreshTokenFromJSON(path)
	if err != nil {
		t.Fatalf("getRefreshTokenFromJSON error: %v", err)
	}
	if token != "snake-wins" {
		t.Errorf("token = %q, want %q (snake_case should be preferred)", token, "snake-wins")
	}
}

func TestGetRefreshToken_Missing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")

	content := map[string]interface{}{
		"access_token": "test-access-token",
		"token_type":   "Bearer",
		// No refresh_token or refreshToken
	}
	writeJSON(t, path, content)

	_, err := getRefreshTokenFromJSON(path)
	if err == nil {
		t.Error("getRefreshTokenFromJSON should error when refresh_token is missing")
	}
}

func TestGetRefreshToken_EmptyToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")

	content := map[string]interface{}{
		"access_token":  "test-access-token",
		"refresh_token": "", // Empty string
	}
	writeJSON(t, path, content)

	_, err := getRefreshTokenFromJSON(path)
	if err == nil {
		t.Error("getRefreshTokenFromJSON should error when refresh_token is empty")
	}
}

func TestGetRefreshToken_InvalidJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")

	// Write invalid JSON
	if err := os.WriteFile(path, []byte("{not valid json"), 0600); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := getRefreshTokenFromJSON(path)
	if err == nil {
		t.Error("getRefreshTokenFromJSON should error on invalid JSON")
	}
}

func TestGetRefreshToken_MissingFile(t *testing.T) {
	_, err := getRefreshTokenFromJSON("/nonexistent/path/auth.json")
	if err == nil {
		t.Error("getRefreshTokenFromJSON should error when file doesn't exist")
	}
	if !os.IsNotExist(err) {
		t.Logf("Note: error is not os.IsNotExist, got: %v", err)
	}
}

func TestGetRefreshToken_NonStringToken(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "auth.json")

	content := map[string]interface{}{
		"access_token":  "test-access-token",
		"refresh_token": 12345, // Number instead of string
	}
	writeJSON(t, path, content)

	_, err := getRefreshTokenFromJSON(path)
	if err == nil {
		t.Error("getRefreshTokenFromJSON should error when refresh_token is not a string")
	}
}

// =============================================================================
// Helper Functions
// =============================================================================

func writeJSON(t *testing.T, path string, data interface{}) {
	t.Helper()
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal JSON: %v", err)
	}
	if err := os.WriteFile(path, jsonBytes, 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}
}
