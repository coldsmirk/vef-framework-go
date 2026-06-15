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

// FixedIVDecrypter is implemented by block ciphers (AES-CBC, SM4-CBC) that can
// decrypt bare ciphertext using a caller-supplied constant IV.
//
// It is an interop escape hatch: native VEF ciphertext prepends a fresh random
// IV and is read back through Cipher.Decrypt, but an external client that
// encrypts with a fixed IV produces ciphertext without that prefix.
// DecryptWithFixedIV decrypts such input using the IV configured at
// construction (WithAESIv / WithSM4Iv).
type FixedIVDecrypter interface {
	// DecryptWithFixedIV decrypts base64-encoded ciphertext that carries no
	// prepended IV, using the fixed IV the cipher was constructed with.
	// Returns an error if no valid fixed IV was configured or decryption fails.
	DecryptWithFixedIV(ciphertext string) (string, error)
}
