package cryptox

// Cipher defines the interface for encryption and decryption operations.
type Cipher interface {
	// Encrypt encrypts the plaintext string and returns the encrypted string.
	// The returned string is typically base64-encoded or hex-encoded.
	// Returns an error if encryption fails.
	Encrypt(plaintext string) (string, error)
	// Decrypt decrypts the encrypted string and returns the plaintext string.
	// The encrypted string is typically base64-encoded or hex-encoded.
	// Returns an error if decryption fails (e.g., invalid format, wrong key, corrupted data).
	Decrypt(ciphertext string) (string, error)
}

// Signer defines the interface for signing and verifying operations.
type Signer interface {
	// Sign signs the data string and returns the signature.
	// The returned signature is typically base64-encoded.
	// Returns an error if signing fails.
	Sign(data string) (signature string, err error)
	// Verify verifies the signature against the data.
	// Returns true if the signature is valid, false otherwise.
	// Returns an error if verification process fails (e.g., invalid format).
	Verify(data, signature string) (bool, error)
}

// CipherSigner defines the interface for encryption, decryption, signing, and verifying.
type CipherSigner interface {
	Cipher
	Signer
}
