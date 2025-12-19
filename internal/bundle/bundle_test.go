package bundle

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewManifest(t *testing.T) {
	m := NewManifest()

	if m.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", m.SchemaVersion, CurrentSchemaVersion)
	}

	if m.CAAMVersion == "" {
		t.Error("CAAMVersion should not be empty")
	}

	if m.ExportTimestamp.IsZero() {
		t.Error("ExportTimestamp should not be zero")
	}

	if m.Checksums.Algorithm != "sha256" {
		t.Errorf("Checksums.Algorithm = %q, want %q", m.Checksums.Algorithm, "sha256")
	}

	if m.Checksums.Files == nil {
		t.Error("Checksums.Files should be initialized")
	}
}

func TestManifestAddProfile(t *testing.T) {
	m := NewManifest()

	m.AddProfile("claude", "alice@gmail.com")
	m.AddProfile("claude", "bob@gmail.com")
	m.AddProfile("codex", "work@company.com")

	if !m.Contents.Vault.Included {
		t.Error("Vault.Included should be true after adding profiles")
	}

	if m.Contents.Vault.TotalProfiles != 3 {
		t.Errorf("TotalProfiles = %d, want %d", m.Contents.Vault.TotalProfiles, 3)
	}

	claudeProfiles := m.Contents.Vault.Profiles["claude"]
	if len(claudeProfiles) != 2 {
		t.Errorf("len(claude profiles) = %d, want %d", len(claudeProfiles), 2)
	}

	codexProfiles := m.Contents.Vault.Profiles["codex"]
	if len(codexProfiles) != 1 {
		t.Errorf("len(codex profiles) = %d, want %d", len(codexProfiles), 1)
	}
}

func TestManifestAddChecksum(t *testing.T) {
	m := NewManifest()

	m.AddChecksum("vault/claude/alice/.claude.json", "abc123def456")
	m.AddChecksum("config.yaml", "xyz789")

	if len(m.Checksums.Files) != 2 {
		t.Errorf("len(Checksums.Files) = %d, want %d", len(m.Checksums.Files), 2)
	}

	if m.Checksums.Files["vault/claude/alice/.claude.json"] != "abc123def456" {
		t.Error("Checksum not stored correctly")
	}
}

func TestValidateManifest(t *testing.T) {
	tests := []struct {
		name    string
		modify  func(*ManifestV1)
		wantErr bool
	}{
		{
			name:    "valid manifest",
			modify:  func(m *ManifestV1) { m.Source.Hostname = "testhost" },
			wantErr: false,
		},
		{
			name:    "nil manifest",
			modify:  nil,
			wantErr: true,
		},
		{
			name: "missing schema version",
			modify: func(m *ManifestV1) {
				m.Source.Hostname = "testhost"
				m.SchemaVersion = 0
			},
			wantErr: true,
		},
		{
			name: "missing caam version",
			modify: func(m *ManifestV1) {
				m.Source.Hostname = "testhost"
				m.CAAMVersion = ""
			},
			wantErr: true,
		},
		{
			name: "missing export timestamp",
			modify: func(m *ManifestV1) {
				m.Source.Hostname = "testhost"
				m.ExportTimestamp = time.Time{}
			},
			wantErr: true,
		},
		{
			name: "missing hostname",
			modify: func(m *ManifestV1) {
				m.Source.Hostname = ""
			},
			wantErr: true,
		},
		{
			name: "invalid platform",
			modify: func(m *ManifestV1) {
				m.Source.Hostname = "testhost"
				m.Source.Platform = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid arch",
			modify: func(m *ManifestV1) {
				m.Source.Hostname = "testhost"
				m.Source.Arch = "invalid"
			},
			wantErr: true,
		},
		{
			name: "invalid checksum algorithm",
			modify: func(m *ManifestV1) {
				m.Source.Hostname = "testhost"
				m.Checksums.Algorithm = "md5"
			},
			wantErr: true,
		},
		{
			name: "invalid checksum length",
			modify: func(m *ManifestV1) {
				m.Source.Hostname = "testhost"
				m.Checksums.Algorithm = "sha256"
				m.Checksums.Files = map[string]string{
					"test.txt": "tooshort",
				}
			},
			wantErr: true,
		},
		{
			name: "non-hex checksum",
			modify: func(m *ManifestV1) {
				m.Source.Hostname = "testhost"
				m.Checksums.Algorithm = "sha256"
				m.Checksums.Files = map[string]string{
					"test.txt": "gggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggggg",
				}
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var m *ManifestV1
			if tt.modify != nil {
				m = NewManifest()
				tt.modify(m)
			}

			err := ValidateManifest(m)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateManifest() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsCompatibleVersion(t *testing.T) {
	tests := []struct {
		name          string
		schemaVersion int
		wantErr       bool
	}{
		{
			name:          "current version",
			schemaVersion: CurrentSchemaVersion,
			wantErr:       false,
		},
		{
			name:          "future version",
			schemaVersion: CurrentSchemaVersion + 1,
			wantErr:       true,
		},
		{
			name:          "zero version",
			schemaVersion: 0,
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManifest()
			m.SchemaVersion = tt.schemaVersion

			err := IsCompatibleVersion(m)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsCompatibleVersion() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestComputeFileChecksum(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("hello world")

	if err := os.WriteFile(testFile, testData, 0600); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	checksum, err := ComputeFileChecksum(testFile, AlgorithmSHA256)
	if err != nil {
		t.Fatalf("ComputeFileChecksum() error = %v", err)
	}

	// Known SHA256 of "hello world"
	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if checksum != expected {
		t.Errorf("checksum = %q, want %q", checksum, expected)
	}
}

func TestComputeDataChecksum(t *testing.T) {
	data := []byte("hello world")

	checksum, err := ComputeDataChecksum(data, AlgorithmSHA256)
	if err != nil {
		t.Fatalf("ComputeDataChecksum() error = %v", err)
	}

	expected := "b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9"
	if checksum != expected {
		t.Errorf("checksum = %q, want %q", checksum, expected)
	}
}

func TestComputeDirectoryChecksums(t *testing.T) {
	tmpDir := t.TempDir()

	// Create some test files
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.MkdirAll(subDir, 0700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	files := map[string][]byte{
		"file1.txt":        []byte("content1"),
		"subdir/file2.txt": []byte("content2"),
	}

	for relPath, content := range files {
		path := filepath.Join(tmpDir, relPath)
		if err := os.WriteFile(path, content, 0600); err != nil {
			t.Fatalf("write file: %v", err)
		}
	}

	checksums, err := ComputeDirectoryChecksums(tmpDir, AlgorithmSHA256)
	if err != nil {
		t.Fatalf("ComputeDirectoryChecksums() error = %v", err)
	}

	if len(checksums) != 2 {
		t.Errorf("len(checksums) = %d, want %d", len(checksums), 2)
	}

	// Verify all paths are normalized (forward slashes)
	for path := range checksums {
		if filepath.IsAbs(path) {
			t.Errorf("path %q should be relative", path)
		}
	}
}

func TestVerifyChecksums(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test files
	testFile := filepath.Join(tmpDir, "test.txt")
	testData := []byte("hello")
	if err := os.WriteFile(testFile, testData, 0600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	// Compute correct checksum
	correctChecksum, _ := ComputeFileChecksum(testFile, AlgorithmSHA256)

	t.Run("valid checksums", func(t *testing.T) {
		m := NewManifest()
		m.AddChecksum("test.txt", correctChecksum)

		result, err := VerifyChecksums(tmpDir, m)
		if err != nil {
			t.Fatalf("VerifyChecksums() error = %v", err)
		}

		if !result.Valid {
			t.Errorf("result.Valid = %v, want %v", result.Valid, true)
		}

		if len(result.Verified) != 1 {
			t.Errorf("len(Verified) = %d, want %d", len(result.Verified), 1)
		}
	})

	t.Run("mismatched checksum", func(t *testing.T) {
		m := NewManifest()
		m.AddChecksum("test.txt", "0000000000000000000000000000000000000000000000000000000000000000")

		result, err := VerifyChecksums(tmpDir, m)
		if err != nil {
			t.Fatalf("VerifyChecksums() error = %v", err)
		}

		if result.Valid {
			t.Errorf("result.Valid = %v, want %v", result.Valid, false)
		}

		if len(result.Mismatch) != 1 {
			t.Errorf("len(Mismatch) = %d, want %d", len(result.Mismatch), 1)
		}
	})

	t.Run("missing file", func(t *testing.T) {
		m := NewManifest()
		m.AddChecksum("missing.txt", correctChecksum)

		result, err := VerifyChecksums(tmpDir, m)
		if err != nil {
			t.Fatalf("VerifyChecksums() error = %v", err)
		}

		if result.Valid {
			t.Errorf("result.Valid = %v, want %v", result.Valid, false)
		}

		if len(result.Missing) != 1 {
			t.Errorf("len(Missing) = %d, want %d", len(result.Missing), 1)
		}
	})
}

func TestEncryptDecryptBundle(t *testing.T) {
	plainData := []byte("this is sensitive data that needs encryption")
	password := "correct-horse-battery-staple"

	// Encrypt
	ciphertext, meta, err := EncryptBundle(plainData, password)
	if err != nil {
		t.Fatalf("EncryptBundle() error = %v", err)
	}

	if len(ciphertext) == 0 {
		t.Error("ciphertext should not be empty")
	}

	if meta == nil {
		t.Fatal("metadata should not be nil")
	}

	if meta.Algorithm != "aes-256-gcm" {
		t.Errorf("Algorithm = %q, want %q", meta.Algorithm, "aes-256-gcm")
	}

	if meta.KDF != "argon2id" {
		t.Errorf("KDF = %q, want %q", meta.KDF, "argon2id")
	}

	// Decrypt with correct password
	decrypted, err := DecryptBundle(ciphertext, meta, password)
	if err != nil {
		t.Fatalf("DecryptBundle() error = %v", err)
	}

	if string(decrypted) != string(plainData) {
		t.Errorf("decrypted = %q, want %q", decrypted, plainData)
	}

	// Decrypt with wrong password
	_, err = DecryptBundle(ciphertext, meta, "wrong-password")
	if err == nil {
		t.Error("DecryptBundle() should fail with wrong password")
	}
}

func TestEncryptBundleEmptyPassword(t *testing.T) {
	_, _, err := EncryptBundle([]byte("data"), "")
	if err == nil {
		t.Error("EncryptBundle() should fail with empty password")
	}
}

func TestDecryptBundleNilMeta(t *testing.T) {
	_, err := DecryptBundle([]byte("data"), nil, "password")
	if err == nil {
		t.Error("DecryptBundle() should fail with nil metadata")
	}
}

func TestValidateEncryptionMetadata(t *testing.T) {
	tests := []struct {
		name    string
		meta    *EncryptionMetadata
		wantErr bool
	}{
		{
			name: "valid metadata",
			meta: &EncryptionMetadata{
				Version:      1,
				Algorithm:    "aes-256-gcm",
				KDF:          "argon2id",
				Salt:         "dGVzdHNhbHQ=",
				Nonce:        "dGVzdG5vbmNl",
				Argon2Params: DefaultArgon2Params(),
			},
			wantErr: false,
		},
		{
			name:    "nil metadata",
			meta:    nil,
			wantErr: true,
		},
		{
			name: "unsupported algorithm",
			meta: &EncryptionMetadata{
				Version:   1,
				Algorithm: "aes-128-cbc",
				KDF:       "argon2id",
				Salt:      "dGVzdHNhbHQ=",
				Nonce:     "dGVzdG5vbmNl",
			},
			wantErr: true,
		},
		{
			name: "unsupported KDF",
			meta: &EncryptionMetadata{
				Version:   1,
				Algorithm: "aes-256-gcm",
				KDF:       "pbkdf2",
				Salt:      "dGVzdHNhbHQ=",
				Nonce:     "dGVzdG5vbmNl",
			},
			wantErr: true,
		},
		{
			name: "missing salt",
			meta: &EncryptionMetadata{
				Version:   1,
				Algorithm: "aes-256-gcm",
				KDF:       "argon2id",
				Nonce:     "dGVzdG5vbmNl",
			},
			wantErr: true,
		},
		{
			name: "missing nonce",
			meta: &EncryptionMetadata{
				Version:   1,
				Algorithm: "aes-256-gcm",
				KDF:       "argon2id",
				Salt:      "dGVzdHNhbHQ=",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEncryptionMetadata(tt.meta)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEncryptionMetadata() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestNormalizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"foo/bar", "foo/bar"},
		{"foo\\bar", "foo/bar"},
		{"foo\\bar\\baz", "foo/bar/baz"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := NormalizePath(tt.input)
			if got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultArgon2Params(t *testing.T) {
	params := DefaultArgon2Params()

	if params.Time < 1 {
		t.Error("Time should be >= 1")
	}

	if params.Memory < 1024 {
		t.Error("Memory should be >= 1024 (1 MiB)")
	}

	if params.Threads < 1 {
		t.Error("Threads should be >= 1")
	}

	if params.KeyLen != 32 {
		t.Errorf("KeyLen = %d, want %d (for AES-256)", params.KeyLen, 32)
	}
}

func TestSaveAndLoadManifest(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a manifest
	m := NewManifest()
	m.Source.Hostname = "test-host"
	m.Source.Platform = "linux"
	m.Source.Arch = "amd64"
	m.AddProfile("claude", "alice@gmail.com")

	// Save it
	if err := SaveManifest(tmpDir, m); err != nil {
		t.Fatalf("SaveManifest() error = %v", err)
	}

	// Load it back
	loaded, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifest() error = %v", err)
	}

	// Verify
	if loaded.Source.Hostname != m.Source.Hostname {
		t.Errorf("Hostname = %q, want %q", loaded.Source.Hostname, m.Source.Hostname)
	}

	if loaded.Contents.Vault.TotalProfiles != 1 {
		t.Errorf("TotalProfiles = %d, want %d", loaded.Contents.Vault.TotalProfiles, 1)
	}
}

func TestSaveAndLoadEncryptionMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	meta := &EncryptionMetadata{
		Version:      1,
		Algorithm:    "aes-256-gcm",
		KDF:          "argon2id",
		Salt:         "dGVzdHNhbHQ=",
		Nonce:        "dGVzdG5vbmNl",
		Argon2Params: DefaultArgon2Params(),
	}

	// Save it
	if err := SaveEncryptionMetadata(tmpDir, meta); err != nil {
		t.Fatalf("SaveEncryptionMetadata() error = %v", err)
	}

	// Load it back
	loaded, err := LoadEncryptionMetadata(tmpDir)
	if err != nil {
		t.Fatalf("LoadEncryptionMetadata() error = %v", err)
	}

	if loaded == nil {
		t.Fatal("loaded metadata should not be nil")
	}

	if loaded.Algorithm != meta.Algorithm {
		t.Errorf("Algorithm = %q, want %q", loaded.Algorithm, meta.Algorithm)
	}

	if loaded.Salt != meta.Salt {
		t.Errorf("Salt = %q, want %q", loaded.Salt, meta.Salt)
	}
}

func TestIsEncrypted(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("not encrypted", func(t *testing.T) {
		encrypted, err := IsEncrypted(tmpDir)
		if err != nil {
			t.Fatalf("IsEncrypted() error = %v", err)
		}
		if encrypted {
			t.Error("should not be encrypted")
		}
	})

	t.Run("encrypted directory", func(t *testing.T) {
		// Create marker file
		markerPath := filepath.Join(tmpDir, EncryptionMarkerFile)
		if err := os.WriteFile(markerPath, []byte("{}"), 0600); err != nil {
			t.Fatalf("write marker: %v", err)
		}

		encrypted, err := IsEncrypted(tmpDir)
		if err != nil {
			t.Fatalf("IsEncrypted() error = %v", err)
		}
		if !encrypted {
			t.Error("should be encrypted")
		}
	})

	t.Run("encrypted filename", func(t *testing.T) {
		encryptedPath := filepath.Join(tmpDir, "bundle.enc.zip")
		encrypted, err := IsEncrypted(encryptedPath)
		if err != nil {
			t.Fatalf("IsEncrypted() error = %v", err)
		}
		if !encrypted {
			t.Error("should be detected as encrypted by filename")
		}
	})
}

func TestVerificationResultSummary(t *testing.T) {
	tests := []struct {
		name   string
		result *VerificationResult
		want   string
	}{
		{
			name: "all valid",
			result: &VerificationResult{
				Valid:    true,
				Verified: []string{"a", "b", "c"},
			},
			want: "Verified 3 files, all checksums match",
		},
		{
			name: "missing only",
			result: &VerificationResult{
				Valid:   false,
				Missing: []string{"a"},
			},
			want: "Verification failed: 1 missing",
		},
		{
			name: "corrupted only",
			result: &VerificationResult{
				Valid:    false,
				Mismatch: []ChecksumMismatch{{Path: "a"}},
			},
			want: "Verification failed: 1 corrupted",
		},
		{
			name: "missing and corrupted",
			result: &VerificationResult{
				Valid:    false,
				Missing:  []string{"a", "b"},
				Mismatch: []ChecksumMismatch{{Path: "c"}},
			},
			want: "Verification failed: 2 missing and 1 corrupted",
		},
		{
			name: "all three",
			result: &VerificationResult{
				Valid:    false,
				Missing:  []string{"a"},
				Mismatch: []ChecksumMismatch{{Path: "b"}},
				Extra:    []string{"c"},
			},
			want: "Verification failed: 1 missing, 1 corrupted, and 1 extra",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.Summary()
			if got != tt.want {
				t.Errorf("Summary() = %q, want %q", got, tt.want)
			}
		})
	}
}
