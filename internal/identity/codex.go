package identity

import (
	"encoding/json"
	"fmt"
	"os"
)

// ExtractFromCodexAuth reads a Codex auth.json file and extracts identity from the JWT.
func ExtractFromCodexAuth(path string) (*Identity, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read codex auth.json: %w", err)
	}

	var auth map[string]interface{}
	if err := json.Unmarshal(data, &auth); err != nil {
		return nil, fmt.Errorf("parse codex auth.json: %w", err)
	}

	token, field, ok := findCodexToken(auth)
	if !ok {
		return nil, fmt.Errorf("id_token not found in auth.json")
	}

	identity, err := ExtractFromJWT(token)
	if err != nil {
		return nil, fmt.Errorf("parse jwt from %s: %w", field, err)
	}
	identity.Provider = "codex"
	return identity, nil
}

func findCodexToken(auth map[string]interface{}) (string, string, bool) {
	if token := stringFromMap(auth, "id_token"); token != "" {
		return token, "id_token", true
	}
	if token := stringFromMap(auth, "idToken"); token != "" {
		return token, "idToken", true
	}

	rawTokens, ok := auth["tokens"]
	var tokenMap map[string]interface{}
	if ok {
		tokenMap, _ = rawTokens.(map[string]interface{})
		if token := stringFromMap(tokenMap, "id_token"); token != "" {
			return token, "tokens.id_token", true
		}
		if token := stringFromMap(tokenMap, "idToken"); token != "" {
			return token, "tokens.idToken", true
		}
	}

	if token := stringFromMap(auth, "access_token"); token != "" {
		return token, "access_token", true
	}
	if token := stringFromMap(auth, "accessToken"); token != "" {
		return token, "accessToken", true
	}
	if token := stringFromMap(auth, "token"); token != "" {
		return token, "token", true
	}

	if tokenMap == nil {
		return "", "", false
	}
	if token := stringFromMap(tokenMap, "access_token"); token != "" {
		return token, "tokens.access_token", true
	}
	if token := stringFromMap(tokenMap, "accessToken"); token != "" {
		return token, "tokens.accessToken", true
	}
	if token := stringFromMap(tokenMap, "token"); token != "" {
		return token, "tokens.token", true
	}

	return "", "", false
}

func stringFromMap(values map[string]interface{}, key string) string {
	if values == nil {
		return ""
	}
	value, ok := values[key]
	if !ok {
		return ""
	}
	if str, ok := value.(string); ok {
		return str
	}
	return ""
}
