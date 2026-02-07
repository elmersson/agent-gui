package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/rasmuselmersson/opencode/pkg/remote"
)

func main() {
	// Parse command-line flags
	length := flag.Int("length", 32, "Token length in bytes")
	count := flag.Int("count", 1, "Number of tokens to generate")
	userID := flag.String("user", "", "User identifier for the token")
	expiresIn := flag.Duration("expires", 0, "Token expiration duration (e.g., 24h, 7d). 0 = never expires")
	showHash := flag.Bool("hash", false, "Show SHA256 hash of tokens")

	flag.Parse()

	// Validate inputs
	if *length < 16 {
		fmt.Fprintf(os.Stderr, "Error: Token length must be at least 16 bytes\n")
		os.Exit(1)
	}

	if *count < 1 {
		fmt.Fprintf(os.Stderr, "Error: Count must be at least 1\n")
		os.Exit(1)
	}

	fmt.Println("Generating authentication tokens...")
	fmt.Println()

	// Generate tokens
	for i := 0; i < *count; i++ {
		token, err := remote.GenerateToken(*length)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating token: %v\n", err)
			os.Exit(1)
		}

		// Create token info
		tokenInfo := remote.CreateTokenInfo(token, *userID, *expiresIn)

		// Display token
		if *count > 1 {
			fmt.Printf("Token %d:\n", i+1)
		}
		fmt.Printf("Token:      %s\n", token)

		if *showHash {
			fmt.Printf("SHA256:     %s\n", tokenInfo.Hash)
		}

		if *userID != "" {
			fmt.Printf("User:       %s\n", *userID)
		}

		fmt.Printf("Created:    %s\n", tokenInfo.CreatedAt.Format(time.RFC3339))

		if tokenInfo.ExpiresAt != nil {
			fmt.Printf("Expires:    %s\n", tokenInfo.ExpiresAt.Format(time.RFC3339))
			duration := tokenInfo.TimeUntilExpiration()
			if duration != nil {
				fmt.Printf("Valid for:  %s\n", duration.String())
			}
		} else {
			fmt.Printf("Expires:    Never\n")
		}

		fmt.Println()
	}

	// Usage instructions
	fmt.Println("Usage:")
	fmt.Println("1. Start the remote agent server with this token:")
	fmt.Println("   opencode-remote --port 50051 --tokens '<token>'")
	fmt.Println()
	fmt.Println("2. Configure clients to use this token in their RemoteConfig:")
	fmt.Println("   AuthToken: \"<token>\"")
	fmt.Println()
	fmt.Println("⚠️  SECURITY WARNING:")
	fmt.Println("   - Store tokens securely (use environment variables or secret managers)")
	fmt.Println("   - Never commit tokens to version control")
	fmt.Println("   - Use TLS in production environments")
	fmt.Println("   - Rotate tokens regularly")
}
