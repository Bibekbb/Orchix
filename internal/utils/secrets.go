package utils

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/Bibekbb/Orchix/pkg/types"
)

// SecretSource represents where secrets are stored
type SecretSource string

const (
	SourceEnv      SecretSource = "env"
	SourceFile     SecretSource = "file"
	SourceVault    SecretSource = "vault"
	SourceAWS      SecretSource = "aws"
	SourceGCP      SecretSource = "gcp"
	SourceAzure    SecretSource = "azure"
	SourceK8s      SecretSource = "kubernetes"
	SourceEncrypted SecretSource = "encrypted"
)

// SecretManager manages secrets
type SecretManager struct {
	mu         sync.RWMutex
	secrets    map[string]string
	encryption EncryptionConfig
	providers  map[SecretSource]SecretProvider
}

// EncryptionConfig holds encryption settings
type EncryptionConfig struct {
	Enabled    bool   `json:"enabled"`
	Key        []byte `json:"-"`
	KeyFile    string `json:"keyFile,omitempty"`
	KeyEnvVar  string `json:"keyEnvVar,omitempty"`
	Algorithm  string `json:"algorithm"`
	IVSize     int    `json:"ivSize"`
}

// SecretProvider defines the interface for secret providers
type SecretProvider interface {
	Name() SecretSource
	GetSecret(ctx context.Context, ref types.SecretRef) (string, error)
	SetSecret(ctx context.Context, ref types.SecretRef, value string) error
	DeleteSecret(ctx context.Context, ref types.SecretRef) error
	ListSecrets(ctx context.Context) ([]string, error)
}

// NewSecretManager creates a new secret manager
func NewSecretManager(config EncryptionConfig) (*SecretManager, error) {
	sm := &SecretManager{
		secrets:    make(map[string]string),
		encryption: config,
		providers:  make(map[SecretSource]SecretProvider),
	}

	// Initialize encryption
	if config.Enabled {
		if err := sm.initEncryption(); err != nil {
			return nil, fmt.Errorf("failed to initialize encryption: %w", err)
		}
	}

	// Register built-in providers
	sm.registerProviders()

	return sm, nil
}

// GetSecret retrieves a secret
func (sm *SecretManager) GetSecret(ctx context.Context, ref types.SecretRef) (string, error) {
	// Check cache first
	sm.mu.RLock()
	if secret, ok := sm.secrets[ref.Name]; ok {
		sm.mu.RUnlock()
		return secret, nil
	}
	sm.mu.RUnlock()

	// Get from provider
	provider, err := sm.getProvider(SecretSource(ref.Source))
	if err != nil {
		return "", err
	}

	secret, err := provider.GetSecret(ctx, ref)
	if err != nil {
		return "", err
	}

	// Decrypt if needed
	if sm.encryption.Enabled && strings.HasPrefix(secret, "encrypted:") {
		secret, err = sm.decrypt(secret[10:]) // Remove "encrypted:" prefix
		if err != nil {
			return "", fmt.Errorf("failed to decrypt secret: %w", err)
		}
	}

	// Cache the secret
	sm.mu.Lock()
	sm.secrets[ref.Name] = secret
	sm.mu.Unlock()

	return secret, nil
}

// SetSecret stores a secret
func (sm *SecretManager) SetSecret(ctx context.Context, ref types.SecretRef, value string) error {
	// Encrypt if needed
	if sm.encryption.Enabled {
		encrypted, err := sm.encrypt(value)
		if err != nil {
			return fmt.Errorf("failed to encrypt secret: %w", err)
		}
		value = "encrypted:" + encrypted
	}

	// Store via provider
	provider, err := sm.getProvider(SecretSource(ref.Source))
	if err != nil {
		return err
	}

	if err := provider.SetSecret(ctx, ref, value); err != nil {
		return err
	}

	// Update cache
	sm.mu.Lock()
	sm.secrets[ref.Name] = value
	sm.mu.Unlock()

	return nil
}

// DeleteSecret removes a secret
func (sm *SecretManager) DeleteSecret(ctx context.Context, ref types.SecretRef) error {
	provider, err := sm.getProvider(SecretSource(ref.Source))
	if err != nil {
		return err
	}

	if err := provider.DeleteSecret(ctx, ref); err != nil {
		return err
	}

	// Remove from cache
	sm.mu.Lock()
	delete(sm.secrets, ref.Name)
	sm.mu.Unlock()

	return nil
}

// ListSecrets lists all secret names
func (sm *SecretManager) ListSecrets(ctx context.Context) ([]string, error) {
	var allSecrets []string

	for _, provider := range sm.providers {
		secrets, err := provider.ListSecrets(ctx)
		if err != nil {
			continue
		}
		allSecrets = append(allSecrets, secrets...)
	}

	return allSecrets, nil
}

// ResolveSecrets resolves all secrets in a manifest
func (sm *SecretManager) ResolveSecrets(ctx context.Context, manifest *types.Manifest) (map[string]string, error) {
	resolved := make(map[string]string)

	for _, secretRef := range manifest.Secrets {
		secret, err := sm.GetSecret(ctx, secretRef)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve secret %s: %w", secretRef.Name, err)
		}
		resolved[secretRef.Name] = secret
	}

	return resolved, nil
}

// RegisterProvider registers a custom secret provider
func (sm *SecretManager) RegisterProvider(provider SecretProvider) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.providers[provider.Name()] = provider
}

// Helper methods

func (sm *SecretManager) getProvider(source SecretSource) (SecretProvider, error) {
	sm.mu.RLock()
	provider, ok := sm.providers[source]
	sm.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("secret provider %s not found", source)
	}

	return provider, nil
}

func (sm *SecretManager) registerProviders() {
	// Environment variable provider
	sm.RegisterProvider(&EnvSecretProvider{})

	// File provider
	sm.RegisterProvider(&FileSecretProvider{})

	// Kubernetes provider (if in cluster)
	if isKubernetes() {
		sm.RegisterProvider(&K8sSecretProvider{})
	}
}

func (sm *SecretManager) initEncryption() error {
	// Try to load encryption key
	if sm.encryption.Key != nil {
		return nil
	}

	// Try environment variable
	if sm.encryption.KeyEnvVar != "" {
		if key := os.Getenv(sm.encryption.KeyEnvVar); key != "" {
			sm.encryption.Key = []byte(key)
			return nil
		}
	}

	// Try key file
	if sm.encryption.KeyFile != "" {
		data, err := os.ReadFile(sm.encryption.KeyFile)
		if err != nil {
			return fmt.Errorf("failed to read key file: %w", err)
		}
		sm.encryption.Key = data
		return nil
	}

	// Generate a new key
	if len(sm.encryption.Key) == 0 {
		key := make([]byte, 32) // AES-256
		if _, err := rand.Read(key); err != nil {
			return fmt.Errorf("failed to generate encryption key: %w", err)
		}
		sm.encryption.Key = key
	}

	return nil
}

// Encryption methods

func (sm *SecretManager) encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(sm.encryption.Key)
	if err != nil {
		return "", err
	}

	// Generate IV
	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	// Encrypt
	stream := cipher.NewCFBEncrypter(block, iv)
	ciphertext := make([]byte, len(plaintext))
	stream.XORKeyStream(ciphertext, []byte(plaintext))

	// Combine IV and ciphertext
	result := append(iv, ciphertext...)
	return base64.StdEncoding.EncodeToString(result), nil
}

func (sm *SecretManager) decrypt(encrypted string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}

	if len(data) < aes.BlockSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	block, err := aes.NewCipher(sm.encryption.Key)
	if err != nil {
		return "", err
	}

	// Extract IV
	iv := data[:aes.BlockSize]
	ciphertext := data[aes.BlockSize:]

	// Decrypt
	stream := cipher.NewCFBDecrypter(block, iv)
	plaintext := make([]byte, len(ciphertext))
	stream.XORKeyStream(plaintext, ciphertext)

	return string(plaintext), nil
}

// Built-in Providers

// EnvSecretProvider reads secrets from environment variables
type EnvSecretProvider struct{}

func (p *EnvSecretProvider) Name() SecretSource {
	return SourceEnv
}

func (p *EnvSecretProvider) GetSecret(ctx context.Context, ref types.SecretRef) (string, error) {
	key := ref.Key
	if key == "" {
		key = ref.Name
	}

	value := os.Getenv(key)
	if value == "" {
		return "", fmt.Errorf("environment variable %s not found", key)
	}

	return value, nil
}

func (p *EnvSecretProvider) SetSecret(ctx context.Context, ref types.SecretRef, value string) error {
	key := ref.Key
	if key == "" {
		key = ref.Name
	}

	return os.Setenv(key, value)
}

func (p *EnvSecretProvider) DeleteSecret(ctx context.Context, ref types.SecretRef) error {
	key := ref.Key
	if key == "" {
		key = ref.Name
	}

	return os.Unsetenv(key)
}

func (p *EnvSecretProvider) ListSecrets(ctx context.Context) ([]string, error) {
	// Not implemented for env vars
	return nil, nil
}

// FileSecretProvider reads secrets from files
type FileSecretProvider struct{}

func (p *FileSecretProvider) Name() SecretSource {
	return SourceFile
}

func (p *FileSecretProvider) GetSecret(ctx context.Context, ref types.SecretRef) (string, error) {
	path := ref.Path
	if path == "" {
		return "", fmt.Errorf("file path is required for file secrets")
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read secret file: %w", err)
	}

	return strings.TrimSpace(string(data)), nil
}

func (p *FileSecretProvider) SetSecret(ctx context.Context, ref types.SecretRef, value string) error {
	path := ref.Path
	if path == "" {
		return fmt.Errorf("file path is required for file secrets")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	return os.WriteFile(path, []byte(value), 0600)
}

func (p *FileSecretProvider) DeleteSecret(ctx context.Context, ref types.SecretRef) error {
	path := ref.Path
	if path == "" {
		return fmt.Errorf("file path is required for file secrets")
	}

	return os.Remove(path)
}

func (p *FileSecretProvider) ListSecrets(ctx context.Context) ([]string, error) {
	// Not easily implemented for files
	return nil, nil
}

// K8sSecretProvider reads secrets from Kubernetes
type K8sSecretProvider struct{}

func (p *K8sSecretProvider) Name() SecretSource {
	return SourceK8s
}

func (p *K8sSecretProvider) GetSecret(ctx context.Context, ref types.SecretRef) (string, error) {
	// Implementation would use kubernetes client
	// For now, return a placeholder
	return "", fmt.Errorf("kubernetes secret provider not implemented")
}

func (p *K8sSecretProvider) SetSecret(ctx context.Context, ref types.SecretRef, value string) error {
	return fmt.Errorf("kubernetes secret provider not implemented")
}

func (p *K8sSecretProvider) DeleteSecret(ctx context.Context, ref types.SecretRef) error {
	return fmt.Errorf("kubernetes secret provider not implemented")
}

func (p *K8sSecretProvider) ListSecrets(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("kubernetes secret provider not implemented")
}

// Helper functions

func isKubernetes() bool {
	// Check if we're running in Kubernetes
	return os.Getenv("KUBERNETES_SERVICE_HOST") != ""
}

// SecretStore represents a secret store configuration
type SecretStore struct {
	Type     SecretSource         `json:"type"`
	Config   map[string]string    `json:"config"`
	Prefix   string               `json:"prefix,omitempty"`
	Defaults map[string]string    `json:"defaults,omitempty"`
}

// LoadSecretsFromJSON loads secrets from a JSON file
func LoadSecretsFromJSON(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var secrets map[string]string
	if err := json.Unmarshal(data, &secrets); err != nil {
		return nil, err
	}

	return secrets, nil
}

// SaveSecretsToJSON saves secrets to a JSON file
func SaveSecretsToJSON(path string, secrets map[string]string) error {
	data, err := json.MarshalIndent(secrets, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

// MaskSecret masks a secret for logging
func MaskSecret(secret string) string {
	if len(secret) <= 4 {
		return "****"
	}
	return secret[:2] + "****" + secret[len(secret)-2:]
}